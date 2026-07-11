// İPKS Worker — arka plan işleri (Faz 4).
//
// İki görev:
//  1. İş kuyruğu (job_queue): send_email / send_sms işlerini FOR UPDATE SKIP
//     LOCKED ile çeker, kanal göndericisine iletir, başarı/başarısızlığı yazar
//     (başarısızlıkta üstel geri çekilme, tavan 5 deneme).
//  2. Deadline hatırlatıcı: termini 24 saat içinde dolan (veya geçmiş) ve Done
//     olmayan görevlerin atananlarına task_deadline bildirimi üretir; görev
//     deadline_notified_at ile işaretlenir (tekrarlanmaz; termin değişirse
//     API alanı sıfırlar ve hatırlatma yeniden kurulur).
//
// Faz 6+: PDF üretimi ve rapor derleme işleri aynı kuyruğa yeni 'kind' olarak eklenir.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/config"
	"github.com/ipks/ipks/backend/internal/dashboard"
	"github.com/ipks/ipks/backend/internal/db"
	"github.com/ipks/ipks/backend/internal/logger"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/ohs"
	"github.com/ipks/ipks/backend/internal/procurement"
	"github.com/ipks/ipks/backend/internal/reports"
	"github.com/ipks/ipks/backend/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel, cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("veritabanı bağlantısı kurulamadı", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	email := &notify.EmailSender{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
	}
	sms := notify.NewSMSSender(cfg.SMSProvider, cfg.SMSAPIURL, cfg.SMSUser, cfg.SMSPass, cfg.SMSHeader, log)
	nt := notify.NewService(pool, log)

	// Faz 6: haftalık PDF'lerin MinIO'ya yüklenmesi için depolama istemcisi.
	store := storage.New(storage.Config{
		Endpoint:       cfg.S3Endpoint,
		PublicEndpoint: cfg.S3PublicEndpoint,
		AccessKey:      cfg.S3AccessKey,
		SecretKey:      cfg.S3SecretKey,
		Bucket:         cfg.S3Bucket,
		Region:         cfg.S3Region,
		UseSSL:         cfg.S3UseSSL,
	})

	w := &worker{pool: pool, log: log, email: email, sms: sms, nt: nt, store: store}
	log.Info("İPKS worker başladı", "env", cfg.Env,
		"smtp", email.Configured(), "sms_adapter", sms.Name())

	jobTick := time.NewTicker(3 * time.Second)
	defer jobTick.Stop()
	deadlineTick := time.NewTicker(15 * time.Minute)
	defer deadlineTick.Stop()
	// Faz 7: geciken sipariş taraması (gecelik değişen veri; saatlik yeter).
	overdueTick := time.NewTicker(time.Hour)
	defer overdueTick.Stop()
	// Faz 9: eşik tabanlı uyarı taraması (EVM ağır sorgudur; 6 saat yeter —
	// tekilleştirme defteri zaten ay içinde tekrarı engeller).
	alertTick := time.NewTicker(6 * time.Hour)
	defer alertTick.Stop()

	// Açılışta bir kez deadline taraması (worker uzun süre kapalı kaldıysa).
	w.remindDeadlines(ctx)
	procurement.NotifyOverduePOs(ctx, pool, nt, log)
	// Faz 8: açılışta bir kez İSG termin taraması.
	ohs.NotifyOverdueFindings(ctx, pool, nt, log)
	// Faz 9: açılışta bir kez kontrol eşiği taraması (CPI/SPI, milestone, bulgu).
	dashboard.RunControlAlerts(ctx, pool, nt, log)

	for {
		select {
		case <-ctx.Done():
			log.Info("worker kapanıyor")
			return
		case <-jobTick.C:
			w.drainJobs(ctx)
		case <-deadlineTick.C:
			w.remindDeadlines(ctx)
		case <-overdueTick.C:
			procurement.NotifyOverduePOs(ctx, pool, w.nt, log)
			// Faz 8: termini geçen İSG bulguları (aynı saatlik ritim).
			ohs.NotifyOverdueFindings(ctx, pool, w.nt, log)
		case <-alertTick.C:
			// Faz 9: CPI/SPI eşik ihlali, geciken milestone, yaşlanan bulgu.
			dashboard.RunControlAlerts(ctx, pool, w.nt, log)
		}
	}
}

