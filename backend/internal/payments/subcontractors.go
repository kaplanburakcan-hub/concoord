package payments

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

const maxImportBytes = 10 << 20 // 10 MB

// ---------------------------------------------------------------------------
// Taşeronlar
// ---------------------------------------------------------------------------

type subcontractorDTO struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	CompanyName   string    `json:"company_name"`
	TaxNo         *string   `json:"tax_no,omitempty"`
	ContactPerson *string   `json:"contact_person,omitempty"`
	Phone         *string   `json:"phone,omitempty"`
	Email         *string   `json:"email,omitempty"`
	Trade         *string   `json:"trade,omitempty"`
	RowVersion    int       `json:"row_version"`
	CreatedAt     time.Time `json:"created_at"`
}

const subCols = `id, project_id, company_name, tax_no, contact_person, phone, email, trade, row_version, created_at`

func scanSub(row pgx.Row, s *subcontractorDTO) error {
	return row.Scan(&s.ID, &s.ProjectID, &s.CompanyName, &s.TaxNo, &s.ContactPerson,
		&s.Phone, &s.Email, &s.Trade, &s.RowVersion, &s.CreatedAt)
}

func (h *Handler) ListSubcontractors(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	// active_on=YYYY-MM-DD verilirse yalnızca o tarihte geçerli sözleşmesi
	// olan taşeronlar döner (günlük rapor personel dropdown'ının kontrol
	// altyapısı — sözleşmesi bitmiş/başlamamış taşeron listelenmez).
	activeOn := strings.TrimSpace(r.URL.Query().Get("active_on"))
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+subCols+` FROM subcontractors s
		WHERE s.project_id=$1 AND s.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR s.id=$2)
		  AND ($3='' OR EXISTS (
		        SELECT 1 FROM contracts c
		        WHERE c.subcontractor_id = s.id AND c.deleted_at IS NULL
		          AND (c.start_date IS NULL OR c.start_date <= $3::date)
		          AND (COALESCE(c.revised_end_date, c.end_date) IS NULL
		               OR COALESCE(c.revised_end_date, c.end_date) >= $3::date)
		      ))
		ORDER BY s.company_name`, pid, scope, activeOn)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []subcontractorDTO{}
	for rows.Next() {
		var s subcontractorDTO
		if err := scanSub(rows, &s); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, s)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"subcontractors": out})
}

type subReq struct {
	CompanyName   string  `json:"company_name"`
	TaxNo         *string `json:"tax_no"`
	ContactPerson *string `json:"contact_person"`
	Phone         *string `json:"phone"`
	Email         *string `json:"email"`
	Trade         *string `json:"trade"`
	RowVersion    int     `json:"row_version"`
}

func (h *Handler) CreateSubcontractor(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req subReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	if f := ValidateSubcontractor(req.CompanyName); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var s subcontractorDTO
	err = scanSub(tx.QueryRow(r.Context(), `
		INSERT INTO subcontractors (project_id, company_name, tax_no, contact_person, phone, email, trade)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+subCols,
		pid, req.CompanyName, req.TaxNo, req.ContactPerson, req.Phone, req.Email, req.Trade), &s)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "subcontractors", EntityID: s.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid, "company_name": s.CompanyName}, IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"subcontractor": s})
}

func (h *Handler) GetSubcontractor(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	if scope != nil && *scope != sid {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu kayda erişiminiz yok.", nil)
		return
	}
	var s subcontractorDTO
	err := scanSub(h.pool.QueryRow(r.Context(),
		`SELECT `+subCols+` FROM subcontractors WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, sid, pid), &s)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Taşeron bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"subcontractor": s})
}

