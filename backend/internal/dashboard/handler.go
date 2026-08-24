package dashboard

// handler.go — Faz 9 HTTP uçları.
//
// Rol duyarlılık (Plan Faz 9: "PY tam, Client sadeleştirilmiş, taşeron kendi
// verisi") BACKEND'de uygulanır:
//   * Finansal blok (EVM, tutarlar) yalnızca reports.view_financial_reports
//     izni olanlara döner — asla yalnızca frontend'de gizlenmez (Plan §4).
//   * Taşeron temsilcisi (project_members.subcontractor_id dolu) bekleyen
//     hakediş/bulgu sayaçlarını yalnızca KENDİ firması için görür; aktivite
//     akışı ve portföy toplulaştırması ona dönmez.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/fixedexpenses"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/ohsaccidents"
	"github.com/ipks/ipks/backend/internal/rbac"
	"github.com/ipks/ipks/backend/internal/storage"
)

type Handler struct {
	pool  *pgxpool.Pool
	eval  *rbac.Evaluator
	rec   *audit.Recorder
	nt    *notify.Service
	store *storage.Client
	log   *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder,
	nt *notify.Service, store *storage.Client, log *slog.Logger) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, nt: nt, store: store, log: log}
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}

func requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, false
	}
	return uid, true
}

// scopedSub — kullanıcı bu projede bir taşerona bağlıysa firma id'si döner
// (satır seviyesi güvenlik — payments/ohs ile aynı desen).
func (h *Handler) scopedSub(ctx context.Context, uid, pid uuid.UUID) (*uuid.UUID, error) {
	var sub *uuid.UUID
	err := h.pool.QueryRow(ctx, `
		SELECT subcontractor_id FROM project_members
		WHERE project_id=$1 AND user_id=$2 AND deleted_at IS NULL
		  AND subcontractor_id IS NOT NULL`, pid, uid).Scan(&sub)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return sub, err
}

// ---------------------------------------------------------------------------
// GET /projects/{projectID}/dashboard — rol duyarlı proje dashboard'u
// ---------------------------------------------------------------------------

type milestoneDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	PlannedDate *string   `json:"planned_date"`
	ActualDate  *string   `json:"actual_date"`
	WeightPct   *float64  `json:"weight_pct"`
	Status      string    `json:"status"`
	Late        bool      `json:"late"` // planlanan geçti, tamamlanmadı
}