type worker struct {
	pool  *pgxpool.Pool
	log   *slog.Logger
	email *notify.EmailSender
	sms   notify.SMSSender
	nt    *notify.Service
	store *storage.Client
}

// drainJobs — hazır işleri boşalana kadar (tur başına en çok 50) işler.
func (w *worker) drainJobs(ctx context.Context) {
	for i := 0; i < 50; i++ {
		job, err := notify.Dequeue(ctx, w.pool)
		if err != nil {
			w.log.Error("kuyruktan iş alınamadı", "err", err)
			return
		}
		if job == nil {
			return // kuyruk boş
		}
		if err := w.handle(ctx, job); err != nil {
			w.log.Warn("iş başarısız", "id", job.ID, "kind", job.Kind, "attempt", job.Attempts, "err", err)
			if ferr := notify.Fail(ctx, w.pool, job, err); ferr != nil {
				w.log.Error("iş durumu güncellenemedi", "id", job.ID, "err", ferr)
			}
			continue
		}
		if cerr := notify.Complete(ctx, w.pool, job.ID); cerr != nil {
			w.log.Error("iş tamamlandı işaretlenemedi", "id", job.ID, "err", cerr)
		}
	}
}

type notifJobPayload struct {
	NotificationID string `json:"notification_id"`
	UserID         string `json:"user_id"`
}

func (w *worker) handle(ctx context.Context, job *notify.Job) error {
	switch job.Kind {
	case "send_email":
		return w.sendEmailJob(ctx, job.Payload)
	case "send_sms":
		return w.sendSMSJob(ctx, job.Payload)
	case reports.JobKindWeeklyPDF:
		// Faz 6: haftalık ilerleme raporu PDF üretimi (snapshot → PDF → MinIO).
		return reports.ProcessWeeklyPDF(ctx, w.pool, w.store, w.nt, w.log, job.Payload)
	case dashboard.JobKindMonthlyPDF:
		// Faz 9: aylık yönetim raporu PDF üretimi (aynı snapshot deseni).
		return dashboard.ProcessMonthlyPDF(ctx, w.pool, w.store, w.nt, w.log, job.Payload)
	default:
		// Bilinmeyen tür: kalıcı hata (yeniden deneme anlamsız) — loglanıp tamamlanır.
		w.log.Warn("bilinmeyen iş türü, atlanıyor", "kind", job.Kind, "id", job.ID)
		return nil
	}
}

// loadNotification — kanal kaydını ve alıcı iletişim bilgisini okur.
func (w *worker) loadNotification(ctx context.Context, payload json.RawMessage) (nid uuid.UUID, title, body, email, phone string, err error) {
	var p notifJobPayload
	if err = json.Unmarshal(payload, &p); err != nil {
		return
	}
	if nid, err = uuid.Parse(p.NotificationID); err != nil {
		return
	}
	var b *string
	err = w.pool.QueryRow(ctx, `
		SELECT n.title, n.body, u.email, COALESCE(u.phone,'')
		FROM notifications n JOIN users u ON u.id = n.user_id
		WHERE n.id=$1 AND n.deleted_at IS NULL AND n.sent_at IS NULL`, nid).
		Scan(&title, &b, &email, &phone)
	if b != nil {
		body = *b
	}
	return
}

func (w *worker) markSent(ctx context.Context, nid uuid.UUID) {
	if _, err := w.pool.Exec(ctx,
		`UPDATE notifications SET sent_at=now() WHERE id=$1`, nid); err != nil {
		w.log.Error("sent_at yazılamadı", "notification", nid, "err", err)
	}
}