func (h *Handler) UpdateSubcontractor(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req subReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before subcontractorDTO
	err = scanSub(tx.QueryRow(r.Context(),
		`SELECT `+subCols+` FROM subcontractors WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, sid, pid), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Taşeron bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}
	name := before.CompanyName
	if strings.TrimSpace(req.CompanyName) != "" {
		name = strings.TrimSpace(req.CompanyName)
	}
	if f := ValidateSubcontractor(name); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	var s subcontractorDTO
	err = scanSub(tx.QueryRow(r.Context(), `
		UPDATE subcontractors SET
			company_name=$3,
			tax_no=COALESCE($4, tax_no),
			contact_person=COALESCE($5, contact_person),
			phone=COALESCE($6, phone),
			email=COALESCE($7, email),
			trade=COALESCE($8, trade),
			row_version=row_version+1
		WHERE id=$1 AND project_id=$2
		RETURNING `+subCols,
		sid, pid, name, req.TaxNo, req.ContactPerson, req.Phone, req.Email, req.Trade), &s)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "subcontractors", EntityID: sid.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"company_name": before.CompanyName},
		After:  map[string]interface{}{"company_name": s.CompanyName}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"subcontractor": s})
}

func (h *Handler) DeleteSubcontractor(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE subcontractors SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, sid, pid)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Taşerona bağlı sözleşme/hakediş var; önce onları arşivleyin.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Taşeron bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "subcontractors", EntityID: sid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Sözleşmeler (alt sözleşme arşivi)
// ---------------------------------------------------------------------------

type contractDTO struct {
	ID               uuid.UUID  `json:"id"`
	ProjectID        uuid.UUID  `json:"project_id"`
	SubcontractorID  *uuid.UUID `json:"subcontractor_id,omitempty"`
	ContractNo       string     `json:"contract_no"`
	Type             string     `json:"type"`
	ParentContractID *uuid.UUID `json:"parent_contract_id,omitempty"`
	Amount           *float64   `json:"amount,omitempty"`
	AdvanceAmount    *float64   `json:"advance_amount,omitempty"`
	RetentionPct     float64    `json:"retention_pct"`
	AdvanceRatePct   float64    `json:"advance_rate_pct"`
	SignDate         *time.Time `json:"sign_date,omitempty"`
	DocumentID       *uuid.UUID `json:"document_id,omitempty"`
	StartDate        *time.Time `json:"start_date,omitempty"`
	EndDate          *time.Time `json:"end_date,omitempty"`
	RevisedEndDate   *time.Time `json:"revised_end_date,omitempty"`
	IsMultiYear      bool       `json:"is_multi_year"`
	WithholdingPct   float64    `json:"withholding_pct"`
	// DefaultPaymentMethod — Nakit Akış Faz A: PO/hakediş/ekstre ödeme
	// planlarının varsayılan ödeme şeklini buradan alması içindir.
	DefaultPaymentMethod *string   `json:"default_payment_method,omitempty"`
	RowVersion           int       `json:"row_version"`
	CreatedAt            time.Time `json:"created_at"`
}

const contractCols = `id, project_id, subcontractor_id, contract_no, type, parent_contract_id,
	amount::float8, advance_amount::float8, retention_pct::float8, advance_rate_pct::float8,
	sign_date, document_id, start_date, end_date, revised_end_date,
	is_multi_year, withholding_pct::float8, default_payment_method, row_version, created_at`

func scanContract(row pgx.Row, c *contractDTO) error {
	return row.Scan(&c.ID, &c.ProjectID, &c.SubcontractorID, &c.ContractNo, &c.Type, &c.ParentContractID,
		&c.Amount, &c.AdvanceAmount, &c.RetentionPct, &c.AdvanceRatePct,
		&c.SignDate, &c.DocumentID, &c.StartDate, &c.EndDate, &c.RevisedEndDate,
		&c.IsMultiYear, &c.WithholdingPct, &c.DefaultPaymentMethod, &c.RowVersion, &c.CreatedAt)
}

// maskFinancials — view_financials yoksa tutar alanlarını gizler (Plan §4).
func (c *contractDTO) maskFinancials() {
	c.Amount = nil
	c.AdvanceAmount = nil
}

func (h *Handler) ListContracts(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	subFilter := r.URL.Query().Get("subcontractor_id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+contractCols+` FROM contracts
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND ($2::uuid IS NULL OR subcontractor_id=$2)
		  AND ($3='' OR subcontractor_id=$3::uuid)
		ORDER BY created_at DESC`, pid, scope, subFilter)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	fin := h.canFin(r, pid)
	out := []contractDTO{}
	for rows.Next() {
		var c contractDTO
		if err := scanContract(rows, &c); err != nil {
			httpx.Internal(w, r)
			return
		}
		if !fin {
			c.maskFinancials()
		}
		out = append(out, c)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"contracts": out})
}

type contractReq struct {
	SubcontractorID  *string  `json:"subcontractor_id"`
	ContractNo       string   `json:"contract_no"`
	Type             *string  `json:"type"`
	ParentContractID *string  `json:"parent_contract_id"`
	Amount           *float64 `json:"amount"`
	AdvanceAmount    *float64 `json:"advance_amount"`
	RetentionPct     *float64 `json:"retention_pct"`
	AdvanceRatePct   *float64 `json:"advance_rate_pct"`
	SignDate         *string  `json:"sign_date"`
	DocumentID       *string  `json:"document_id"`
	// Faz 11 — yıllara sari inşaat işi (GVK 42-44). true ise bu sözleşmenin
	// hakedişlerinde stopaj varsayılan olarak uygulanır (hakediş bazında
	// elle geçersiz kılınabilir).
	StartDate        *string  `json:"start_date"`
	EndDate          *string  `json:"end_date"`
	RevisedEndDate   *string  `json:"revised_end_date"`
	IsMultiYear      *bool    `json:"is_multi_year"`
	WithholdingPct   *float64 `json:"withholding_pct"`
	// DefaultPaymentMethod — Nakit Akış Faz A (nakit|havale|cek).
	DefaultPaymentMethod *string `json:"default_payment_method"`
	RowVersion           int     `json:"row_version"`
}

// normalizePaymentMethod — boş/whitespace ise nil (DB CHECK'i nullable
// bırakır); aksi halde trim edilmiş değeri döner (geçersiz değer varsa DB
// CHECK constraint'i reddeder).
func normalizePaymentMethod(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func optUUID(s *string) (*uuid.UUID, bool) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*s))
	if err != nil {
		return nil, false
	}
	return &id, true
}

func (h *Handler) CreateContract(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req contractReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.ContractNo = strings.TrimSpace(req.ContractNo)
	ctype := "Sub"
	if req.Type != nil && *req.Type != "" {
		ctype = *req.Type
	}
	retention := 3.0
	if req.RetentionPct != nil {
		retention = *req.RetentionPct
	}
	advRate := 20.0
	if req.AdvanceRatePct != nil {
		advRate = *req.AdvanceRatePct
	}
	if f := ValidateContract(req.ContractNo, ctype, retention, advRate); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	subID, ok1 := optUUID(req.SubcontractorID)
	parentID, ok2 := optUUID(req.ParentContractID)
	docID, ok3 := optUUID(req.DocumentID)
	if !ok1 || !ok2 || !ok3 {
		httpx.ValidationFailed(w, r, map[string]string{"id": "geçersiz UUID alanı"})
		return
	}
	advAmount := 0.0
	if req.AdvanceAmount != nil {
		advAmount = *req.AdvanceAmount
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var c contractDTO
	err = scanContract(tx.QueryRow(r.Context(), `
		INSERT INTO contracts (project_id, subcontractor_id, contract_no, type, parent_contract_id,
			amount, advance_amount, retention_pct, advance_rate_pct, sign_date, document_id,
			is_multi_year, withholding_pct, start_date, end_date, revised_end_date, default_payment_method)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, NULLIF($10,'')::date, $11,
			-- Yıllara sari varsayılanı: açıkça verilmemişse proje takviminden
			-- türetilir (başlangıç ve bitiş yılı farklıysa iş yıllara saridir).
			COALESCE($12, (
				SELECT p.start_date IS NOT NULL AND p.end_date IS NOT NULL
				       AND date_part('year', p.end_date) > date_part('year', p.start_date)
				FROM projects p WHERE p.id = $1
			), false),
			COALESCE($13, 5.00),
			NULLIF($14,'')::date, NULLIF($15,'')::date, NULLIF($16,'')::date, $17)
		RETURNING `+contractCols,
		pid, subID, req.ContractNo, ctype, parentID, req.Amount, advAmount, retention, advRate,
		strDeref(req.SignDate), docID, req.IsMultiYear, req.WithholdingPct,
		strDeref(req.StartDate), strDeref(req.EndDate), strDeref(req.RevisedEndDate),
		normalizePaymentMethod(req.DefaultPaymentMethod)), &c)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu sözleşme no zaten kullanımda.", nil)
			return
		}
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Taşeron/üst sözleşme/doküman bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "contracts", EntityID: c.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid, "contract_no": c.ContractNo, "type": c.Type}, IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	if !h.canFin(r, pid) {
		c.maskFinancials()
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"contract": c})
}

func (h *Handler) UpdateContract(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	cid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req contractReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before contractDTO
	err = scanContract(tx.QueryRow(r.Context(),
		`SELECT `+contractCols+` FROM contracts WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, cid, pid), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Sözleşme bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}
	ctype := before.Type
	if req.Type != nil && *req.Type != "" {
		ctype = *req.Type
	}
	contractNo := before.ContractNo
	if strings.TrimSpace(req.ContractNo) != "" {
		contractNo = strings.TrimSpace(req.ContractNo)
	}
	retention := before.RetentionPct
	if req.RetentionPct != nil {
		retention = *req.RetentionPct
	}
	advRate := before.AdvanceRatePct
	if req.AdvanceRatePct != nil {
		advRate = *req.AdvanceRatePct
	}
	if f := ValidateContract(contractNo, ctype, retention, advRate); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	amount := before.Amount
	if req.Amount != nil {
		amount = req.Amount
	}
	advAmount := 0.0
	if before.AdvanceAmount != nil {
		advAmount = *before.AdvanceAmount
	}
	if req.AdvanceAmount != nil {
		advAmount = *req.AdvanceAmount
	}
	paymentMethod := before.DefaultPaymentMethod
	if req.DefaultPaymentMethod != nil {
		paymentMethod = normalizePaymentMethod(req.DefaultPaymentMethod)
	}

	var c contractDTO
	err = scanContract(tx.QueryRow(r.Context(), `
		UPDATE contracts SET
			contract_no=$3, type=$4, amount=$5, advance_amount=$6, retention_pct=$7, advance_rate_pct=$8,
			sign_date=COALESCE(NULLIF($9,'')::date, sign_date),
			is_multi_year=COALESCE($10, is_multi_year),
			withholding_pct=COALESCE($11, withholding_pct),
			start_date=NULLIF($12,'')::date,
			end_date=NULLIF($13,'')::date,
			revised_end_date=NULLIF($14,'')::date,
			default_payment_method=$15,
			row_version=row_version+1
		WHERE id=$1 AND project_id=$2
		RETURNING `+contractCols,
		cid, pid, contractNo, ctype, amount, advAmount, retention, advRate, strDeref(req.SignDate),
		req.IsMultiYear, req.WithholdingPct,
		strDeref(req.StartDate), strDeref(req.EndDate), strDeref(req.RevisedEndDate), paymentMethod), &c)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu sözleşme no zaten kullanımda.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "contracts", EntityID: cid.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"contract_no": before.ContractNo},
		After:  map[string]interface{}{"contract_no": c.ContractNo}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	if !h.canFin(r, pid) {
		c.maskFinancials()
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"contract": c})
}

