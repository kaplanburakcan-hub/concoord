// Package notify — merkezi bildirim motoru (Plan §8, Faz 4).
//
// Tek giriş noktası Service.Send'dir: in-app bildirim kaydını yazar ve
// kullanıcının kanal tercihlerine göre e-posta/SMS işlerini kuyruğa atar.
// Gönderim ASENKRONDUR (worker); API isteği hiçbir zaman SMTP/SMS beklemez.
// Sonraki tüm modüller (MAR, İSG, hakediş) bu servisi kullanır — her modül
// kendi bildirim kodunu yazmaz (Plan §9, mimari borç önlemi).
package notify

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Bildirim türleri — Faz 4 görev seti. Sonraki fazlar kendi türlerini ekler.
const (
	TypeTaskAssigned = "task_assigned"
	TypeTaskMention  = "task_mention"
	TypeTaskComment  = "task_comment"
	TypeTaskDeadline = "task_deadline"

	// Faz 5 — Malzeme Onay Süreci (MAR)
	TypeMARSubmitted   = "mar_submitted"    // yeni MAR sunuldu → inceleyicilere
	TypeMARUnderReview = "mar_under_review" // incelemeye alındı / karara sunuldu → karar vericilere
	TypeMARDecided     = "mar_decided"      // karar verildi → talep sahibi + taşeron temsilcileri

	// Faz 6 — Saha raporlama
	TypeWeeklyReportReady  = "weekly_report_ready"  // PDF üretildi → üreten kullanıcı
	TypeWeeklyReportFailed = "weekly_report_failed" // PDF üretimi başarısız → üreten kullanıcı

	// Faz 7 — Satınalma ve tedarik zinciri
	TypePRSubmitted = "pr_submitted" // PR onaya sunuldu → onay yetkilileri
	TypePRDecided   = "pr_decided"   // PR onaylandı/reddedildi → talep sahibi
	TypePOOverdue   = "po_overdue"   // beklenen teslim tarihi geçti → sipariş sahibi + PR sahibi

	// Faz 8 — İSG
	TypeOHSPenaltyIssued  = "ohs_penalty_issued"  // ceza tutanağı kesildi → taşeron temsilcisi + PY
	TypeOHSFindingOverdue = "ohs_finding_overdue" // bulgu termini geçti → raporlayan

	// Nakit Akış Faz C — ödeme planı değişikliği onay akışı
	TypePaymentPlanRequested = "payment_plan_requested" // varsayılan dışı ödeme şekli onaya sunuldu → onay yetkilileri
	TypePaymentPlanDecided   = "payment_plan_decided"   // karar verildi → talep sahibi

	// Faz 9 — Dashboard, EVM ve yönetim raporlaması
	TypeMonthlyReportReady  = "monthly_report_ready"  // aylık PDF üretildi → üreten kullanıcı
	TypeMonthlyReportFailed = "monthly_report_failed" // aylık PDF başarısız → üreten kullanıcı
	TypeEVMThresholdAlert   = "evm_threshold_alert"   // CPI/SPI eşik ihlali → PY
	TypeMilestoneLateAlert  = "milestone_late_alert"  // geciken milestone → PY
	TypeFindingAgingAlert   = "finding_aging_alert"   // yaşlanan açık İSG bulgusu → PY

	// Makine/Ekipman/Araç Envanteri Faz B — proje-arası transfer talebi
	TypeEquipmentTransferRequested = "equipment_transfer_requested" // talep açıldı → kaynak projenin onay yetkilileri
	TypeEquipmentTransferDecided   = "equipment_transfer_decided"   // karar verildi → talep sahibi
	TypeEquipmentDelivered         = "equipment_delivered"          // hedef projede teslim alındı → onay yetkilileri

	// Makine/Ekipman/Araç Envanteri Faz E — kiralama sözleşmesi hatırlatması
	TypeRentalContractMissing = "rental_contract_missing" // kiralık ekipmanın sözleşmesi 7+ gündür yüklenmedi → onay yetkilileri

	// Saha Tutanakları — Zimmet Tutanağı
	TypeZimmetCreated = "zimmet_created" // zimmet tutanağı oluşturuldu → ilgili firmanın tanımlı kullanıcıları
)

