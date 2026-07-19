package payments

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// Hakedişler (kümülatif iş akışı: Draft → Submitted → SiteApproved → Finalized)
// ---------------------------------------------------------------------------

type paymentDTO struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       uuid.UUID  `json:"project_id"`
	SubcontractorID uuid.UUID  `json:"subcontractor_id"`
	PeriodNo        int        `json:"period_no"`
	PeriodStart     *time.Time `json:"period_start,omitempty"`
	PeriodEnd       *time.Time `json:"period_end,omitempty"`
	Status          string     `json:"status"`
	GrossCum        *float64   `json:"gross_cum,omitempty"`
	GrossPrev       *float64   `json:"gross_prev,omitempty"`
	GrossThis       *float64   `json:"gross_this,omitempty"`
	TotalDeductions *float64   `json:"total_deductions,omitempty"`
	VatPct          float64    `json:"vat_pct"`
	NetPayable      *float64   `json:"net_payable,omitempty"`
	// Stopaj (Faz 11): nil = sözleşme varsayılanını izle (yıllara sari),
	// true/false = bu hakediş için elle geçersiz kılma.
	WithholdingApplied *bool   `json:"withholding_applied,omitempty"`
	// KDV tevkifatı (0=yok, 0.4=4/10) ve %0 KDV'de istisna gerekçesi.
	VatWithholdingRatio float64 `json:"vat_withholding_ratio"`
	VatExemptionCode    *string `json:"vat_exemption_code,omitempty"`
	VatAmount     float64 `json:"vat_amount"`
	VatWithheld   float64 `json:"vat_withheld"`
	VatCollected  float64 `json:"vat_collected"`
	PayableGross  float64 `json:"payable_gross"`
	ActualCost    float64 `json:"actual_cost"`
	CurrentStepNo int     `json:"current_step_no"`
	FinalizedAt     *time.Time `json:"finalized_at,omitempty"`
	RowVersion      int        `json:"row_version"`
	CreatedAt       time.Time  `json:"created_at"`
}

const paymentCols = `id, project_id, subcontractor_id, period_no, period_start, period_end, status,
	gross_cum::float8, gross_prev::float8, gross_this::float8, total_deductions::float8,
	vat_pct::float8, net_payable::float8, withholding_applied,
	vat_withholding_ratio::float8, vat_exemption_code,
	vat_amount::float8, vat_withheld::float8, vat_collected::float8,
	payable_gross::float8, actual_cost::float8, current_step_no,
	finalized_at, row_version, created_at`

func scanPayment(row pgx.Row, p *paymentDTO) error {
	var gc, gp, gt, td, np float64
	if err := row.Scan(&p.ID, &p.ProjectID, &p.SubcontractorID, &p.PeriodNo, &p.PeriodStart, &p.PeriodEnd,
		&p.Status, &gc, &gp, &gt, &td, &p.VatPct, &np, &p.WithholdingApplied,
		&p.VatWithholdingRatio, &p.VatExemptionCode,
		&p.VatAmount, &p.VatWithheld, &p.VatCollected, &p.PayableGross, &p.ActualCost, &p.CurrentStepNo,
		&p.FinalizedAt, &p.RowVersion, &p.CreatedAt); err != nil {
		return err
	}
	p.GrossCum, p.GrossPrev, p.GrossThis = &gc, &gp, &gt
	p.TotalDeductions, p.NetPayable = &td, &np
	return nil
}

func (p *paymentDTO) maskFinancials() {
	p.GrossCum, p.GrossPrev, p.GrossThis = nil, nil, nil
	p.TotalDeductions, p.NetPayable = nil, nil
}

func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
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
		SELECT `+paymentCols+` FROM progress_payments
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
	out := []paymentDTO{}
	for rows.Next() {
		var p paymentDTO
		if err := scanPayment(rows, &p); err != nil {
			httpx.Internal(w, r)
			return
		}
		if !fin {
			p.maskFinancials()
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"payments": out})
}

type createPaymentReq struct {
	SubcontractorID string  `json:"subcontractor_id"`
	PeriodNo        *int    `json:"period_no"`
	PeriodStart     *string `json:"period_start"`
	PeriodEnd       *string `json:"period_end"`
	VatPct          *float64 `json:"vat_pct"`
}

