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
)

// ---------------------------------------------------------------------------
// Hakediş Ödeme Planı (kısmi ödeme) — Nakit Akış Faz B.
//
// Bir hakedişin birden çok kısmi ödemesi olabilir (nakit/havale/çek).
// Her satır oluşturulduğunda merkezi nakit hareketi defterine (cash_events)
// bir "out" satırı yazılır — çekte olay tarihi keşide tarihidir, diğerlerinde
// kullanıcının girdiği ödeme tarihidir. Ödeme şekli sözleşme varsayılanından
// farklıysa onay akışına düşürme (Faz C) henüz uygulanmadı — bu fazda her
// satır doğrudan deftere yazılır.
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
}

const disbCols = `id, progress_payment_id, amount::float8, payment_method,
	to_char(event_date,'YYYY-MM-DD'), to_char(cek_keside_tarihi,'YYYY-MM-DD'), note, created_at`

func scanDisb(row pgx.Row, d *disbursementDTO) error {
	return row.Scan(&d.ID, &d.ProgressPaymentID, &d.Amount, &d.PaymentMethod,
		&d.EventDate, &d.CekKesideTarihi, &d.Note, &d.CreatedAt)
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
	if err := tx.QueryRow(r.Context(),
		`SELECT period_no FROM progress_payments WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		ppid, pid).Scan(&periodNo); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
			return
		}
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

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
		VALUES ($1,'out','progress_payment_disbursement',$2,$3,$4,$5::date,$6,$7)`,
		pid, d.ID, "Hakediş #"+itoa(periodNo)+" ödemesi", req.Amount, cashDate, req.PaymentMethod, uid); err != nil {
		httpx.Internal(w, r)
		return
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
	httpx.JSON(w, http.StatusCreated, map[string]any{"disbursement": d})
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