type activityDTO struct {
	Entity     string    `json:"entity"`
	EntityID   uuid.UUID `json:"entity_id"`
	FromStatus *string   `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Actor      *string   `json:"actor"`
	At         time.Time `json:"at"`
}

type dashboardDTO struct {
	Project struct {
		ID       uuid.UUID `json:"id"`
		Code     string    `json:"code"`
		Name     string    `json:"name"`
		Status   string    `json:"status"`
		Currency string    `json:"currency"`
	} `json:"project"`

	// Fiziki ilerleme herkese açık; TUTARLAR (evm bloğu) izinle döner.
	ProgressPct float64        `json:"progress_pct"`
	Milestones  []milestoneDTO `json:"milestones"`

	EVM *EVMResult `json:"evm,omitempty"` // yalnızca reports.view_financial_reports

	OpenFindings struct {
		Total       int `json:"total"`
		Critical    int `json:"critical"`
		Major       int `json:"major"`
		Minor       int `json:"minor"`
		Observation int `json:"observation"`
		Overdue     int `json:"overdue"`
	} `json:"open_findings"`

	Pending struct {
		Payments     int `json:"payments"` // Draft|Submitted|SiteApproved
		MARs         int `json:"mars"`     // Submitted|UnderReview
		PRs          int `json:"prs"`      // Submitted
		OpenPOs      int `json:"open_pos"` // Ordered|PartiallyDelivered (gecikmemiş)
		DeliveredPOs int `json:"delivered_pos"`
		OverduePO    int `json:"overdue_pos"`
		OpenTasks    int `json:"open_tasks"`
	} `json:"pending"`

	Activity []activityDTO `json:"activity,omitempty"` // taşerona dönmez

	// Dashboard v2 — kullanıcının paylaştığı örnek panelden ilham alınan,
	// birebir değil KONSEPT olarak uyarlanan yeni widget'lar.
	CostBreakdown    *costBreakdownDTO           `json:"cost_breakdown,omitempty"` // finansal izinle (evm ile aynı kapı)
	Giderler         *giderlerDTO                `json:"giderler,omitempty"`       // Proje Özet: 4 ayrı kalem (finansal izinle)
	DocumentStatus   *documentStatusDTO          `json:"document_status,omitempty"`
	CoverImage       *coverImageDTO              `json:"cover_image,omitempty"`
	AccidentFreeDays ohsaccidents.FreeDaysSummary `json:"accident_free_days"`

	SubcontractorScoped bool `json:"subcontractor_scoped"`
}

type costBreakdownDTO struct {
	Tasaron float64 `json:"tasaron"`
	Malzeme float64 `json:"malzeme"`
	Diger   float64 `json:"diger"`
}

// giderlerDTO — Proje Özet sayfasındaki gider kırılımı: aynı cash_events
// verisi costBreakdownDTO ile aynı kaynaktan gelir, sadece "Diğer" burada
// Kasa Harcamaları ve Sabit Giderler olarak ayrıştırılır.
type giderlerDTO struct {
	Satinalma       float64 `json:"satinalma"`
	KasaHarcamalari float64 `json:"kasa_harcamalari"`
	TasaronHakedis  float64 `json:"tasaron_hakedis"`
	SabitGiderler   float64 `json:"sabit_giderler"`
}

type documentStatusDTO struct {
	Onayli   int `json:"onayli"`
	Revizyon int `json:"revizyon"`
	Taslak   int `json:"taslak"`
}

type coverImageDTO struct {
	DocumentID uuid.UUID `json:"document_id"`
	Version    int       `json:"version"`
}

func (h *Handler) ProjectDashboard(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	sub, err := h.scopedSub(ctx, uid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	var out dashboardDTO
	out.SubcontractorScoped = sub != nil
	if err := h.pool.QueryRow(ctx, `
		SELECT id, code, name, status, currency FROM projects
		WHERE id=$1 AND deleted_at IS NULL`, pid).
		Scan(&out.Project.ID, &out.Project.Code, &out.Project.Name,
			&out.Project.Status, &out.Project.Currency); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	// EVM — herkes için hesaplanır (ilerleme % buradan gelir); TUTARLAR yalnızca
	// finansal rapor izni olanlara serileştirilir.
	evm, err := LoadEVM(ctx, h.pool, pid, time.Now().UTC())
	if err != nil {
		h.log.Error("dashboard EVM derlenemedi", "err", err, "project", pid)
		httpx.Internal(w, r)
		return
	}
	out.ProgressPct = evm.ProgressPct
	canFin, err := h.eval.Can(ctx, uid, &pid, "reports.view_financial_reports")
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if canFin && sub == nil {
		out.EVM = evm

		// Maliyet dağılımı (Dashboard v2) — cash_events'teki gerçek harcamalar
		// (Taşeron: hakediş ödeme planı, Malzeme: tedarikçi ekstresi + PO
		// ödemesi, Diğer: kasa fişi) + Faz E'nin sanal sabit gider satırları
		// (proje başlangıcından bugüne, Diğer'e eklenir). İşçilik kalemi
		// YOK — puantaj yalnızca saat tutar, ücret/maliyet alanı hiçbir
		// yerde yok (bilinçli olarak dışarıda bırakıldı).
		cb := costBreakdownDTO{}
		if err := h.pool.QueryRow(ctx, `
			SELECT
			  COALESCE(sum(amount) FILTER (WHERE source_entity='progress_payment_disbursement'),0)::float8,
			  COALESCE(sum(amount) FILTER (WHERE source_entity IN ('supplier_payment','po_payment')),0)::float8,
			  COALESCE(sum(amount) FILTER (WHERE source_entity='kasa_fisi'),0)::float8
			FROM cash_events
			WHERE project_id=$1 AND direction='out' AND deleted_at IS NULL`, pid).
			Scan(&cb.Tasaron, &cb.Malzeme, &cb.Diger); err != nil {
			httpx.Internal(w, r)
			return
		}
		var projectStart *time.Time
		if err := h.pool.QueryRow(ctx, `SELECT start_date FROM projects WHERE id=$1`, pid).
			Scan(&projectStart); err != nil {
			httpx.Internal(w, r)
			return
		}
		var expandedFixed float64
		if projectStart != nil {
			expenses, err := fixedexpenses.ListActive(ctx, h.pool, pid)
			if err != nil {
				httpx.Internal(w, r)
				return
			}
			for _, v := range fixedexpenses.Expand(expenses, *projectStart, time.Now().UTC()) {
				expandedFixed += v.Amount
			}
		}
		cb.Diger += expandedFixed
		out.CostBreakdown = &cb

		// Proje Özet giderler kırılımı — aynı cash_events sorgusu, "Diğer"i
		// Kasa Harcamaları / Sabit Giderler olarak ayırır.
		gd := giderlerDTO{Satinalma: cb.Malzeme, TasaronHakedis: cb.Tasaron, SabitGiderler: expandedFixed}
		if err := h.pool.QueryRow(ctx, `
			SELECT COALESCE(sum(amount) FILTER (WHERE source_entity='kasa_fisi'),0)::float8
			FROM cash_events
			WHERE project_id=$1 AND direction='out' AND deleted_at IS NULL`, pid).
			Scan(&gd.KasaHarcamalari); err != nil {
			httpx.Internal(w, r)
			return
		}
		out.Giderler = &gd
	}

	// Kazasız gün sayacı (Dashboard v2) — herkese açık (finansal veri değil).
	freeDays, err := ohsaccidents.LoadFreeDays(ctx, h.pool, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	out.AccidentFreeDays = freeDays

	// Proje görseli (Dashboard v2) — herkese açık.
	var cover coverImageDTO
	if err := h.pool.QueryRow(ctx, `
		SELECT d.id, v.latest
		FROM documents d
		JOIN (SELECT document_id, max(version_no) latest FROM document_versions GROUP BY document_id) v
		  ON v.document_id = d.id
		WHERE d.project_id=$1 AND d.doc_category='ProjeGorseli' AND d.deleted_at IS NULL
		ORDER BY d.created_at DESC LIMIT 1`, pid).
		Scan(&cover.DocumentID, &cover.Version); err == nil {
		out.CoverImage = &cover
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.Internal(w, r)
		return
	}

	// Milestone timeline.
	rows, err := h.pool.Query(ctx, `
		SELECT id, name, to_char(planned_date,'YYYY-MM-DD'), to_char(actual_date,'YYYY-MM-DD'),
		       weight_pct, status,
		       (planned_date IS NOT NULL AND planned_date < CURRENT_DATE AND status <> 'Completed')
		FROM milestones WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY sort_order, planned_date NULLS LAST, created_at`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	out.Milestones = []milestoneDTO{}
	for rows.Next() {
		var m milestoneDTO
		if err := rows.Scan(&m.ID, &m.Name, &m.PlannedDate, &m.ActualDate,
			&m.WeightPct, &m.Status, &m.Late); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		out.Milestones = append(out.Milestones, m)
	}
	rows.Close()

	// Açık İSG bulguları (taşeron kapsamı zorunlu filtre).
	subFilter := ""
	args := []interface{}{pid}
	if sub != nil {
		subFilter = " AND subcontractor_id=$2"
		args = append(args, *sub)
	}
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE severity='Critical'),
		       count(*) FILTER (WHERE severity='Major'),
		       count(*) FILTER (WHERE severity='Minor'),
		       count(*) FILTER (WHERE severity='Observation'),
		       count(*) FILTER (WHERE due_date IS NOT NULL AND due_date < CURRENT_DATE)
		FROM ohs_findings
		WHERE project_id=$1 AND deleted_at IS NULL AND status IN ('Open','InProgress')`+subFilter,
		args...).Scan(&out.OpenFindings.Total, &out.OpenFindings.Critical,
		&out.OpenFindings.Major, &out.OpenFindings.Minor,
		&out.OpenFindings.Observation, &out.OpenFindings.Overdue); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Bekleyen onaylar (taşeron: yalnızca kendi hakedişleri; MAR/PR/PO/task
	// sayaçları operasyonel özet olduğundan taşerona hakediş dışında dönmez).
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FROM progress_payments
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND status IN ('Draft','Submitted','SiteApproved')`+subFilter,
		args...).Scan(&out.Pending.Payments); err != nil {
		httpx.Internal(w, r)
		return
	}
	if sub == nil {
		// Doküman durumu (Dashboard v2) — kategoriden bağımsız, projedeki
		// tüm dokümanların approval_status kırılımı.
		ds := documentStatusDTO{}
		if err := h.pool.QueryRow(ctx, `
			SELECT
			  count(*) FILTER (WHERE approval_status='Onaylı'),
			  count(*) FILTER (WHERE approval_status='Revizyon'),
			  count(*) FILTER (WHERE approval_status='Taslak')
			FROM documents WHERE project_id=$1 AND deleted_at IS NULL`, pid).
			Scan(&ds.Onayli, &ds.Revizyon, &ds.Taslak); err != nil {
			httpx.Internal(w, r)
			return
		}
		out.DocumentStatus = &ds

		if err := h.pool.QueryRow(ctx, `
			SELECT
			  (SELECT count(*) FROM material_approvals
			   WHERE project_id=$1 AND deleted_at IS NULL AND status IN ('Submitted','UnderReview')),
			  (SELECT count(*) FROM purchase_requests
			   WHERE project_id=$1 AND deleted_at IS NULL AND status='Submitted'),
			  (SELECT count(*) FROM purchase_orders
			   WHERE project_id=$1 AND deleted_at IS NULL
			     AND status IN ('Ordered','PartiallyDelivered')),
			  (SELECT count(*) FROM purchase_orders
			   WHERE project_id=$1 AND deleted_at IS NULL AND status='Delivered'),
			  (SELECT count(*) FROM purchase_orders
			   WHERE project_id=$1 AND deleted_at IS NULL
			     AND status IN ('Ordered','PartiallyDelivered')
			     AND expected_date IS NOT NULL AND expected_date < CURRENT_DATE),
			  (SELECT count(*) FROM tasks
			   WHERE project_id=$1 AND deleted_at IS NULL AND status <> 'Done')`, pid).
			Scan(&out.Pending.MARs, &out.Pending.PRs, &out.Pending.OpenPOs, &out.Pending.DeliveredPOs,
				&out.Pending.OverduePO, &out.Pending.OpenTasks); err != nil {
			httpx.Internal(w, r)
			return
		}

		// Aktivite akışı: son iş akışı geçişleri (audit değil — iş dili).
		arows, err := h.pool.Query(ctx, `
			SELECT wt.entity, wt.entity_id, wt.from_status, wt.to_status, u.full_name, wt.at
			FROM workflow_transitions wt
			LEFT JOIN users u ON u.id = wt.actor_id
			WHERE wt.entity_id IN (
			    SELECT id FROM progress_payments WHERE project_id=$1
			    UNION ALL SELECT id FROM material_approvals WHERE project_id=$1
			    UNION ALL SELECT id FROM purchase_requests   WHERE project_id=$1
			    UNION ALL SELECT id FROM purchase_orders     WHERE project_id=$1
			    UNION ALL SELECT id FROM ohs_findings        WHERE project_id=$1
			    UNION ALL SELECT id FROM daily_reports       WHERE project_id=$1
			)
			ORDER BY wt.at DESC LIMIT 15`, pid)
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		out.Activity = []activityDTO{}
		for arows.Next() {
			var a activityDTO
			if err := arows.Scan(&a.Entity, &a.EntityID, &a.FromStatus,
				&a.ToStatus, &a.Actor, &a.At); err != nil {
				arows.Close()
				httpx.Internal(w, r)
				return
			}
			out.Activity = append(out.Activity, a)
		}
		arows.Close()
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{"dashboard": out})
}

// ---------------------------------------------------------------------------
// GET /portfolio — projeler arası özet kartlar (Plan §3 portföy görünümü)
// ---------------------------------------------------------------------------

type portfolioCardDTO struct {
	ProjectID    uuid.UUID  `json:"project_id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	Currency     string     `json:"currency"`
	ProgressPct  float64    `json:"progress_pct"` // fiziki ilerleme (EV/BAC) — herkese açık
	SPI          *float64   `json:"spi,omitempty"` // yalnızca finansal izinle
	CPI          *float64   `json:"cpi,omitempty"`
	OpenFindings int        `json:"open_findings"`
	Pending      int        `json:"pending_approvals"` // hakediş+MAR+PR
	NetPayableCum *float64  `json:"net_payable_cum,omitempty"`

	// Zamansal ilerleme — künye tarihleri varsa herkese açık (parasal
	// değil, izin gerektirmez). Bitiş geçmişse DaysToEnd negatif döner.
	StartDate  *string  `json:"start_date,omitempty"`
	EndDate    *string  `json:"end_date,omitempty"`
	ElapsedPct *float64 `json:"elapsed_pct,omitempty"`
	DaysToEnd  *int     `json:"days_to_end,omitempty"`

	// Parasal ilerleme (AC/sözleşme bedeli) — SPI/CPI ile aynı finansal
	// izin kapısından geçer.
	ParasalPct *float64 `json:"parasal_pct,omitempty"`
}

