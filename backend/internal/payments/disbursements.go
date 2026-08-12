package payments

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/paymentplans"
)

// ---------------------------------------------------------------------------
// Hakediş Ödeme Planı (kısmi ödeme) — Nakit Akış Faz B/C.
//
// Bir hakedişin birden çok kısmi ödemesi olabilir (nakit/havale/çek). Girilen
// ödeme şekli, taşeronun sözleşmesindeki varsayılanla AYNIYSA (veya sözleşmede
// varsayılan tanımlı değilse) satır doğrudan merkezi nakit hareketi defterine
// (cash_events) bir "out" satırı olarak yazılır — çekte olay tarihi keşide
// tarihidir, diğerlerinde kullanıcının girdiği ödeme tarihidir. FARKLIYSA,
// deftere hemen yazılmaz; bir onay talebi açılır (Faz C, `paymentplans`
// paketi) ve `payments.approve_plan_change` yetkisi olanlara bildirim gider.
// ---------------------------------------------------------------------------

type disbursementDTO struct {
	ID                uuid.UUID `json:"id"`
	ProgressPaymentID uuid.UUID `json:"progress_payment_id"`
	Amount            float64   `json:"amount"`
	PaymentMethod     string    `json:"payment_method"`
	EventDate         string    `json:"event_date"`
	CekKesideTarihi   *string   `json:"cek_keside_tarihi,omitempty"`
	Note              *string   `json:"note,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	PendingApproval   bool      `json:"pending_approval"`
}

const disbCols = `id, progress_payment_id, amount::float8, payment_method,
	to_char(event_date,'YYYY-MM-DD'), to_char(cek_keside_tarihi,'YYYY-MM-DD'), note, created_at,
	EXISTS(SELECT 1 FROM payment_plan_change_requests c
	       WHERE c.source_entity='progress_payment_disbursement'
	         AND c.source_id=progress_payment_disbursements.id AND c.status='pending')`

func scanDisb(row pgx.Row, d *disbursementDTO) error {
	return row.Scan(&d.ID, &d.ProgressPaymentID, &d.Amount, &d.PaymentMethod,
		&d.EventDate, &d.CekKesideTarihi, &d.Note, &d.CreatedAt, &d.PendingApproval)
}

func (h *Handler) ListDisbursements(w http.ResponseWriter, r *http.Request) {
	_, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ppid, ok := parseID(w, r, "paymentID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+disbCols+` FROM progress_payment_disbursements
		WHERE progress_payment_id=$1 AND deleted_at IS NULL
		ORDER BY event_date, created_at`, ppid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []disbursementDTO{}
	for rows.Next() {
		var d disbursementDTO
		if err := scanDisb(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"disbursements": out})
}

type disbReq struct {
	Amount          float64 `json:"amount"`
	PaymentMethod   string  `json:"payment_method"`
	EventDate       string  `json:"event_date"`
	CekKesideTarihi *string `json:"cek_keside_tarihi"`
	Note            *string `json:"note"`
}

func (h *Handler) CreateDisbursement(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ppid, ok := parseID(w, r, "paymentID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	var req disbReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	f := map[string]string{}
	if req.Amount <= 0 {
		f["amount"] = "0'dan büyük olmalı"
	}
	if req.PaymentMethod != "nakit" && req.PaymentMethod != "havale" && req.PaymentMethod != "cek" {
		f["payment_method"] = "nakit, havale veya cek olmalı"
	}
	if _, err := time.Parse("2006-01-02", req.EventDate); err != nil {
		f["event_date"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	}
	if req.PaymentMethod == "cek" {
		if req.CekKesideTarihi == nil || strings.TrimSpace(*req.CekKesideTarihi) == "" {
			f["cek_keside_tarihi"] = "çek için keşide tarihi zorunlu"
		} else if _, err := time.Parse("2006-01-02", *req.CekKesideTarihi); err != nil {
			f["cek_keside_tarihi"] = "geçerli bir tarih (YYYY-MM-DD) girin"
		}
	}
	if len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}

	// Nakit hareketi tarihi: çekte keşide tarihi, diğerlerinde ödeme tarihi.
	cashDate := req.EventDate
	if req.PaymentMethod == "cek" {
		cashDate = *req.CekKesideTarihi
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var periodNo int
	var subID uuid.UUID
	if err := tx.QueryRow(r.Context(),
		`SELECT period_no, subcontractor_id FROM progress_payments WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		ppid, pid).Scan(&periodNo, &subID); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	// Sözleşmedeki varsayılan ödeme şekli — en güncel sözleşme esas alınır.
	var defaultMethod *string
	if err := tx.QueryRow(r.Context(), `
		SELECT default_payment_method FROM contracts
		WHERE subcontractor_id=$1 AND project_id=$2 AND deleted_at IS NULL AND default_payment_method IS NOT NULL
		ORDER BY created_at DESC LIMIT 1`, subID, pid).Scan(&defaultMethod); err != nil && err != pgx.ErrNoRows {
		httpx.Internal(w, r)
		return
	}

	var d disbursementDTO
	err = scanDisb(tx.QueryRow(r.Context(), `
		INSERT INTO progress_payment_disbursements
			(progress_payment_id, amount, payment_method, event_date, cek_keside_tarihi, note, created_by)
		VALUES ($1,$2,$3,$4::date,NULLIF($5,'')::date,$6,$7)
		RETURNING `+disbCols,
		ppid, req.Amount, req.PaymentMethod, req.EventDate, strDerefPtr(req.CekKesideTarihi), req.Note, uid), &d)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	description := "Hakediş #" + itoa(periodNo) + " ödemesi"
	pendingApproval := defaultMethod != nil && *defaultMethod != req.PaymentMethod
	if pendingApproval {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO payment_plan_change_requests
				(project_id, source_entity, source_id, amount_snapshot, description_snapshot, default_method, requested_method, requested_by)
			VALUES ($1,'progress_payment_disbursement',$2,$3,$4,$5,$6,$7)`,
			pid, d.ID, req.Amount, description, *defaultMethod, req.PaymentMethod, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	} else {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
			VALUES ($1,'out','progress_payment_disbursement',$2,$3,$4,$5::date,$6,$7)`,
			pid, d.ID, description, req.Amount, cashDate, req.PaymentMethod, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "progress_payment_disbursements", EntityID: d.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"progress_payment_id": ppid, "amount": d.Amount}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}

	if pendingApproval {
		did := d.ID
		paymentplans.NotifyApprovers(r.Context(), h.pool, h.nt, pid, uid, notify.Input{
			Type: notify.TypePaymentPlanRequested, Title: description + " — ödeme şekli değişikliği onay bekliyor",
			Body:       "Varsayılan: " + *defaultMethod + " · İstenen: " + req.PaymentMethod,
			EntityType: "progress_payment_disbursement", EntityID: &did, ProjectID: &pid,
		})
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"disbursement": d, "pending_approval": pendingApproval})
}

func (h *Handler) DeleteDisbursement(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE progress_payment_disbursements SET deleted_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		  AND progress_payment_id IN (SELECT id FROM progress_payments WHERE project_id=$2)`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ödeme bulunamadı.", nil)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM cash_events WHERE source_entity='progress_payment_disbursement' AND source_id=$1`, id); err != nil {
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		DELETE FROM payment_plan_change_requests
		WHERE source_entity='progress_payment_disbursement' AND source_id=$1 AND status='pending'`, id); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func strDerefPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
