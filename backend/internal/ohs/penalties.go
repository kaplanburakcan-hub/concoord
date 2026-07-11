package ohs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
)

// ---------------------------------------------------------------------------
// Ceza tutanağı otomasyonu (Plan Faz 8 kabul kriteri: 60 sn içinde PDF+bildirim)
//
// PDF, tutanak kesme isteğinin İÇİNDE senkron üretilir (stdlib motor, Faz 3
// hakediş özeti deseni — deterministik, milisaniyeler sürer) ve MinIO'ya
// yazılır. Worker kuyruğuna gerek yoktur; kabul kriterindeki 60 sn sınırı
// yapısal olarak garanti edilir. Bildirimler notify servisiyle asenkron gider.
// ---------------------------------------------------------------------------

type penaltyDTO struct {
	ID                 uuid.UUID  `json:"id"`
	PenaltyNo          string     `json:"penalty_no"`
	SubcontractorID    uuid.UUID  `json:"subcontractor_id"`
	SubcontractorName  string     `json:"subcontractor_name"`
	FindingID          *uuid.UUID `json:"finding_id,omitempty"`
	ViolationType      string     `json:"violation_type"`
	PenaltyLevel       string     `json:"penalty_level"`
	Amount             *float64   `json:"amount,omitempty"` // view_financials olmadan maskelenir
	Note               *string    `json:"note,omitempty"`
	EvidenceDocumentID *uuid.UUID `json:"evidence_document_id,omitempty"`
	IssuedBy           uuid.UUID  `json:"issued_by"`
	IssuedByName       string     `json:"issued_by_name"`
	IssuedAt           time.Time  `json:"issued_at"`
	Status             string     `json:"status"`
	AppliedPaymentID   *uuid.UUID `json:"applied_payment_id,omitempty"`
	HasPDF             bool       `json:"has_pdf"`
	RowVersion         int        `json:"row_version"`
	CreatedAt          time.Time  `json:"created_at"`
}

const penaltySelect = `
	SELECT p.id, p.penalty_no, p.subcontractor_id, s.company_name, p.finding_id,
	       p.violation_type, p.penalty_level, p.amount::float8, p.note,
	       p.evidence_document_id, p.issued_by, u.full_name, p.issued_at,
	       p.status, p.applied_payment_id, p.pdf_file_id IS NOT NULL,
	       p.row_version, p.created_at
	FROM ohs_penalties p
	JOIN subcontractors s ON s.id = p.subcontractor_id
	JOIN users u ON u.id = p.issued_by
`

func scanPenalty(row pgx.Row, d *penaltyDTO) error {
	return row.Scan(&d.ID, &d.PenaltyNo, &d.SubcontractorID, &d.SubcontractorName,
		&d.FindingID, &d.ViolationType, &d.PenaltyLevel, &d.Amount, &d.Note,
		&d.EvidenceDocumentID, &d.IssuedBy, &d.IssuedByName, &d.IssuedAt,
		&d.Status, &d.AppliedPaymentID, &d.HasPDF, &d.RowVersion, &d.CreatedAt)
}

// canFin — tutar görünürlüğü Plan §4 view_financials ayrımıyla tutarlı:
// ceza tutarları da finansal veridir (hakediş kesintisine dönüşür).
func (h *Handler) canFin(r *http.Request, pid uuid.UUID) bool {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		return false
	}
	allowed, err := h.eval.Can(r.Context(), uid, &pid, "progress_payments.view_financials")
	return err == nil && allowed
}

// ListPenalties — taşeron temsilcisi yalnız kendi firmasını görür; tutarlar
// kendisininkiler olduğundan maskelenmez (sözleşme tarafıdır).
func (h *Handler) ListPenalties(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), penaltySelect+`
		WHERE p.project_id=$1 AND p.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR p.subcontractor_id=$2)
		ORDER BY p.issued_at DESC
		LIMIT 500`, pid, scope)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	fin := h.canFin(r, pid) || scope != nil
	list := []penaltyDTO{}
	for rows.Next() {
		var d penaltyDTO
		if err := scanPenalty(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		if !fin {
			d.Amount = nil
		}
		list = append(list, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"penalties": list})
}

func (h *Handler) GetPenalty(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	var d penaltyDTO
	err := scanPenalty(h.pool.QueryRow(r.Context(), penaltySelect+`
		WHERE p.id=$1 AND p.project_id=$2 AND p.deleted_at IS NULL`, id, pid), &d)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ceza tutanağı bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if scope != nil && *scope != d.SubcontractorID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu tutanağa erişiminiz yok.", nil)
		return
	}
	if !(h.canFin(r, pid) || scope != nil) {
		d.Amount = nil
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"penalty": d})
}