func (h *Handler) Portfolio(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Görünürlük: proje üyeliği VEYA global projects.view GRANT'i
	// (projects.ListProjects ile birebir aynı kural).
	rows, err := h.pool.Query(ctx, `
		SELECT p.id, p.code, p.name, p.status, p.currency
		FROM projects p
		WHERE p.deleted_at IS NULL AND p.status <> 'Archived' AND (
			EXISTS (SELECT 1 FROM project_members pm
			        WHERE pm.project_id = p.id AND pm.user_id = $1 AND pm.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM user_permissions up
			           JOIN permissions pe ON pe.id = up.permission_id
			           WHERE up.user_id = $1 AND up.project_id IS NULL
			             AND up.effect = 'GRANT' AND pe.code = 'projects.view')
		)
		ORDER BY p.created_at DESC`, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	type proj struct {
		id                           uuid.UUID
		code, name, status, currency string
	}
	var projects []proj
	for rows.Next() {
		var p proj
		if err := rows.Scan(&p.id, &p.code, &p.name, &p.status, &p.currency); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		projects = append(projects, p)
	}
	rows.Close()

	now := time.Now().UTC()
	out := []portfolioCardDTO{}
	for _, p := range projects {
		// Taşeron temsilcisi portföy toplulaştırması görmez — o proje atlanır.
		sub, err := h.scopedSub(ctx, uid, p.id)
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		if sub != nil {
			continue
		}
		card := portfolioCardDTO{
			ProjectID: p.id, Code: p.code, Name: p.name,
			Status: p.status, Currency: p.currency,
		}
		evm, err := LoadEVM(ctx, h.pool, p.id, now)
		if err != nil {
			h.log.Error("portföy EVM derlenemedi", "err", err, "project", p.id)
			httpx.Internal(w, r)
			return
		}
		card.ProgressPct = evm.ProgressPct
		if evm.StartDate != nil && evm.EndDate != nil && evm.EndDate.After(*evm.StartDate) {
			sd, ed := evm.StartDate.Format("2006-01-02"), evm.EndDate.Format("2006-01-02")
			card.StartDate, card.EndDate = &sd, &ed
			elapsed := round2(now.Sub(*evm.StartDate).Hours() / evm.EndDate.Sub(*evm.StartDate).Hours() * 100)
			if elapsed < 0 {
				elapsed = 0
			}
			if elapsed > 100 {
				elapsed = 100
			}
			card.ElapsedPct = &elapsed
			days := int(evm.EndDate.Sub(now).Hours() / 24)
			card.DaysToEnd = &days
		}
		canFin, err := h.eval.Can(ctx, uid, &p.id, "reports.view_financial_reports")
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		if canFin {
			spi, cpi, ac := evm.SPI, evm.CPI, evm.AC
			card.SPI, card.CPI, card.NetPayableCum = &spi, &cpi, &ac
			if evm.MainContractAmount != nil && *evm.MainContractAmount > 0 {
				fp := evm.FinancialProgressPct
				card.ParasalPct = &fp
			}
		}
		if err := h.pool.QueryRow(ctx, `
			SELECT
			  (SELECT count(*) FROM ohs_findings
			   WHERE project_id=$1 AND deleted_at IS NULL AND status IN ('Open','InProgress')),
			  (SELECT count(*) FROM progress_payments
			   WHERE project_id=$1 AND deleted_at IS NULL AND status IN ('Submitted','SiteApproved'))
			+ (SELECT count(*) FROM material_approvals
			   WHERE project_id=$1 AND deleted_at IS NULL AND status IN ('Submitted','UnderReview'))
			+ (SELECT count(*) FROM purchase_requests
			   WHERE project_id=$1 AND deleted_at IS NULL AND status='Submitted')`, p.id).
			Scan(&card.OpenFindings, &card.Pending); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, card)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"portfolio": out})
}

