package dashboard

// alerts.go — Eşik tabanlı otomatik uyarılar (Plan Faz 9: "CPI/SPI, geciken
// milestone, yaşlanan bulgu"). Worker periyodik çağırır.
//
// Tekilleştirme: control_alerts (project, alert_key, period='YYYY-MM') tekil —
// aynı uyarı aynı ay içinde bir kez bildirilir; sonraki ay durum sürüyorsa
// yeniden hatırlatılır (aylık kontrol ritmiyle uyumlu).
//
// Hedef kitle (Plan §7: "PY'ye otomatik uyarı"): proje üyeleri arasında rolü
// hakediş kesinleştirme yetkisi (progress_payments.finalize) taşıyanlar —
// pratikte ProjectManager (+ Admin üye ise). ohs.notifyByCapability deseni.

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
)

// RunControlAlerts — tüm Aktif projeleri tarar, eşik ihlallerini bildirir.
func RunControlAlerts(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service, log *slog.Logger) {
	rows, err := pool.Query(ctx, `
		SELECT id, code FROM projects
		WHERE deleted_at IS NULL AND status='Active'`)
	if err != nil {
		log.Error("uyarı taraması: projeler okunamadı", "err", err)
		return
	}
	type proj struct {
		id   uuid.UUID
		code string
	}
	var projects []proj
	for rows.Next() {
		var p proj
		if rows.Scan(&p.id, &p.code) == nil {
			projects = append(projects, p)
		}
	}
	rows.Close()

	now := time.Now().UTC()
	period := MonthKey(now)

	for _, p := range projects {
		settings, err := loadControlSettings(ctx, pool, p.id)
		if err != nil {
			log.Error("uyarı taraması: eşikler okunamadı", "err", err, "project", p.id)
			continue
		}

		// --- CPI / SPI eşik ihlali ---
		evm, err := LoadEVM(ctx, pool, p.id, now)
		if err != nil {
			log.Error("uyarı taraması: EVM derlenemedi", "err", err, "project", p.id)
			continue
		}
		if evm.CPI > 0 && evm.CPI < settings.CPIMin {
			raiseAlert(ctx, pool, nt, log, p.id, "cpi_low", period,
				notify.TypeEVMThresholdAlert,
				fmt.Sprintf("CPI eşik altında: %s", p.code),
				fmt.Sprintf("CPI %.3f < eşik %.2f (EV %.2f / AC %.2f). Maliyet performansı incelenmeli.",
					evm.CPI, settings.CPIMin, evm.EV, evm.AC))
		}
		if evm.SPI > 0 && evm.SPI < settings.SPIMin {
			raiseAlert(ctx, pool, nt, log, p.id, "spi_low", period,
				notify.TypeEVMThresholdAlert,
				fmt.Sprintf("SPI eşik altında: %s", p.code),
				fmt.Sprintf("SPI %.3f < eşik %.2f (EV %.2f / PV %.2f). Takvim performansı incelenmeli.",
					evm.SPI, settings.SPIMin, evm.EV, evm.PV))
		}

		// --- Geciken milestone'lar ---
		mrows, err := pool.Query(ctx, `
			SELECT id, name, to_char(planned_date,'YYYY-MM-DD')
			FROM milestones
			WHERE project_id=$1 AND deleted_at IS NULL
			  AND planned_date IS NOT NULL AND planned_date < CURRENT_DATE
			  AND status <> 'Completed'`, p.id)
		if err != nil {
			log.Error("uyarı taraması: milestone okunamadı", "err", err, "project", p.id)
			continue
		}
		type ms struct {
			id      uuid.UUID
			name, d string
		}
		var late []ms
		for mrows.Next() {
			var m ms
			if mrows.Scan(&m.id, &m.name, &m.d) == nil {
				late = append(late, m)
			}
		}
		mrows.Close()
		for _, m := range late {
			raiseAlert(ctx, pool, nt, log, p.id, "milestone_late:"+m.id.String(), period,
				notify.TypeMilestoneLateAlert,
				fmt.Sprintf("Geciken milestone: %s", p.code),
				fmt.Sprintf("%q planlanan tarihi (%s) geçti ve tamamlanmadı.", m.name, m.d))
		}

		// --- Yaşlanan açık İSG bulguları ---
		frows, err := pool.Query(ctx, `
			SELECT id, severity, description
			FROM ohs_findings
			WHERE project_id=$1 AND deleted_at IS NULL
			  AND status IN ('Open','InProgress')
			  AND created_at < now() - make_interval(days => $2)`,
			p.id, settings.FindingAgingDays)
		if err != nil {
			log.Error("uyarı taraması: bulgular okunamadı", "err", err, "project", p.id)
			continue
		}
		type fd struct {
			id        uuid.UUID
			sev, desc string
		}
		var aging []fd
		for frows.Next() {
			var f fd
			if frows.Scan(&f.id, &f.sev, &f.desc) == nil {
				aging = append(aging, f)
			}
		}
		frows.Close()
		for _, f := range aging {
			d := f.desc
			if len(d) > 80 {
				d = d[:79] + "…"
			}
			raiseAlert(ctx, pool, nt, log, p.id, "finding_aging:"+f.id.String(), period,
				notify.TypeFindingAgingAlert,
				fmt.Sprintf("Yaşlanan İSG bulgusu: %s", p.code),
				fmt.Sprintf("[%s] %s — %d günden uzun süredir açık.", f.sev, d, settings.FindingAgingDays))
		}
	}
}

// raiseAlert — tekilleştirme defterine yazmayı dener; yeni satır açıldıysa
// (yani bu ay ilk kez) PY'lere bildirim gönderir.
func raiseAlert(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service, log *slog.Logger,
	pid uuid.UUID, key, period, ntype, title, body string) {

	tag, err := pool.Exec(ctx, `
		INSERT INTO control_alerts (project_id, alert_key, period, detail)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id, alert_key, period) DO NOTHING`,
		pid, key, period, body)
	if err != nil {
		log.Error("uyarı defterine yazılamadı", "err", err, "project", pid, "key", key)
		return
	}
	if tag.RowsAffected() == 0 {
		return // bu ay zaten bildirildi
	}

	targets, err := projectManagersOf(ctx, pool, pid)
	if err != nil {
		log.Error("uyarı hedefleri okunamadı", "err", err, "project", pid)
		return
	}
	if len(targets) == 0 {
		log.Warn("uyarı üretildi ama PY hedefi yok", "project", pid, "key", key)
		return
	}
	nt.Send(ctx, notify.Input{
		UserIDs: targets, Type: ntype, Title: title, Body: body,
		EntityType: "projects", EntityID: &pid, ProjectID: &pid,
	})
	log.Info("kontrol uyarısı bildirildi", "project", pid, "key", key, "targets", len(targets))
}

// projectManagersOf — rol varsayılanı progress_payments.finalize içeren üyeler.
func projectManagersOf(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx, `
		SELECT pm.user_id, r.code FROM project_members pm
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		var role string
		if rows.Scan(&uid, &role) != nil {
			continue
		}
		if rbac.RoleHasDefault(role, "progress_payments.finalize") {
			out = append(out, uid)
		}
	}
	return out, nil
}