const (
	ChannelInApp = "InApp"
	ChannelEmail = "Email"
	ChannelSMS   = "SMS"
)

type Service struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{pool: pool, log: log}
}

// Input — tek bir bildirim olayı; birden çok alıcıya yayılır.
type Input struct {
	UserIDs    []uuid.UUID
	Type       string
	Title      string
	Body       string
	EntityType string
	EntityID   *uuid.UUID
	ProjectID  *uuid.UUID
}

// kanal varsayılanları: satır yoksa geçerli olan değerler (Plan: SMS adaptörü
// hazır ama varsayılan kapalı; sağlayıcı anlaşması yapılınca kullanıcı açar).
var channelDefaults = map[string]bool{
	ChannelInApp: true,
	ChannelEmail: true,
	ChannelSMS:   false,
}

// Send — alıcı başına: (1) tercih çöz, (2) InApp kaydı yaz, (3) Email/SMS
// açıksa kuyruk işi + kanal kaydı oluştur. Hatalar loglanır ama çağıranın
// isteğini düşürmez: bildirim, iş değişikliğinin yan etkisidir.
func (s *Service) Send(ctx context.Context, in Input) {
	for _, uid := range dedupe(in.UserIDs) {
		prefs, err := s.preferences(ctx, uid)
		if err != nil {
			s.log.Error("bildirim tercihi okunamadı", "err", err, "user", uid)
			prefs = channelDefaults
		}
		if prefs[ChannelInApp] {
			if _, err := s.insert(ctx, uid, in, ChannelInApp); err != nil {
				s.log.Error("in-app bildirim yazılamadı", "err", err, "user", uid)
			}
		}
		if prefs[ChannelEmail] {
			s.enqueueChannel(ctx, uid, in, ChannelEmail, "send_email")
		}
		if prefs[ChannelSMS] {
			s.enqueueChannel(ctx, uid, in, ChannelSMS, "send_sms")
		}
	}
}

func (s *Service) enqueueChannel(ctx context.Context, uid uuid.UUID, in Input, channel, jobKind string) {
	nid, err := s.insert(ctx, uid, in, channel)
	if err != nil {
		s.log.Error("kanal bildirimi yazılamadı", "err", err, "channel", channel, "user", uid)
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"notification_id": nid.String(),
		"user_id":         uid.String(),
	})
	if err := Enqueue(ctx, s.pool, jobKind, payload); err != nil {
		s.log.Error("bildirim işi kuyruğa atılamadı", "err", err, "kind", jobKind)
	}
}

func (s *Service) insert(ctx context.Context, uid uuid.UUID, in Input, channel string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, title, body, entity_type, entity_id, project_id, channel, sent_at)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7,$8,
		        CASE WHEN $8='InApp' THEN now() ELSE NULL END)
		RETURNING id`,
		uid, in.Type, in.Title, in.Body, in.EntityType, in.EntityID, in.ProjectID, channel).Scan(&id)
	return id, err
}

// preferences — kullanıcının kanal haritası; eksik kanallar varsayılanla dolar.
func (s *Service) preferences(ctx context.Context, uid uuid.UUID) (map[string]bool, error) {
	out := map[string]bool{}
	for k, v := range channelDefaults {
		out[k] = v
	}
	rows, err := s.pool.Query(ctx,
		`SELECT channel, enabled FROM notification_preferences WHERE user_id=$1`, uid)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var ch string
		var en bool
		if err := rows.Scan(&ch, &en); err != nil {
			return out, err
		}
		out[ch] = en
	}
	return out, rows.Err()
}

func dedupe(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
