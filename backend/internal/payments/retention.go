package payments

// Faz 11 — Teminat (geçici kesinti) bakiyesi ve iade akışı.
//
// Sözleşme gereği taşerondan tutulan teminat, kabul aşamalarında iade edilir.
// Bugüne dek kesinti birikiyordu ama iade edilecek bir mekanizma yoktu; yani
// sözleşmeden doğan bir YÜKÜMLÜLÜK sistemde takipsiz kalıyordu.
//
// Bakiye türetilir (v_retention_balance): kesinleşmiş hakedişlerdeki geçici
// kesintiler eksi iadeler. Ayrı bir bakiye alanı TUTULMAZ ki iki kaynak arasında
// tutarsızlık oluşamasın.
//
// İade bir hakedişe bağlanırsa o hakedişte İLAVE olarak ödenecek tutarı artırır;
// bağımsız da kaydedilebilir (ör. banka teminat mektubunun iadesi).

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type retentionBalanceDTO struct {
	SubcontractorID string  `json:"subcontractor_id"`
	CompanyName     string  `json:"company_name"`
	TotalWithheld   float64 `json:"total_withheld"`
	TotalRefunded   float64 `json:"total_refunded"`
	Balance         float64 `json:"balance"`
}

type refundDTO struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Amount      float64  `json:"amount"`
	Stage       string   `json:"stage"`
	CatalogCode *string  `json:"catalog_code,omitempty"`
	DocumentID  *string  `json:"document_id,omitempty"`
	PaymentID   *string  `json:"progress_payment_id,omitempty"`
	Note        *string  `json:"note,omitempty"`
	CreatedBy   string   `json:"created_by_name"`
	CreatedAt   string   `json:"created_at"`
}

// İade aşamaları — katalogdaki refund_stage ile aynı dil.
var refundStages = map[string]bool{
	"ProvisionalAcceptance": true, // geçici kabul
	"FinalAcceptance":       true, // kesin kabul
	"ClearanceCertificate":  true, // ilişiksiz belgesi
	"WarrantyEnd":           true, // garanti süresi sonu
	"Other":                 true,
}

// RetentionBalances — GET /projects/{projectID}/retention
// Proje genelinde taşeron bazında teminat bakiyeleri ve iade geçmişi.
func (h *Handler) RetentionBalances(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	subFilter := r.URL.Query().Get("subcontractor_id")
	var rows pgx.Rows
	var err error
	if subFilter != "" {
		subID, perr := uuid.Parse(subFilter)
		if perr != nil {
			httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "geçersiz UUID"})
			return
		}
		rows, err = h.pool.Query(r.Context(), `
			SELECT subcontractor_id, company_name,
			       total_withheld::float8, total_refunded::float8, balance::float8
			FROM v_retention_balance
			WHERE project_id = $1 AND subcontractor_id = $2
			ORDER BY balance DESC, company_name`, pid, subID)
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT subcontractor_id, company_name,
			       total_withheld::float8, total_refunded::float8, balance::float8
			FROM v_retention_balance
			WHERE project_id = $1
			ORDER BY balance DESC, company_name`, pid)
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []retentionBalanceDTO{}
	for rows.Next() {
		var b retentionBalanceDTO
		if err := rows.Scan(&b.SubcontractorID, &b.CompanyName,
			&b.TotalWithheld, &b.TotalRefunded, &b.Balance); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, b)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"balances": out})
}

// ListRefunds — GET /projects/{projectID}/subcontractors/{subID}/refunds
func (h *Handler) ListRefunds(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	subID, ok := parseID(w, r, "subID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT rf.id, rf.description, rf.amount::float8, rf.stage, rf.catalog_code,
		       rf.document_id, rf.progress_payment_id, rf.note, u.full_name,
		       to_char(rf.created_at,'YYYY-MM-DD')
		FROM deduction_refunds rf
		JOIN users u ON u.id = rf.created_by
		WHERE rf.project_id=$1 AND rf.subcontractor_id=$2 AND rf.deleted_at IS NULL
		ORDER BY rf.created_at DESC`, pid, subID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []refundDTO{}
	for rows.Next() {
		var d refundDTO
		var docID, payID *uuid.UUID
		if err := rows.Scan(&d.ID, &d.Description, &d.Amount, &d.Stage, &d.CatalogCode,
			&docID, &payID, &d.Note, &d.CreatedBy, &d.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		if docID != nil {
			s := docID.String()
			d.DocumentID = &s
		}
		if payID != nil {
			s := payID.String()
			d.PaymentID = &s
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"refunds": out})
}

type createRefundReq struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Stage       string  `json:"stage"`
	CatalogCode string  `json:"catalog_code"`
	DocumentID  string  `json:"document_id"`
	PaymentID   string  `json:"progress_payment_id"`
	Note        string  `json:"note"`
}

// CreateRefund — POST /projects/{projectID}/subcontractors/{subID}/refunds
// Teminat iadesi kaydeder. Bakiyeyi aşan iade kabul edilmez.
func (h *Handler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	subID, ok := parseID(w, r, "subID")
	if !ok {
		return
	}
	var req createRefundReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Description) == "" {
		errs["description"] = "açıklama zorunludur"
	}
	if req.Amount <= 0 {
		errs["amount"] = "tutar sıfırdan büyük olmalıdır"
	}
	if !refundStages[req.Stage] {
		errs["stage"] = "geçersiz iade aşaması"
	}
	// Kabul aşamasına dayanan iadelerde belge zorunludur: iade, kabul tutanağına
	// dayanmalıdır ki denetimde gerekçesi gösterilebilsin.
	if (req.Stage == "ProvisionalAcceptance" || req.Stage == "FinalAcceptance") &&
		strings.TrimSpace(req.DocumentID) == "" {
		errs["document_id"] = "kabul tutanağı/belgesi zorunludur"
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}

	// Bakiye kontrolü: mevcut teminattan fazlası iade edilemez.
	var balance float64
	if err := h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(balance,0)::float8 FROM v_retention_balance WHERE subcontractor_id=$1`,
		subID).Scan(&balance); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.Internal(w, r)
		return
	}
	if req.Amount > balance+0.005 {
		httpx.ValidationFailed(w, r, map[string]string{
			"amount": "iade tutarı teminat bakiyesini aşamaz",
		})
		return
	}

	m := audit.MetaFrom(r.Context())
	actorID, err := uuid.Parse(m.ActorID)
	if err != nil {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum bulunamadı.", nil)
		return
	}

	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO deduction_refunds
		  (project_id, subcontractor_id, progress_payment_id, catalog_code, description,
		   amount, stage, document_id, note, created_by)
		VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),$5,$6,$7,NULLIF($8,'')::uuid,NULLIF($9,''),$10)
		RETURNING id`,
		pid, subID, req.PaymentID, req.CatalogCode, strings.TrimSpace(req.Description),
		req.Amount, req.Stage, req.DocumentID, req.Note, actorID).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "deduction_refunds", EntityID: id.String(),
		Action: audit.ActionInsert,
		After:  map[string]interface{}{"amount": req.Amount, "stage": req.Stage},
	})

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id": id.String(), "amount": req.Amount, "stage": req.Stage,
	})
}

