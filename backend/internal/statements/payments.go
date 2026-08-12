package statements

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/paymentplans"
)

// Tedarikçi ekstresi ödeme planı (kısmi ödeme) — Nakit Akış Faz B/C.
// Girilen ödeme şekli, ekstredeki tedarikçi adına isim eşleşmesiyle bulunan
// `tedarikciler` kaydının varsayılanıyla AYNIYSA (ya da eşleşen kayıt/
// varsayılan yoksa) cash_events'e doğrudan yazılır. FARKLIYSA bir onay
// talebi açılır (Faz C). Not: supplier_statements'ta gerçek bir tedarikci_id
// FK'si yok (serbest metin isim) — bu yüzden eşleşme isim bazlı, mevcut
// mimarideki aynı desenin devamı.

type StatementPayment struct {
	ID              string  `json:"id"`
	StatementID     string  `json:"statement_id"`
	Amount          float64 `json:"amount"`
	PaymentMethod   string  `json:"payment_method"`
	EventDate       string  `json:"event_date"`
	CekKesideTarihi *string `json:"cek_keside_tarihi,omitempty"`
	Note            *string `json:"note,omitempty"`
	CreatedAt       string  `json:"created_at"`
	PendingApproval bool    `json:"pending_approval"`
}

const stmtPayCols = `id, statement_id, amount::float8, payment_method,
	to_char(event_date,'YYYY-MM-DD'), to_char(cek_keside_tarihi,'YYYY-MM-DD'), note,
	to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
	EXISTS(SELECT 1 FROM payment_plan_change_requests c
	       WHERE c.source_entity='supplier_payment'
	         AND c.source_id=supplier_statement_payments.id AND c.status='pending')`

func scanStmtPay(row pgx.Row, p *StatementPayment) error {
	return row.Scan(&p.ID, &p.StatementID, &p.Amount, &p.PaymentMethod,
		&p.EventDate, &p.CekKesideTarihi, &p.Note, &p.CreatedAt, &p.PendingApproval)
}

func (h *Handler) ListStatementPayments(w http.ResponseWriter, r *http.Request) {
	stmtID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ekstre ID.", nil)
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT `+stmtPayCols+` FROM supplier_statement_payments
		WHERE statement_id=$1 AND deleted_at IS NULL
		ORDER BY event_date, created_at`, stmtID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []StatementPayment{}
	for rows.Next() {
		var p StatementPayment
		if err := scanStmtPay(rows, &p); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"payments": out})
}

type stmtPayReq struct {
	Amount          float64 `json:"amount"`
	PaymentMethod   string  `json:"payment_method"`
	EventDate       string  `json:"event_date"`
	CekKesideTarihi *string `json:"cek_keside_tarihi"`
	Note            *string `json:"note"`
}

func (h *Handler) CreateStatementPayment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	stmtID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ekstre ID.", nil)
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	var req stmtPayReq
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

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var tedarikciAdi, ekstreNo string
	if err := tx.QueryRow(r.Context(),
		`SELECT tedarikci_adi, ekstre_no FROM supplier_statements WHERE id=$1 AND project_id=$2`,
		stmtID, pid).Scan(&tedarikciAdi, &ekstreNo); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ekstre bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	// İsim eşleşmesiyle varsayılan ödeme şekli (gerçek FK yok).
	var defaultMethod *string
	if err := tx.QueryRow(r.Context(), `
		SELECT default_payment_method FROM tedarikciler
		WHERE project_id=$1 AND company_name=$2 AND deleted_at IS NULL AND default_payment_method IS NOT NULL
		ORDER BY created_at DESC LIMIT 1`, pid, tedarikciAdi).Scan(&defaultMethod); err != nil && err != pgx.ErrNoRows {
		httpx.Internal(w, r)
		return
	}

	var p StatementPayment
	cekVal := ""
	if req.CekKesideTarihi != nil {
		cekVal = *req.CekKesideTarihi
	}
	if err := scanStmtPay(tx.QueryRow(r.Context(), `
		INSERT INTO supplier_statement_payments
			(statement_id, amount, payment_method, event_date, cek_keside_tarihi, note, created_by)
		VALUES ($1,$2,$3,$4::date,NULLIF($5,'')::date,$6,$7)
		RETURNING `+stmtPayCols,
		stmtID, req.Amount, req.PaymentMethod, req.EventDate, cekVal, req.Note, uid), &p); err != nil {
		httpx.Internal(w, r)
		return
	}

	description := tedarikciAdi + " — " + ekstreNo
	pendingApproval := defaultMethod != nil && *defaultMethod != req.PaymentMethod
	if pendingApproval {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO payment_plan_change_requests
				(project_id, source_entity, source_id, amount_snapshot, description_snapshot, default_method, requested_method, requested_by)
			VALUES ($1,'supplier_payment',$2,$3,$4,$5,$6,$7)`,
			pid, p.ID, req.Amount, description, *defaultMethod, req.PaymentMethod, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	} else {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
			VALUES ($1,'out','supplier_payment',$2,$3,$4,$5::date,$6,$7)`,
			pid, p.ID, description, req.Amount, cashDate, req.PaymentMethod, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}

	if pendingApproval {
		payID, err := uuid.Parse(p.ID)
		if err == nil {
			paymentplans.NotifyApprovers(r.Context(), h.db, h.nt, pid, uid, notify.Input{
				Type: notify.TypePaymentPlanRequested, Title: description + " — ödeme şekli değişikliği onay bekliyor",
				Body:       "Varsayılan: " + *defaultMethod + " · İstenen: " + req.PaymentMethod,
				EntityType: "supplier_payment", EntityID: &payID, ProjectID: &pid,
			})
		}
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"payment": p, "pending_approval": pendingApproval})
}

func (h *Handler) DeleteStatementPayment(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	payID, err := uuid.Parse(chi.URLParam(r, "paymentID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ödeme ID.", nil)
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE supplier_statement_payments SET deleted_at=now()
		WHERE id=$1 AND deleted_at IS NULL
		  AND statement_id IN (SELECT id FROM supplier_statements WHERE project_id=$2)`, payID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ödeme bulunamadı.", nil)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM cash_events WHERE source_entity='supplier_payment' AND source_id=$1`, payID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		DELETE FROM payment_plan_change_requests
		WHERE source_entity='supplier_payment' AND source_id=$1 AND status='pending'`, payID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
