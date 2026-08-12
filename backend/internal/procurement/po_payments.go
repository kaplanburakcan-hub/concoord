package procurement

// Sipariş (PO) ödeme planı (kısmi ödeme) — Nakit Akış Faz B.
// Her satır oluşturulduğunda cash_events'e bir "out" satırı yazılır — çekte
// olay tarihi keşide tarihidir, diğerlerinde ödeme tarihidir.

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type poPaymentDTO struct {
	ID              string  `json:"id"`
	POID            string  `json:"po_id"`
	Amount          float64 `json:"amount"`
	PaymentMethod   string  `json:"payment_method"`
	EventDate       string  `json:"event_date"`
	CekKesideTarihi *string `json:"cek_keside_tarihi,omitempty"`
	Note            *string `json:"note,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

const poPayCols = `id, po_id, amount::float8, payment_method,
	to_char(event_date,'YYYY-MM-DD'), to_char(cek_keside_tarihi,'YYYY-MM-DD'), note,
	to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')`

func scanPOPay(row pgx.Row, p *poPaymentDTO) error {
	return row.Scan(&p.ID, &p.POID, &p.Amount, &p.PaymentMethod,
		&p.EventDate, &p.CekKesideTarihi, &p.Note, &p.CreatedAt)
}

func (h *Handler) ListPOPayments(w http.ResponseWriter, r *http.Request) {
	_, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	poID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+poPayCols+` FROM purchase_order_payments
		WHERE po_id=$1 AND deleted_at IS NULL
		ORDER BY event_date, created_at`, poID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []poPaymentDTO{}
	for rows.Next() {
		var p poPaymentDTO
		if err := scanPOPay(rows, &p); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"payments": out})
}

type poPayReq struct {
	Amount          float64 `json:"amount"`
	PaymentMethod   string  `json:"payment_method"`
	EventDate       string  `json:"event_date"`
	CekKesideTarihi *string `json:"cek_keside_tarihi"`
	Note            *string `json:"note"`
}

func (h *Handler) CreatePOPayment(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	poID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	var req poPayReq
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

	var poNo, supplierName string
	if err := tx.QueryRow(r.Context(),
		`SELECT po_no, supplier_name FROM purchase_orders WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		poID, pid).Scan(&poNo, &supplierName); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Sipariş bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	cekVal := ""
	if req.CekKesideTarihi != nil {
		cekVal = *req.CekKesideTarihi
	}
	var p poPaymentDTO
	if err := scanPOPay(tx.QueryRow(r.Context(), `
		INSERT INTO purchase_order_payments
			(po_id, amount, payment_method, event_date, cek_keside_tarihi, note, created_by)
		VALUES ($1,$2,$3,$4::date,NULLIF($5,'')::date,$6,$7)
		RETURNING `+poPayCols,
		poID, req.Amount, req.PaymentMethod, req.EventDate, cekVal, req.Note, uid), &p); err != nil {
		httpx.Internal(w, r)
		return
	}

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
		VALUES ($1,'out','po_payment',$2,$3,$4,$5::date,$6,$7)`,
		pid, p.ID, poNo+" — "+supplierName, req.Amount, cashDate, req.PaymentMethod, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"payment": p})
}

func (h *Handler) DeletePOPayment(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	payID, ok := parseID(w, r, "paymentID")
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
		UPDATE purchase_order_payments SET deleted_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		  AND po_id IN (SELECT id FROM purchase_orders WHERE project_id=$2)`, payID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ödeme bulunamadı.", nil)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM cash_events WHERE source_entity='po_payment' AND source_id=$1`, payID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