func (w *worker) sendEmailJob(ctx context.Context, payload json.RawMessage) error {
	nid, title, body, to, _, err := w.loadNotification(ctx, payload)
	if err != nil {
		// Kayıt yoksa (zaten gönderilmiş/silinmiş) işi başarılı say — idempotentlik.
		w.log.Warn("e-posta bildirimi bulunamadı, atlanıyor", "err", err)
		return nil
	}
	if !w.email.Configured() {
		w.log.Info("SMTP yapılandırılmamış; e-posta loglandı", "to", to, "subject", title)
		w.markSent(ctx, nid)
		return nil
	}
	if err := w.email.Send(to, "[İPKS] "+title, body); err != nil {
		return fmt.Errorf("smtp gönderim hatası: %w", err)
	}
	w.markSent(ctx, nid)
	return nil
}

func (w *worker) sendSMSJob(ctx context.Context, payload json.RawMessage) error {
	nid, title, _, _, phone, err := w.loadNotification(ctx, payload)
	if err != nil {
		w.log.Warn("SMS bildirimi bulunamadı, atlanıyor", "err", err)
		return nil
	}
	if phone == "" {
		w.log.Info("kullanıcının telefonu yok; SMS atlandı", "notification", nid)
		w.markSent(ctx, nid)
		return nil
	}
	if err := w.sms.Send(ctx, phone, "İPKS: "+title); err != nil {
		return fmt.Errorf("sms gönderim hatası: %w", err)
	}
	w.markSent(ctx, nid)
	return nil
}

// remindDeadlines — Plan Faz 4 kabul kriteri: "deadline yaklaşınca otomatik
// hatırlatma üretiliyor". Kural: due_date <= yarın, statü Done değil, daha önce
// hatırlatılmamış. Atanan yoksa görevi oluşturan bilgilendirilir.
func (w *worker) remindDeadlines(ctx context.Context) {
	rows, err := w.pool.Query(ctx, `
		SELECT t.id, t.project_id, t.title, t.due_date, t.created_by
		FROM tasks t
		WHERE t.deleted_at IS NULL AND t.status <> 'Done'
		  AND t.due_date IS NOT NULL AND t.due_date <= (CURRENT_DATE + 1)
		  AND t.deadline_notified_at IS NULL
		LIMIT 200`)
	if err != nil {
		w.log.Error("deadline taraması başarısız", "err", err)
		return
	}
	type due struct {
		id, pid, createdBy uuid.UUID
		title              string
		dueDate            time.Time
	}
	items := []due{}
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.pid, &d.title, &d.dueDate, &d.createdBy); err != nil {
			rows.Close()
			w.log.Error("deadline satırı okunamadı", "err", err)
			return
		}
		items = append(items, d)
	}
	rows.Close()

	for _, d := range items {
		recipients := []uuid.UUID{}
		arows, err := w.pool.Query(ctx, `SELECT user_id FROM task_assignees WHERE task_id=$1`, d.id)
		if err != nil {
			w.log.Error("atananlar okunamadı", "task", d.id, "err", err)
			continue
		}
		for arows.Next() {
			var u uuid.UUID
			if err := arows.Scan(&u); err == nil {
				recipients = append(recipients, u)
			}
		}
		arows.Close()
		if len(recipients) == 0 {
			recipients = append(recipients, d.createdBy)
		}

		verb := "yaklaşıyor"
		if d.dueDate.Before(time.Now().Truncate(24 * time.Hour)) {
			verb = "geçti"
		}
		taskID := d.id
		pid := d.pid
		w.nt.Send(ctx, notify.Input{
			UserIDs: recipients, Type: notify.TypeTaskDeadline,
			Title: "Görev termini " + verb + ": " + d.title,
			Body:  "Termin: " + d.dueDate.Format("02.01.2006"),
			EntityType: "tasks", EntityID: &taskID, ProjectID: &pid,
		})
		if _, err := w.pool.Exec(ctx,
			`UPDATE tasks SET deadline_notified_at=now() WHERE id=$1`, d.id); err != nil {
			w.log.Error("deadline işareti yazılamadı", "task", d.id, "err", err)
		}
	}
	if len(items) > 0 {
		w.log.Info("deadline hatırlatmaları üretildi", "count", len(items))
	}
}