func (h *Handler) DeleteContract(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	cid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE contracts SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, cid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Sözleşme bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "contracts", EntityID: cid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Birim fiyat cetveli (work_items / BOQ)
// ---------------------------------------------------------------------------

type workItemDTO struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       uuid.UUID  `json:"project_id"`
	SubcontractorID uuid.UUID  `json:"subcontractor_id"`
	ContractID      *uuid.UUID `json:"contract_id,omitempty"`
	PozNo           string     `json:"poz_no"`
	Description     string     `json:"description"`
	Unit            string     `json:"unit"`
	ContractQty     float64    `json:"contract_qty"`
	UnitPrice       *float64   `json:"unit_price,omitempty"`
	ContractAmount  *float64   `json:"contract_amount,omitempty"`
	RowVersion      int        `json:"row_version"`
	CreatedAt       time.Time  `json:"created_at"`
}

const workItemCols = `id, project_id, subcontractor_id, contract_id, poz_no, description, unit,
	contract_qty::float8, unit_price::float8, row_version, created_at`

func scanWorkItem(row pgx.Row, wi *workItemDTO, fin bool) error {
	var unitPrice float64
	if err := row.Scan(&wi.ID, &wi.ProjectID, &wi.SubcontractorID, &wi.ContractID, &wi.PozNo,
		&wi.Description, &wi.Unit, &wi.ContractQty, &unitPrice, &wi.RowVersion, &wi.CreatedAt); err != nil {
		return err
	}
	if fin {
		up := unitPrice
		amt := round2(wi.ContractQty * unitPrice)
		wi.UnitPrice = &up
		wi.ContractAmount = &amt
	}
	return nil
}