// CreatePayment — yeni Draft hakediş. period_no verilmezse taşeronun en yüksek
// döneminin bir fazlası atanır.
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	var req createPaymentReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	subID, err := uuid.Parse(req.SubcontractorID)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "geçersiz UUID"})
		return
	}
	if scope != nil && *scope != subID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu taşerona erişiminiz yok.", nil)
		return
	}
	vat := 20.0
	if req.VatPct != nil {
		vat = *req.VatPct
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// Taşeron projeye ait mi?
	var exists bool
	if err := tx.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM subcontractors WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL)`,
		subID, pid).Scan(&exists); err != nil {
		httpx.Internal(w, r)
		return
	}
	if !exists {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Taşeron bulunamadı.", nil)
		return
	}

	periodNo := 0
	if req.PeriodNo != nil {
		periodNo = *req.PeriodNo
	} else {
		var maxPeriod *int
		if err := tx.QueryRow(r.Context(),
			`SELECT MAX(period_no) FROM progress_payments WHERE subcontractor_id=$1 AND deleted_at IS NULL`,
			subID).Scan(&maxPeriod); err != nil {
			httpx.Internal(w, r)
			return
		}
		if maxPeriod != nil {
			periodNo = *maxPeriod + 1
		} else {
			periodNo = 1
		}
	}

	// Faz 11 — dönem kontrolleri: tarih sırası, aynı taşeronda aralık çakışması,
	// sözleşme süresi aşımı. Kullanıcıya anlaşılır mesaj döner; veritabanında
	// ayrıca chk_pp_period_order + excl_pp_period_overlap koruması vardır.
	if ps, pe := parseDatePtr(req.PeriodStart), parseDatePtr(req.PeriodEnd); ps != nil || pe != nil {
		if f, err := ValidatePeriod(r.Context(), tx, PeriodCheckInput{
			SubcontractorID: subID.String(), Start: ps, End: pe,
		}); err != nil {
			httpx.Internal(w, r)
			return
		} else if len(f) > 0 {
			httpx.ValidationFailed(w, r, f)
			return
		}
	}

	var p paymentDTO
	err = scanPayment(tx.QueryRow(r.Context(), `
		INSERT INTO progress_payments (project_id, subcontractor_id, period_no, period_start, period_end, vat_pct, created_by)
		VALUES ($1,$2,$3, NULLIF($4,'')::date, NULLIF($5,'')::date, $6, $7)
		RETURNING `+paymentCols,
		pid, subID, periodNo, strDeref(req.PeriodStart), strDeref(req.PeriodEnd), vat, uid), &p)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu taşeron için bu dönem no zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "progress_payments", EntityID: p.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"subcontractor_id": subID, "period_no": periodNo, "status": "Draft"}, IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"payment": p})
}

// paymentItemView / deductionView — GET detay yanıtı için.
type paymentItemView struct {
	WorkItemID    uuid.UUID `json:"work_item_id"`
	PozNo         string    `json:"poz_no"`
	Description   string    `json:"description"`
	Unit          string    `json:"unit"`
	UnitPrice     *float64  `json:"unit_price,omitempty"`
	PrevCumQty    float64   `json:"prev_cum_qty"`
	ThisPeriodQty float64   `json:"this_period_qty"`
	CumQty        float64   `json:"cum_qty"`
	CumAmount     *float64  `json:"cum_amount,omitempty"`
	ThisAmount    *float64  `json:"this_amount,omitempty"`
}

type deductionView struct {
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	RatePct      *float64 `json:"rate_pct,omitempty"`
	Amount       *float64 `json:"amount,omitempty"`
	SourceEntity *string  `json:"source_entity,omitempty"`
	SourceID     *string  `json:"source_id,omitempty"`
}

// loadPayment — proje + satır-seviyesi doğrulamalı hakediş getirir.
func (h *Handler) loadPayment(w http.ResponseWriter, r *http.Request, pid uuid.UUID, ppID uuid.UUID, scope *uuid.UUID) (*paymentDTO, bool) {
	var p paymentDTO
	err := scanPayment(h.pool.QueryRow(r.Context(),
		`SELECT `+paymentCols+` FROM progress_payments WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, ppID, pid), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
		return nil, false
	}
	if err != nil {
		httpx.Internal(w, r)
		return nil, false
	}
	if scope != nil && *scope != p.SubcontractorID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu hakedişe erişiminiz yok.", nil)
		return nil, false
	}
	return &p, true
}