type penaltyReq struct {
	SubcontractorID string   `json:"subcontractor_id"`
	FindingID       *string  `json:"finding_id"`
	ViolationType   string   `json:"violation_type"`
	PenaltyLevel    string   `json:"penalty_level"` // Warning | Fine
	Amount          *float64 `json:"amount"`        // Fine için zorunlu
	Note            *string  `json:"note"`
	// Kanıt fotoğrafı — bulgudaki gibi tek istekte data-URL.
	EvidenceBase64 *string `json:"evidence_base64"`
	EvidenceName   *string `json:"evidence_name"`
}

// CreatePenalty — ihlal türü + taşeron + kanıt + tutar/seviye → tek istekte:
// tutanak kaydı + PDF (MinIO) + workflow/audit + bildirimler (taşeron
// temsilcisi + PY). Para cezası, taşeronun BİR SONRAKİ taslak hakedişinde
// kesinti önerisi olarak otomatik görünür (payments GET, PY onayıyla uygulanır).
func (h *Handler) CreatePenalty(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	if scope != nil {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Taşeron hesabıyla ceza kesilemez.", nil)
		return
	}
	var req penaltyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if errs := validatePenalty(req.PenaltyLevel, req.Amount, req.ViolationType); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	subID, err := uuid.Parse(req.SubcontractorID)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "geçersiz UUID"})
		return
	}
	var findingID *uuid.UUID
	if req.FindingID != nil && *req.FindingID != "" {
		v, err := uuid.Parse(*req.FindingID)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"finding_id": "geçersiz UUID"})
			return
		}
		findingID = &v
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// Taşeron + proje künyesi (PDF için) tek sorguda; yoksa 404.
	var subName, projName, projCode string
	err = tx.QueryRow(r.Context(), `
		SELECT s.company_name, pr.name, pr.code
		FROM subcontractors s JOIN projects pr ON pr.id = s.project_id
		WHERE s.id=$1 AND s.project_id=$2 AND s.deleted_at IS NULL`, subID, pid).
		Scan(&subName, &projName, &projCode)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Taşeron bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	// Tutanak numarası — proje içinde sıralı, advisory lock ile yarışsız
	// (PR/PO numaralandırma deseni).
	if _, err := tx.Exec(r.Context(),
		`SELECT pg_advisory_xact_lock(hashtext('ohs_penalty_no:' || $1::text))`, pid); err != nil {
		httpx.Internal(w, r)
		return
	}
	var seq int
	if err := tx.QueryRow(r.Context(), `
		SELECT COALESCE(MAX(substring(penalty_no from 'ISG-(\d+)')::int),0)+1
		FROM ohs_penalties WHERE project_id=$1`, pid).Scan(&seq); err != nil {
		httpx.Internal(w, r)
		return
	}
	penaltyNo := fmt.Sprintf("ISG-%03d", seq)

	var id uuid.UUID
	var issuedAt time.Time
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO ohs_penalties
			(project_id, subcontractor_id, finding_id, penalty_no, violation_type,
			 penalty_level, amount, note, issued_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF(TRIM($8),''),$9)
		RETURNING id, issued_at`,
		pid, subID, findingID, penaltyNo, strDeref(&req.ViolationType),
		req.PenaltyLevel, req.Amount, strDeref(req.Note), uid).Scan(&id, &issuedAt); err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Bulgu bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	// Kanıt fotoğrafı (opsiyonel; doküman motoru).
	if req.EvidenceBase64 != nil && *req.EvidenceBase64 != "" {
		docID, perr := h.savePhotoDocument(r.Context(), tx, pid, uid,
			penaltyNo+" — kanıt fotoğrafı", "OHS", "ohs_penalty", id,
			*req.EvidenceBase64, strDeref(req.EvidenceName))
		if perr != nil {
			if errors.Is(perr, errPhotoTooLarge) || errors.Is(perr, errPhotoInvalid) {
				httpx.ValidationFailed(w, r, map[string]string{"evidence_base64": perr.Error()})
				return
			}
			h.log.Error("ceza kanıtı yazılamadı", "err", perr)
			httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Kanıt deposuna yazılamadı.", nil)
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE ohs_penalties SET evidence_document_id=$2 WHERE id=$1`, id, docID); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	// Denetleyen adı (PDF için).
	var issuerName string
	_ = tx.QueryRow(r.Context(), `SELECT full_name FROM users WHERE id=$1`, uid).Scan(&issuerName)

	// PDF: senkron üret, MinIO'ya yaz, files kaydı aç, tutanağa bağla.
	pdf := BuildPenaltyPDF(PenaltyPDFData{
		PenaltyNo: penaltyNo, ProjectName: projName, ProjectCode: projCode,
		Subcontractor: subName, ViolationType: strDeref(&req.ViolationType),
		PenaltyLevel: req.PenaltyLevel, Amount: req.Amount,
		Note: strDeref(req.Note), IssuedBy: issuerName,
		IssuedAt:    issuedAt.Format("02.01.2006 15:04"),
		HasEvidence: req.EvidenceBase64 != nil && *req.EvidenceBase64 != "",
	})
	key := fmt.Sprintf("project/%s/ohs/penalties/%s.pdf", pid, id)
	sum := sha256Hex(pdf)
	if err := h.store.PutObject(r.Context(), key, "application/pdf",
		bytes.NewReader(pdf), int64(len(pdf)), sum); err != nil {
		h.log.Error("ceza PDF'i depoya yazılamadı", "err", err, "key", key)
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Tutanak PDF'i yazılamadı.", nil)
		return
	}
	var pdfFileID uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO files (storage_key, original_name, mime, size_bytes, sha256, uploaded_by)
		VALUES ($1,$2,'application/pdf',$3,$4,$5) RETURNING id`,
		key, fmt.Sprintf("isg-ceza-%s-%s.pdf", projCode, penaltyNo), len(pdf), sum, uid).Scan(&pdfFileID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE ohs_penalties SET pdf_file_id=$2 WHERE id=$1`, id, pdfFileID); err != nil {
		httpx.Internal(w, r)
		return
	}

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('ohs_penalty', $1, NULL, 'Issued', $2, $3)`, id, uid, penaltyNo); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_penalties", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"penalty_no": penaltyNo, "level": req.PenaltyLevel,
			"amount": req.Amount, "subcontractor_id": subID},
		IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Bildirimler: taşeron temsilcileri + PY/onay yetkilileri (Plan Faz 8).
	body := "İhlal: " + strDeref(&req.ViolationType)
	if req.Amount != nil {
		body += fmt.Sprintf(" — Tutar: %.2f TL", *req.Amount)
	}
	in := notify.Input{
		Type:  notify.TypeOHSPenaltyIssued,
		Title: penaltyNo + " — İSG ceza tutanağı kesildi (" + subName + ")",
		Body:  body, EntityType: "ohs_penalties", EntityID: &id, ProjectID: &pid,
	}
	h.notifySubReps(r.Context(), pid, subID, in)
	h.notifyByCapability(r.Context(), pid, "progress_payments.finalize", uid, in)

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id": id, "penalty_no": penaltyNo, "pdf_ready": true,
	})
}

// AcknowledgePenalty — taşeron temsilcisi tutanağı tebellüğ eder
// (Issued → Acknowledged). Kesinti önerisini etkilemez; izlenebilirlik içindir.
func (h *Handler) AcknowledgePenalty(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var subID uuid.UUID
	var status string
	err = tx.QueryRow(r.Context(), `
		SELECT subcontractor_id, status FROM ohs_penalties
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, pid).
		Scan(&subID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ceza tutanağı bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if scope == nil || *scope != subID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Tebellüğ yalnızca ilgili taşeron temsilcisi tarafından yapılır.", nil)
		return
	}
	if status != "Issued" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Tutanak zaten tebellüğ edilmiş.", map[string]string{"status": status})
		return
	}
	if _, err := tx.Exec(r.Context(), `
		UPDATE ohs_penalties SET status='Acknowledged', row_version=row_version+1 WHERE id=$1`, id); err != nil {
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('ohs_penalty', $1, 'Issued', 'Acknowledged', $2, NULL)`, id, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_penalties", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": "Issued"}, After: map[string]interface{}{"status": "Acknowledged"},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "Acknowledged"})
}

// DownloadPenaltyPDF — tutanak PDF'ini kimlikli akışla indirir (depolama
// anahtarı istemciye sızmaz — haftalık rapor indirme deseni).
func (h *Handler) DownloadPenaltyPDF(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	var subID uuid.UUID
	var key, name string
	err := h.pool.QueryRow(r.Context(), `
		SELECT p.subcontractor_id, f.storage_key, f.original_name
		FROM ohs_penalties p JOIN files f ON f.id = p.pdf_file_id
		WHERE p.id=$1 AND p.project_id=$2 AND p.deleted_at IS NULL`, id, pid).
		Scan(&subID, &key, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Tutanak PDF'i bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if scope != nil && *scope != subID {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu tutanağa erişiminiz yok.", nil)
		return
	}
	obj, err := h.store.GetObject(r.Context(), key)
	if err != nil {
		h.log.Error("ceza PDF'i okunamadı", "err", err, "key", key)
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Dosya deposundan okunamadı.", nil)
		return
	}
	defer obj.Body.Close()
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	_, _ = io.Copy(w, obj.Body)
}