// resolveSub — work_item uçları için taşeronu çözer, satır seviyesi güvenliği
// zorlar (rep yalnızca kendi taşeronu). Taşeron projeye ait olmalı.
func (h *Handler) resolveSub(w http.ResponseWriter, r *http.Request, pid uuid.UUID) (uuid.UUID, bool) {
	sid, ok := parseID(w, r, "subID")
	if !ok {
		return uuid.Nil, false
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return uuid.Nil, false
	}
	if scope != nil && *scope != sid {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu taşerona erişiminiz yok.", nil)
		return uuid.Nil, false
	}
	var exists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM subcontractors WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL)`,
		sid, pid).Scan(&exists); err != nil {
		httpx.Internal(w, r)
		return uuid.Nil, false
	}
	if !exists {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Taşeron bulunamadı.", nil)
		return uuid.Nil, false
	}
	return sid, true
}

func (h *Handler) ListWorkItems(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := h.resolveSub(w, r, pid)
	if !ok {
		return
	}
	fin := h.canFin(r, pid)
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+workItemCols+` FROM work_items
		 WHERE subcontractor_id=$1 AND deleted_at IS NULL ORDER BY poz_no`, sid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []workItemDTO{}
	for rows.Next() {
		var wi workItemDTO
		if err := scanWorkItem(rows, &wi, fin); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, wi)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"work_items": out})
}

type workItemReq struct {
	ContractID  *string  `json:"contract_id"`
	PozNo       string   `json:"poz_no"`
	Description string   `json:"description"`
	Unit        *string  `json:"unit"`
	ContractQty *float64 `json:"contract_qty"`
	UnitPrice   *float64 `json:"unit_price"`
	RowVersion  int      `json:"row_version"`
}

func (h *Handler) CreateWorkItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := h.resolveSub(w, r, pid)
	if !ok {
		return
	}
	var req workItemReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.PozNo = strings.TrimSpace(req.PozNo)
	req.Description = strings.TrimSpace(req.Description)
	unit := "ad"
	if req.Unit != nil && strings.TrimSpace(*req.Unit) != "" {
		unit = strings.TrimSpace(*req.Unit)
	}
	qty := 0.0
	if req.ContractQty != nil {
		qty = *req.ContractQty
	}
	price := 0.0
	if req.UnitPrice != nil {
		price = *req.UnitPrice
	}
	if f := ValidateWorkItem(req.PozNo, req.Description, qty, price); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	contractID, okc := optUUID(req.ContractID)
	if !okc {
		httpx.ValidationFailed(w, r, map[string]string{"contract_id": "geçersiz UUID"})
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var wi workItemDTO
	err = scanWorkItem(tx.QueryRow(r.Context(), `
		INSERT INTO work_items (project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+workItemCols,
		pid, sid, contractID, req.PozNo, req.Description, unit, qty, price), &wi, true)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu poz no bu taşeronda zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "work_items", EntityID: wi.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"subcontractor_id": sid, "poz_no": wi.PozNo}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	if !h.canFin(r, pid) {
		wi.UnitPrice = nil
		wi.ContractAmount = nil
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"work_item": wi})
}