func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ppID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	p, ok := h.loadPayment(w, r, pid, ppID, scope)
	if !ok {
		return
	}
	fin := h.canFin(r, pid)

	// Kalemler
	itemRows, err := h.pool.Query(r.Context(), `
		SELECT ppi.work_item_id, wi.poz_no, wi.description, wi.unit, wi.unit_price::float8,
		       ppi.prev_cum_qty::float8, ppi.this_period_qty::float8, ppi.cum_qty::float8,
		       ppi.cum_amount::float8, ppi.this_amount::float8
		FROM progress_payment_items ppi
		JOIN work_items wi ON wi.id = ppi.work_item_id
		WHERE ppi.progress_payment_id=$1
		ORDER BY wi.poz_no`, ppID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer itemRows.Close()
	items := []paymentItemView{}
	for itemRows.Next() {
		var iv paymentItemView
		var unitPrice, cumAmount, thisAmount float64
		if err := itemRows.Scan(&iv.WorkItemID, &iv.PozNo, &iv.Description, &iv.Unit, &unitPrice,
			&iv.PrevCumQty, &iv.ThisPeriodQty, &iv.CumQty, &cumAmount, &thisAmount); err != nil {
			httpx.Internal(w, r)
			return
		}
		if fin {
			iv.UnitPrice, iv.CumAmount, iv.ThisAmount = &unitPrice, &cumAmount, &thisAmount
		}
		items = append(items, iv)
	}

	// Kesintiler
	dedRows, err := h.pool.Query(r.Context(),
		`SELECT type, COALESCE(description,''), rate_pct::float8, amount::float8,
		        source_entity, source_id::text
		 FROM payment_deductions WHERE progress_payment_id=$1 ORDER BY created_at`, ppID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer dedRows.Close()
	deductions := []deductionView{}
	for dedRows.Next() {
		var dv deductionView
		var rate *float64
		var amount float64
		if err := dedRows.Scan(&dv.Type, &dv.Description, &rate, &amount, &dv.SourceEntity, &dv.SourceID); err != nil {
			httpx.Internal(w, r)
			return
		}
		dv.RatePct = rate
		if fin {
			dv.Amount = &amount
		}
		deductions = append(deductions, dv)
	}

	// Faz 8 köprüsü: taslak hakedişte, bu taşerona kesilmiş ve HENÜZ hiçbir
	// hakedişe uygulanmamış İSG para cezaları kesinti ÖNERİSİ olarak listelenir.
	// Öneri otomatik uygulanmaz — PY taslağa ekleyip kesinleştirerek onaylar
	// (Plan Faz 8: "PY onayıyla uygulanır, source_id ile izlenebilir").
	var suggestions []ohsPenaltySuggestion
	if p.Status == "Draft" && fin {
		suggestions, err = h.ohsPenaltySuggestions(r.Context(), ppID, p.SubcontractorID)
		if err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	if !fin {
		p.maskFinancials()
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"payment": p, "items": items, "deductions": deductions,
		"ohs_penalty_suggestions": suggestions,
	})
}

// ohsPenaltySuggestion — Faz 8: bekleyen İSG para cezası → kesinti önerisi.
type ohsPenaltySuggestion struct {
	PenaltyID     string  `json:"penalty_id"`
	PenaltyNo     string  `json:"penalty_no"`
	ViolationType string  `json:"violation_type"`
	IssuedAt      string  `json:"issued_at"`
	Amount        float64 `json:"amount"`
	AlreadyAdded  bool    `json:"already_added"` // bu taslağın kesintilerinde zaten var
}

// ohsPenaltySuggestions — taşeronun hakedişe uygulanmamış (Issued/Acknowledged)
// para cezalarını döner; taslakta hâlihazırda kesinti olarak eklenmiş olanlar
// işaretlenir (UI mükerrer eklemeyi engeller).
func (h *Handler) ohsPenaltySuggestions(ctx context.Context, ppID, subID uuid.UUID) ([]ohsPenaltySuggestion, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT op.id::text, op.penalty_no, op.violation_type,
		       to_char(op.issued_at,'YYYY-MM-DD'), op.amount::float8,
		       EXISTS(SELECT 1 FROM payment_deductions pd
		              WHERE pd.progress_payment_id=$1
		                AND pd.source_entity='ohs_penalties' AND pd.source_id=op.id)
		FROM ohs_penalties op
		WHERE op.subcontractor_id=$2 AND op.deleted_at IS NULL
		  AND op.penalty_level='Fine' AND op.amount IS NOT NULL
		  AND op.status IN ('Issued','Acknowledged')
		ORDER BY op.issued_at`, ppID, subID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ohsPenaltySuggestion{}
	for rows.Next() {
		var sg ohsPenaltySuggestion
		if err := rows.Scan(&sg.PenaltyID, &sg.PenaltyNo, &sg.ViolationType,
			&sg.IssuedAt, &sg.Amount, &sg.AlreadyAdded); err != nil {
			return nil, err
		}
		list = append(list, sg)
	}
	return list, nil
}

// --- hesap köprüsü: girilen metrajlardan tam sonuç üret -------------------

type itemEntry struct {
	WorkItemID uuid.UUID
	CumQty     float64
}

// computeForPayment — verilen kalem kümülatiflerinden (entries) ve ekstra
// kesintilerden tam CalcResult üretir; birim fiyatları work_items'tan, önceki
// kümülatifi/brütü son Finalized hakedişten, sözleşme şartlarını taşeronun
// yürürlükteki sözleşmesinden okur.
func (h *Handler) computeForPayment(ctx context.Context, q pgxQuerier, pp *paymentDTO, entries []itemEntry, extras []ExtraDeduction) (CalcResult, error) {
	// Önceki Finalized dönem: grossPrev + poz bazında prev_cum_qty
	var grossPrev float64
	var prevPPID *uuid.UUID
	err := q.QueryRow(ctx, `
		SELECT id, gross_cum::float8 FROM progress_payments
		WHERE subcontractor_id=$1 AND status='Finalized' AND deleted_at IS NULL AND period_no < $2
		ORDER BY period_no DESC LIMIT 1`, pp.SubcontractorID, pp.PeriodNo).Scan(&prevPPID, &grossPrev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CalcResult{}, err
	}
	prevCum := map[uuid.UUID]float64{}
	if prevPPID != nil {
		rows, err := q.Query(ctx,
			`SELECT work_item_id, cum_qty::float8 FROM progress_payment_items WHERE progress_payment_id=$1`, *prevPPID)
		if err != nil {
			return CalcResult{}, err
		}
		for rows.Next() {
			var wid uuid.UUID
			var cq float64
			if err := rows.Scan(&wid, &cq); err != nil {
				rows.Close()
				return CalcResult{}, err
			}
			prevCum[wid] = cq
		}
		rows.Close()
	}

	// Sözleşme şartları (yürürlükteki Main/Sub, en yeni)
	var terms ContractTerms
	terms.VatPct = pp.VatPct
	var advAmount, retPct, advRate, whPct float64
	var multiYear bool
	err = q.QueryRow(ctx, `
		SELECT COALESCE(advance_amount,0)::float8, retention_pct::float8, advance_rate_pct::float8,
		       COALESCE(withholding_pct,5)::float8, COALESCE(is_multi_year,false)
		FROM contracts
		WHERE subcontractor_id=$1 AND deleted_at IS NULL AND type IN ('Main','Sub')
		ORDER BY sign_date DESC NULLS LAST, created_at DESC LIMIT 1`, pp.SubcontractorID).
		Scan(&advAmount, &retPct, &advRate, &whPct, &multiYear)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CalcResult{}, err
	}
	terms.AdvanceAmount = advAmount
	terms.RetentionPct = retPct
	terms.AdvanceRatePct = advRate

	// Stopaj (Faz 11): hakedişte elle işaretlenmişse o değer, değilse
	// sözleşmenin yıllara sari (is_multi_year) varsayılanı geçerlidir.
	terms.WithholdingPct = whPct
	terms.ApplyWithholding = ResolveWithholding(pp.WithholdingApplied, multiYear)
	terms.VatWithholdingRatio = pp.VatWithholdingRatio

	// Önceki dönemlere dek mahsup edilmiş avans
	var advRecovered float64
	if err := q.QueryRow(ctx, `
		SELECT COALESCE(SUM(pd.amount),0)::float8
		FROM payment_deductions pd
		JOIN progress_payments pp2 ON pp2.id = pd.progress_payment_id
		WHERE pp2.subcontractor_id=$1 AND pp2.status='Finalized' AND pp2.deleted_at IS NULL
		  AND pd.type='AdvanceOffset' AND pp2.period_no < $2`, pp.SubcontractorID, pp.PeriodNo).
		Scan(&advRecovered); err != nil {
		return CalcResult{}, err
	}
	terms.AdvanceRecoveredPrev = advRecovered

	// Kalem girdileri — birim fiyat/poz/açıklama work_items'tan
	inputs := make([]CalcLineInput, 0, len(entries))
	for _, e := range entries {
		var poz, desc, unit string
		var price, cqty float64
		err := q.QueryRow(ctx,
			`SELECT poz_no, description, unit, unit_price::float8, contract_qty::float8
			 FROM work_items WHERE id=$1 AND subcontractor_id=$2 AND deleted_at IS NULL`,
			e.WorkItemID, pp.SubcontractorID).Scan(&poz, &desc, &unit, &price, &cqty)
		if errors.Is(err, pgx.ErrNoRows) {
			return CalcResult{}, errWorkItemNotFound
		}
		if err != nil {
			return CalcResult{}, err
		}
		inputs = append(inputs, CalcLineInput{
			WorkItemID: e.WorkItemID.String(), PozNo: poz, Description: desc, Unit: unit,
			UnitPrice: price, ContractQty: cqty, PrevCumQty: prevCum[e.WorkItemID], CumQty: e.CumQty,
		})
	}
	return Compute(inputs, grossPrev, terms, extras), nil
}

var errWorkItemNotFound = errors.New("poz bulunamadı veya bu taşerona ait değil")

// persistCalc — items + deductions'ı yeniden yazar, payment toplamlarını günceller.
// Çağıran açık bir tx sağlar. status DEĞİŞTİRİLMEZ (Draft düzenleme için).
func (h *Handler) persistCalc(ctx context.Context, tx pgx.Tx, ppID uuid.UUID, res CalcResult) error {
	if _, err := tx.Exec(ctx, `DELETE FROM payment_deductions WHERE progress_payment_id=$1`, ppID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM progress_payment_items WHERE progress_payment_id=$1`, ppID); err != nil {
		return err
	}
	for _, l := range res.Lines {
		wid, err := uuid.Parse(l.WorkItemID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO progress_payment_items
			  (progress_payment_id, work_item_id, prev_cum_qty, this_period_qty, cum_qty, cum_amount, this_amount)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			ppID, wid, l.PrevCumQty, l.ThisPeriodQty, l.CumQty, l.CumAmount, l.ThisAmount); err != nil {
			return err
		}
	}
	for _, d := range res.Deductions {
		var srcID *uuid.UUID
		if d.SourceID != nil {
			if id, err := uuid.Parse(*d.SourceID); err == nil {
				srcID = &id
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO payment_deductions
			  (progress_payment_id, type, source_entity, source_id, description, rate_pct, amount,
			   nature, reduces_cost, group_code, catalog_code, vat_pct, net_amount)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			ppID, d.Type, d.SourceEntity, srcID, d.Description, d.RatePct, d.Amount,
			d.Nature, d.ReducesCost, nullIfEmpty(d.GroupCode), nullIfEmpty(d.CatalogCode),
			d.VatPct, d.NetAmount); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `
		UPDATE progress_payments SET
			gross_cum=$2, gross_prev=$3, gross_this=$4, total_deductions=$5, net_payable=$6, vat_pct=$7,
			vat_amount=$8, vat_withheld=$9, vat_collected=$10, payable_gross=$11, actual_cost=$12,
			row_version=row_version+1
		WHERE id=$1`,
		ppID, res.GrossCum, res.GrossPrev, res.GrossThis, res.TotalDeductions, res.NetPayable, res.VatPct,
		res.VatAmount, res.VatWithheld, res.VatCollected, res.PayableGross, res.ActualCost)
	return err
}

type updateDraftReq struct {
	PeriodStart *string `json:"period_start"`
	PeriodEnd   *string `json:"period_end"`
	VatPct      *float64 `json:"vat_pct"`
	// Stopaj tiki (Faz 11). Gönderilmezse mevcut değer korunur; gönderilirse
	// bu hakediş için sözleşme varsayılanını geçersiz kılar.
	WithholdingApplied *bool `json:"withholding_applied"`
	// KDV tevkifat oranı (0=yok, 0.4=4/10) ve %0 KDV istisna gerekçesi.
	VatWithholdingRatio *float64 `json:"vat_withholding_ratio"`
	VatExemptionCode    *string  `json:"vat_exemption_code"`
	RowVersion  int      `json:"row_version"`
	Items       []struct {
		WorkItemID string  `json:"work_item_id"`
		CumQty     float64 `json:"cum_qty"`
	} `json:"items"`
	Deductions []struct {
		Type         string   `json:"type"`
		Description  string   `json:"description"`
		Amount       float64  `json:"amount"`
		RatePct      *float64 `json:"rate_pct"`
		SourceEntity *string  `json:"source_entity"` // Faz 8: 'ohs_penalties'
		SourceID     *string  `json:"source_id"`
		// Faz 11 — katalog seçimi (grup + kalem kodu). Boş bırakılırsa tipten türetilir.
		GroupCode    string   `json:"group_code"`
		CatalogCode  string   `json:"catalog_code"`
		VatPct       float64  `json:"vat_pct"` // kesintinin kendi KDV oranı (Amount KDV dahil)
	} `json:"deductions"`
}

// UpdatePaymentDraft — metraj girişi + ekstra kesintiler; yeniden hesaplayıp
// kalemleri/kesintileri/toplamları yazar. Yalnızca Draft durumunda.
func (h *Handler) UpdatePaymentDraft(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ppID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	var req updateDraftReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var pp paymentDTO
	err = scanPayment(tx.QueryRow(r.Context(),
		`SELECT `+paymentCols+` FROM progress_payments WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`,
		ppID, pid), &pp)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if scope != nil && *scope != pp.SubcontractorID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu hakedişe erişiminiz yok.", nil)
		return
	}
	if pp.Status != "Draft" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca taslak hakediş düzenlenebilir.", map[string]string{"status": pp.Status})
		return
	}
	if req.RowVersion != 0 && req.RowVersion != pp.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": pp.RowVersion})
		return
	}
	if req.WithholdingApplied != nil {
		pp.WithholdingApplied = req.WithholdingApplied
	}
	if req.VatWithholdingRatio != nil {
		pp.VatWithholdingRatio = *req.VatWithholdingRatio
	}
	if req.VatPct != nil {
		pp.VatPct = *req.VatPct
	}

	entries := make([]itemEntry, 0, len(req.Items))
	for _, it := range req.Items {
		wid, err := uuid.Parse(it.WorkItemID)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"items": "geçersiz work_item_id"})
			return
		}
		entries = append(entries, itemEntry{WorkItemID: wid, CumQty: it.CumQty})
	}
	extras := make([]ExtraDeduction, 0, len(req.Deductions))
	for _, d := range req.Deductions {
		if d.Type == "AdvanceOffset" || d.Type == "Retention" {
			continue // otomatik kalemler manuel girilemez
		}
		if !DeductionTypes[d.Type] {
			httpx.ValidationFailed(w, r, map[string]string{"deductions": "geçersiz kesinti türü: " + d.Type})
			return
		}
		extras = append(extras, ExtraDeduction{Type: d.Type, Description: d.Description, Amount: d.Amount,
			RatePct: d.RatePct, SourceEntity: d.SourceEntity, SourceID: d.SourceID,
			GroupCode: d.GroupCode, CatalogCode: d.CatalogCode, VatPct: d.VatPct})
	}

	res, err := h.computeForPayment(r.Context(), tx, &pp, entries, extras)
	if errors.Is(err, errWorkItemNotFound) {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, err.Error(), nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	if req.PeriodStart != nil || req.PeriodEnd != nil {
		// Yeni tarihler mevcut değerlerle birleştirilip doğrulanır.
		ns, ne := pp.PeriodStart, pp.PeriodEnd
		if d := parseDatePtr(req.PeriodStart); d != nil {
			ns = d
		}
		if d := parseDatePtr(req.PeriodEnd); d != nil {
			ne = d
		}
		if f, err := ValidatePeriod(r.Context(), tx, PeriodCheckInput{
			SubcontractorID: pp.SubcontractorID.String(), PaymentID: ppID.String(),
			Start: ns, End: ne,
		}); err != nil {
			httpx.Internal(w, r)
			return
		} else if len(f) > 0 {
			httpx.ValidationFailed(w, r, f)
			return
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE progress_payments SET
				period_start=COALESCE(NULLIF($2,'')::date, period_start),
				period_end=COALESCE(NULLIF($3,'')::date, period_end)
			WHERE id=$1`, ppID, strDeref(req.PeriodStart), strDeref(req.PeriodEnd)); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if req.WithholdingApplied != nil {
		if _, err := tx.Exec(r.Context(),
			`UPDATE progress_payments SET withholding_applied=$2 WHERE id=$1`,
			ppID, *req.WithholdingApplied); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if req.VatWithholdingRatio != nil || req.VatExemptionCode != nil {
		if _, err := tx.Exec(r.Context(), `
			UPDATE progress_payments SET
				vat_withholding_ratio = COALESCE($2, vat_withholding_ratio),
				vat_exemption_code    = COALESCE(NULLIF($3,''), vat_exemption_code)
			WHERE id=$1`, ppID, req.VatWithholdingRatio, strDeref(req.VatExemptionCode)); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if err := h.persistCalc(r.Context(), tx, ppID, res); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "progress_payments", EntityID: ppID.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"gross_this": res.GrossThis, "net_payable": res.NetPayable, "items": len(res.Lines)},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"status": "recalculated", "gross_this": res.GrossThis, "total_deductions": res.TotalDeductions,
		"net_payable": res.NetPayable, "vat_amount": res.VatAmount, "actual_cost": res.ActualCost,
	})
}

// --- iş akışı geçişleri ---

type transitionReq struct {
	Note       string `json:"note"`
	RowVersion int    `json:"row_version"`
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request, to string) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ppID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	var req transitionReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var pp paymentDTO
	err = scanPayment(tx.QueryRow(r.Context(),
		`SELECT `+paymentCols+` FROM progress_payments WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`,
		ppID, pid), &pp)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if scope != nil && *scope != pp.SubcontractorID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu hakedişe erişiminiz yok.", nil)
		return
	}
	if !CanTransition(pp.Status, to) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Geçersiz durum geçişi.", map[string]string{"from": pp.Status, "to": to})
		return
	}
	if req.RowVersion != 0 && req.RowVersion != pp.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": pp.RowVersion})
		return
	}

	// Finalize: kilitlemeden önce SON KEZ yeniden hesapla (tutarlılık).
	if to == "Finalized" {
		if err := h.finalizeRecompute(r.Context(), tx, &pp); err != nil {
			if errors.Is(err, errNoItems) {
				httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Kalemi olmayan hakediş kesinleştirilemez.", nil)
				return
			}
			httpx.Internal(w, r)
			return
		}
	}

	// Durum + zaman damgaları tek UPDATE'te (Finalized kilidi bu geçişe izin verir).
	var q string
	switch to {
	case "Submitted":
		q = `UPDATE progress_payments SET status='Submitted', submitted_at=now(), row_version=row_version+1 WHERE id=$1`
	case "SiteApproved":
		q = `UPDATE progress_payments SET status='SiteApproved', site_approved_at=now(), row_version=row_version+1 WHERE id=$1`
	case "Finalized":
		q = `UPDATE progress_payments SET status='Finalized', finalized_at=now(), finalized_by=$2, row_version=row_version+1 WHERE id=$1`
	case "Rejected":
		q = `UPDATE progress_payments SET status='Rejected', row_version=row_version+1 WHERE id=$1`
	case "Draft":
		q = `UPDATE progress_payments SET status='Draft', submitted_at=NULL, site_approved_at=NULL, row_version=row_version+1 WHERE id=$1`
	default:
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Bilinmeyen durum.", nil)
		return
	}
	var execErr error
	if to == "Finalized" {
		_, execErr = tx.Exec(r.Context(), q, ppID, uid)
	} else {
		_, execErr = tx.Exec(r.Context(), q, ppID)
	}
	if execErr != nil {
		if isLockViolation(execErr) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Kesinleşmiş hakediş değiştirilemez.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	// Faz 8 köprüsü: kesinleşen hakedişteki İSG kaynaklı kesintiler,
	// bağlı ceza tutanaklarını AppliedToPayment yapar (source_id izlenebilirliği).
	if to == "Finalized" {
		if _, err := tx.Exec(r.Context(), `
			UPDATE ohs_penalties SET status='AppliedToPayment', applied_payment_id=$1,
			       row_version=row_version+1
			WHERE id IN (SELECT source_id FROM payment_deductions
			             WHERE progress_payment_id=$1 AND source_entity='ohs_penalties'
			               AND source_id IS NOT NULL)
			  AND status IN ('Issued','Acknowledged') AND deleted_at IS NULL`, ppID); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	// İş akışı geçiş kaydı (workflow_transitions Faz 2'de mevcut)
	_, _ = tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('progress_payment', $1, $2, $3, $4, NULLIF($5,''))`,
		ppID, pp.Status, to, uid, req.Note)

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "progress_payments", EntityID: ppID.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": pp.Status}, After: map[string]interface{}{"status": to},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": to})
}