// ---------------------------------------------------------------------------
// PV planı (aylık dağılım girişi) — GET herkese (finansal izinle), PUT yönetime
// ---------------------------------------------------------------------------

type pvEntryDTO struct {
	Month      string  `json:"month"` // YYYY-MM
	PlannedPct float64 `json:"planned_pct"`
}

func (h *Handler) GetPVPlan(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT to_char(month,'YYYY-MM'), planned_pct
		FROM pv_plan_entries
		WHERE project_id=$1 AND deleted_at IS NULL ORDER BY month`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []pvEntryDTO{}
	for rows.Next() {
		var e pvEntryDTO
		if err := rows.Scan(&e.Month, &e.PlannedPct); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, e)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"entries": out})
}

// PutPVPlan — dağılımın tamamını değiştirir (tam liste gelir; soft delete +
// yeniden yazım tek transaction'da). Toplam 100±0.5 olmalıdır.
func (h *Handler) PutPVPlan(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var in struct {
		Entries []pvEntryDTO `json:"entries"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	sum := 0.0
	seen := map[string]bool{}
	for i := range in.Entries {
		in.Entries[i].Month = strings.TrimSpace(in.Entries[i].Month)
		if _, err := time.Parse("2006-01", in.Entries[i].Month); err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"entries": "ay biçimi YYYY-MM olmalı: " + in.Entries[i].Month})
			return
		}
		if seen[in.Entries[i].Month] {
			httpx.ValidationFailed(w, r, map[string]string{"entries": "yinelenen ay: " + in.Entries[i].Month})
			return
		}
		seen[in.Entries[i].Month] = true
		if in.Entries[i].PlannedPct < 0 || in.Entries[i].PlannedPct > 100 {
			httpx.ValidationFailed(w, r, map[string]string{"entries": "planned_pct 0-100 aralığında olmalı"})
			return
		}
		sum += in.Entries[i].PlannedPct
	}
	if len(in.Entries) > 0 && (sum < 99.5 || sum > 100.5) {
		httpx.ValidationFailed(w, r, map[string]string{"entries": "dağılım toplamı 100 olmalı (±0.5); girilen toplam farklı"})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), `
		UPDATE pv_plan_entries SET deleted_at=now(), row_version=row_version+1
		WHERE project_id=$1 AND deleted_at IS NULL`, pid); err != nil {
		httpx.Internal(w, r)
		return
	}
	for _, e := range in.Entries {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO pv_plan_entries (project_id, month, planned_pct)
			VALUES ($1, to_date($2,'YYYY-MM'), $3)`, pid, e.Month, e.PlannedPct); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "pv_plan_entries", EntityID: pid.String(),
		Action: audit.ActionUpdate,
		After:  map[string]interface{}{"entries": in.Entries, "sum_pct": sum},
		IP:     m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"entries": in.Entries})
}

// ---------------------------------------------------------------------------
// Kontrol eşikleri (Plan §7: eşikler veri olarak tutulur)
// ---------------------------------------------------------------------------

type controlSettingsDTO struct {
	CPIMin           float64 `json:"cpi_min"`
	SPIMin           float64 `json:"spi_min"`
	FindingAgingDays int     `json:"finding_aging_days"`
}

var defaultControlSettings = controlSettingsDTO{CPIMin: 0.90, SPIMin: 0.90, FindingAgingDays: 14}

func loadControlSettings(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID) (controlSettingsDTO, error) {
	s := defaultControlSettings
	err := pool.QueryRow(ctx, `
		SELECT cpi_min, spi_min, finding_aging_days FROM project_control_settings
		WHERE project_id=$1 AND deleted_at IS NULL`, pid).
		Scan(&s.CPIMin, &s.SPIMin, &s.FindingAgingDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultControlSettings, nil
	}
	return s, err
}

func (h *Handler) GetControlSettings(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	s, err := loadControlSettings(r.Context(), h.pool, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"settings": s})
}

func (h *Handler) PutControlSettings(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var in controlSettingsDTO
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	fields := map[string]string{}
	if in.CPIMin <= 0 || in.CPIMin > 2 {
		fields["cpi_min"] = "0 ile 2 arasında olmalı"
	}
	if in.SPIMin <= 0 || in.SPIMin > 2 {
		fields["spi_min"] = "0 ile 2 arasında olmalı"
	}
	if in.FindingAgingDays < 1 {
		fields["finding_aging_days"] = "en az 1 gün olmalı"
	}
	if len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}
	// Upsert (yaşayan satır tekil; soft delete edilmişse yeni satır açılır).
	_, err := h.pool.Exec(r.Context(), `
		INSERT INTO project_control_settings (project_id, cpi_min, spi_min, finding_aging_days)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id) WHERE deleted_at IS NULL
		DO UPDATE SET cpi_min=$2, spi_min=$3, finding_aging_days=$4,
		              row_version=project_control_settings.row_version+1`,
		pid, in.CPIMin, in.SPIMin, in.FindingAgingDays)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "project_control_settings", EntityID: pid.String(),
		Action: audit.ActionUpdate,
		After:  map[string]interface{}{"cpi_min": in.CPIMin, "spi_min": in.SPIMin, "finding_aging_days": in.FindingAgingDays},
		IP:     m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"settings": in})
}