func (h *Handler) UpdateWorkItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := h.resolveSub(w, r, pid)
	if !ok {
		return
	}
	wid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req workItemReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before workItemDTO
	err = scanWorkItem(tx.QueryRow(r.Context(),
		`SELECT `+workItemCols+` FROM work_items WHERE id=$1 AND subcontractor_id=$2 AND deleted_at IS NULL FOR UPDATE`,
		wid, sid), &before, true)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Poz bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}
	poz := before.PozNo
	if strings.TrimSpace(req.PozNo) != "" {
		poz = strings.TrimSpace(req.PozNo)
	}
	desc := before.Description
	if strings.TrimSpace(req.Description) != "" {
		desc = strings.TrimSpace(req.Description)
	}
	unit := before.Unit
	if req.Unit != nil && strings.TrimSpace(*req.Unit) != "" {
		unit = strings.TrimSpace(*req.Unit)
	}
	qty := before.ContractQty
	if req.ContractQty != nil {
		qty = *req.ContractQty
	}
	price := 0.0
	if before.UnitPrice != nil {
		price = *before.UnitPrice
	}
	if req.UnitPrice != nil {
		price = *req.UnitPrice
	}
	if f := ValidateWorkItem(poz, desc, qty, price); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	var wi workItemDTO
	err = scanWorkItem(tx.QueryRow(r.Context(), `
		UPDATE work_items SET poz_no=$3, description=$4, unit=$5, contract_qty=$6, unit_price=$7, row_version=row_version+1
		WHERE id=$1 AND subcontractor_id=$2
		RETURNING `+workItemCols, wid, sid, poz, desc, unit, qty, price), &wi, true)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu poz no bu taşeronda zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "work_items", EntityID: wid.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"poz_no": before.PozNo}, After: map[string]interface{}{"poz_no": wi.PozNo},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	if !h.canFin(r, pid) {
		wi.UnitPrice = nil
		wi.ContractAmount = nil
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"work_item": wi})
}