var errNoItems = errors.New("kalem yok")

// finalizeRecompute — kesinleştirme öncesi kayıtlı metraj + güncel birim fiyat +
// kayıtlı manuel kesintilerle yeniden hesaplar ve yazar (auto D/E tazelenir).
func (h *Handler) finalizeRecompute(ctx context.Context, tx pgx.Tx, pp *paymentDTO) error {
	rows, err := tx.Query(ctx,
		`SELECT work_item_id, cum_qty::float8 FROM progress_payment_items WHERE progress_payment_id=$1`, pp.ID)
	if err != nil {
		return err
	}
	var entries []itemEntry
	for rows.Next() {
		var wid uuid.UUID
		var cq float64
		if err := rows.Scan(&wid, &cq); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, itemEntry{WorkItemID: wid, CumQty: cq})
	}
	rows.Close()
	if len(entries) == 0 {
		return errNoItems
	}

	dr, err := tx.Query(ctx, `
		SELECT type, COALESCE(description,''), rate_pct::float8, amount::float8,
		       source_entity, source_id::text
		FROM payment_deductions
		WHERE progress_payment_id=$1 AND type NOT IN ('AdvanceOffset','Retention')`, pp.ID)
	if err != nil {
		return err
	}
	var extras []ExtraDeduction
	for dr.Next() {
		var t, d string
		var rate *float64
		var amt float64
		var srcEnt, srcID *string
		if err := dr.Scan(&t, &d, &rate, &amt, &srcEnt, &srcID); err != nil {
			dr.Close()
			return err
		}
		extras = append(extras, ExtraDeduction{Type: t, Description: d, RatePct: rate, Amount: amt,
			SourceEntity: srcEnt, SourceID: srcID})
	}
	dr.Close()

	res, err := h.computeForPayment(ctx, tx, pp, entries, extras)
	if err != nil {
		return err
	}
	return h.persistCalc(ctx, tx, pp.ID, res)
}

func (h *Handler) SubmitPayment(w http.ResponseWriter, r *http.Request)   { h.transition(w, r, "Submitted") }
func (h *Handler) ApprovePayment(w http.ResponseWriter, r *http.Request)  { h.transition(w, r, "SiteApproved") }
func (h *Handler) FinalizePayment(w http.ResponseWriter, r *http.Request) { h.transition(w, r, "Finalized") }
func (h *Handler) RejectPayment(w http.ResponseWriter, r *http.Request)   { h.transition(w, r, "Rejected") }

// SummaryPDF — hakediş özet PDF'i (application/pdf). view_financials gerektirir.
func (h *Handler) SummaryPDF(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	ppID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	if !h.canFin(r, pid) {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Finansal görüntüleme yetkisi gerekli.", nil)
		return
	}
	pp, ok := h.loadPayment(w, r, pid, ppID, scope)
	if !ok {
		return
	}

	var data PDFData
	data.PeriodNo = pp.PeriodNo
	data.Status = pp.Status
	if pp.GrossCum != nil {
		data.GrossCum, data.GrossPrev, data.GrossThis = *pp.GrossCum, *pp.GrossPrev, *pp.GrossThis
		data.TotalDeductions, data.NetPayable = *pp.TotalDeductions, *pp.NetPayable
	}
	// Faz 11 — KDV dönem brütü üzerinden hesaplanır; tevkifat kadarı ödemeye
	// eklenmez. Kesintiler KDV dahil ödenebilir toplamdan düşülmüştür.
	data.VatPct = pp.VatPct
	data.VatAmount = pp.VatAmount
	data.VatWithheld = pp.VatWithheld
	data.VatCollected = pp.VatCollected
	data.PayableGross = pp.PayableGross
	if pp.PeriodStart != nil {
		data.PeriodStart = pp.PeriodStart.Format("2006-01-02")
	}
	if pp.PeriodEnd != nil {
		data.PeriodEnd = pp.PeriodEnd.Format("2006-01-02")
	}

	_ = h.pool.QueryRow(r.Context(), `
		SELECT p.name, p.code, COALESCE(p.currency,'TRY'), s.company_name
		FROM progress_payments pp
		JOIN projects p ON p.id = pp.project_id
		JOIN subcontractors s ON s.id = pp.subcontractor_id
		WHERE pp.id=$1`, ppID).Scan(&data.ProjectName, &data.ProjectCode, &data.Currency, &data.Subcontractor)

	itemRows, err := h.pool.Query(r.Context(), `
		SELECT wi.poz_no, wi.description, wi.unit, wi.unit_price::float8, wi.contract_qty::float8,
		       ppi.prev_cum_qty::float8, ppi.this_period_qty::float8, ppi.cum_qty::float8,
		       ppi.cum_amount::float8, ppi.this_amount::float8
		FROM progress_payment_items ppi JOIN work_items wi ON wi.id = ppi.work_item_id
		WHERE ppi.progress_payment_id=$1 ORDER BY wi.poz_no`, ppID)
	if err == nil {
		for itemRows.Next() {
			var l CalcLine
			_ = itemRows.Scan(&l.PozNo, &l.Description, &l.Unit, &l.UnitPrice, &l.ContractQty,
				&l.PrevCumQty, &l.ThisPeriodQty, &l.CumQty, &l.CumAmount, &l.ThisAmount)
			data.Lines = append(data.Lines, l)
		}
		itemRows.Close()
	}
	dedRows, err := h.pool.Query(r.Context(),
		`SELECT type, COALESCE(description,''), rate_pct::float8, amount::float8
		 FROM payment_deductions WHERE progress_payment_id=$1 ORDER BY created_at`, ppID)
	if err == nil {
		for dedRows.Next() {
			var d DeductionLine
			var rate *float64
			_ = dedRows.Scan(&d.Type, &d.Description, &rate, &d.Amount)
			d.RatePct = rate
			data.Deductions = append(data.Deductions, d)
		}
		dedRows.Close()
	}

	pdf := BuildSummaryPDF(data)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"hakedis-"+ppID.String()+".pdf\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

// pgxQuerier — tx veya pool ile ortak sorgu arayüzü (computeForPayment için).
type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