func (h *Handler) DeleteWorkItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := h.resolveSub(w, r, pid)
	if !ok {
		return
	}
	wid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE work_items SET deleted_at=now() WHERE id=$1 AND subcontractor_id=$2 AND deleted_at IS NULL`, wid, sid)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Poza bağlı hakediş kalemi var; silinemez.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Poz bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "work_items", EntityID: wid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ImportWorkItems — .xlsx/.csv toplu içe aktarma. poz_no bazında upsert:
// mevcut poz güncellenir, yeni poz eklenir. (Plan §8 Faz 3: Excel içe aktarma.)
func (h *Handler) ImportWorkItems(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := h.resolveSub(w, r, pid)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya çözümlenemedi (sınır 10 MB).", nil)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"file": "dosya alanı zorunlu (.xlsx veya .csv)"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImportBytes))
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	rowsIn, err := ParseImport(hdr.Filename, data)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya okunamadı: "+err.Error(), nil)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var inserted, updated, skipped int
	for _, row := range rowsIn {
		if strings.TrimSpace(row.PozNo) == "" || strings.TrimSpace(row.Description) == "" {
			skipped++
			continue
		}
		unit := row.Unit
		if strings.TrimSpace(unit) == "" {
			unit = "ad"
		}
		ct, err := tx.Exec(r.Context(), `
			INSERT INTO work_items (project_id, subcontractor_id, poz_no, description, unit, contract_qty, unit_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (subcontractor_id, poz_no) WHERE deleted_at IS NULL
			DO UPDATE SET description=EXCLUDED.description, unit=EXCLUDED.unit,
			              contract_qty=EXCLUDED.contract_qty, unit_price=EXCLUDED.unit_price,
			              row_version=work_items.row_version+1`,
			pid, sid, strings.TrimSpace(row.PozNo), strings.TrimSpace(row.Description), unit,
			row.ContractQty, row.UnitPrice)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Satır yazılamadı: "+err.Error(), nil)
			return
		}
		if ct.RowsAffected() > 0 {
			// pgx CommandTag INSERT/UPDATE ayrımını taşımaz; upsert'i "işlendi" sayarız.
			inserted++
		}
		_ = updated
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "work_items", EntityID: sid.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"import": "bulk", "rows": inserted, "skipped": skipped}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"processed": inserted, "skipped": skipped, "total": len(rowsIn)})
}

