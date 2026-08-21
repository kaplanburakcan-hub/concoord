package ohs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/validate"
)

// ---------------------------------------------------------------------------
// Bulgular
// ---------------------------------------------------------------------------

type findingDTO struct {
	ID                uuid.UUID  `json:"id"`
	InspectionID      *uuid.UUID `json:"inspection_id,omitempty"`
	SubcontractorID   *uuid.UUID `json:"subcontractor_id,omitempty"`
	SubcontractorName *string    `json:"subcontractor_name,omitempty"`
	Severity          string     `json:"severity"`
	Description       string     `json:"description"`
	Location          *string    `json:"location,omitempty"`
	GpsLat            *float64   `json:"gps_lat,omitempty"`
	GpsLng            *float64   `json:"gps_lng,omitempty"`
	PhotoDocumentID   *uuid.UUID `json:"photo_document_id,omitempty"`
	DueDate           *string    `json:"due_date,omitempty"`
	Status            string     `json:"status"`
	Overdue           bool       `json:"overdue"`  // termini geçmiş, kapanmamış
	AgeDays           int        `json:"age_days"` // yaşlandırma (haftalık rapor tüketir)
	ReportedBy        uuid.UUID  `json:"reported_by"`
	ReportedByName    string     `json:"reported_by_name"`
	ClosedByName      *string    `json:"closed_by_name,omitempty"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	CloseNote         *string    `json:"close_note,omitempty"`
	RowVersion        int        `json:"row_version"`
	CreatedAt         time.Time  `json:"created_at"`
}

const findingSelect = `
	SELECT f.id, f.inspection_id, f.subcontractor_id, s.company_name,
	       f.severity, f.description, f.location, f.gps_lat, f.gps_lng,
	       f.photo_document_id, to_char(f.due_date,'YYYY-MM-DD'),
	       f.status,
	       (f.due_date IS NOT NULL AND f.due_date < CURRENT_DATE AND f.status <> 'Closed'),
	       GREATEST(0, (CURRENT_DATE - f.created_at::date)),
	       f.reported_by, ru.full_name, cu.full_name, f.closed_at, f.close_note,
	       f.row_version, f.created_at
	FROM ohs_findings f
	LEFT JOIN subcontractors s ON s.id = f.subcontractor_id
	JOIN users ru ON ru.id = f.reported_by
	LEFT JOIN users cu ON cu.id = f.closed_by
`

func scanFinding(row pgx.Row, d *findingDTO) error {
	return row.Scan(&d.ID, &d.InspectionID, &d.SubcontractorID, &d.SubcontractorName,
		&d.Severity, &d.Description, &d.Location, &d.GpsLat, &d.GpsLng,
		&d.PhotoDocumentID, &d.DueDate, &d.Status, &d.Overdue, &d.AgeDays,
		&d.ReportedBy, &d.ReportedByName, &d.ClosedByName, &d.ClosedAt, &d.CloseNote,
		&d.RowVersion, &d.CreatedAt)
}

// ListFindings — ?status= ile filtre; taşeron temsilcisi yalnızca kendi
// firmasının bulgularını görür (satır seviyesi güvenlik, backend'de zorunlu).
func (h *Handler) ListFindings(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	_, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	rows, err := h.pool.Query(r.Context(), findingSelect+`
		WHERE f.project_id=$1 AND f.deleted_at IS NULL
		  AND ($2='' OR f.status=$2)
		  AND ($3::uuid IS NULL OR f.subcontractor_id=$3)
		ORDER BY f.created_at DESC
		LIMIT 500`, pid, status, scope)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	list := []findingDTO{}
	for rows.Next() {
		var d findingDTO
		if err := scanFinding(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		list = append(list, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"findings": list})
}

type findingReq struct {
	InspectionID    *string  `json:"inspection_id"`
	SubcontractorID *string  `json:"subcontractor_id"`
	Severity        string   `json:"severity"`
	Description     string   `json:"description"`
	Location        *string  `json:"location"`
	GpsLat          *float64 `json:"gps_lat"`
	GpsLng          *float64 `json:"gps_lng"`
	DueDate         *string  `json:"due_date"` // YYYY-MM-DD
	// Offline kuyruk JSON-only olduğundan foto data-URL olarak gömülür (≤ 8 MB).
	PhotoBase64 *string `json:"photo_base64"`
	PhotoName   *string `json:"photo_name"`
}

// CreateFinding — foto + GPS/lokasyonlu bulgu kaydı (mobil öncelikli).
// Fotoğraf tek istekte doküman motoruna yazılır: offline kuyruğun JSON-only
// kısıtıyla uyum (bkz. ADR-0009 §3).
func (h *Handler) CreateFinding(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, scope, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	if scope != nil {
		// Bulgu düzenleme saha tarafının işidir; taşeron temsilcisi bulgu açamaz.
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Taşeron hesabıyla bulgu açılamaz.", nil)
		return
	}
	var req findingReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	errs := map[string]string{}
	if !Severities[req.Severity] {
		errs["severity"] = "Observation/Minor/Major/Critical olmalı"
	}
	if strDeref(&req.Description) == "" {
		errs["description"] = "açıklama zorunludur"
	}
	if dueDate := strDeref(req.DueDate); dueDate != "" {
		t, perr := time.Parse("2006-01-02", dueDate)
		if perr != nil {
			errs["due_date"] = "geçersiz tarih biçimi"
		} else if kerrs, kerr := validate.NotAfterKesinKabul(r.Context(), h.pool, pid, t, "due_date"); kerr != nil {
			httpx.Internal(w, r)
			return
		} else if len(kerrs) > 0 {
			for k, v := range kerrs {
				errs[k] = v
			}
		}
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}

	var inspID, subID *uuid.UUID
	if req.InspectionID != nil && *req.InspectionID != "" {
		v, err := uuid.Parse(*req.InspectionID)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"inspection_id": "geçersiz UUID"})
			return
		}
		inspID = &v
	}
	if req.SubcontractorID != nil && *req.SubcontractorID != "" {
		v, err := uuid.Parse(*req.SubcontractorID)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "geçersiz UUID"})
			return
		}
		subID = &v
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO ohs_findings
			(project_id, inspection_id, subcontractor_id, severity, description,
			 location, gps_lat, gps_lng, due_date, reported_by)
		VALUES ($1,$2,$3,$4,$5,NULLIF(TRIM($6),''),$7,$8,NULLIF($9,'')::date,$10)
		RETURNING id`,
		pid, inspID, subID, req.Severity, strDeref(&req.Description),
		strDeref(req.Location), req.GpsLat, req.GpsLng, strDeref(req.DueDate), uid).Scan(&id)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation,
				"Denetim ya da taşeron bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	// Foto varsa: doküman motoru üzerinden, bulguya polimorfik bağla.
	if req.PhotoBase64 != nil && *req.PhotoBase64 != "" {
		docID, perr := h.savePhotoDocument(r.Context(), tx, pid, uid,
			fmt.Sprintf("İSG bulgu fotoğrafı — %s", req.Severity), "OHS",
			"ohs_finding", id, *req.PhotoBase64, strDeref(req.PhotoName))
		if perr != nil {
			if errors.Is(perr, errPhotoTooLarge) || errors.Is(perr, errPhotoInvalid) {
				httpx.ValidationFailed(w, r, map[string]string{"photo_base64": perr.Error()})
				return
			}
			h.log.Error("bulgu fotoğrafı yazılamadı", "err", perr)
			httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Fotoğraf deposuna yazılamadı.", nil)
			return
		}
		if _, err := tx.Exec(r.Context(),
			`UPDATE ohs_findings SET photo_document_id=$2 WHERE id=$1`, id, docID); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('ohs_finding', $1, NULL, 'Open', $2, NULL)`, id, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_findings", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"severity": req.Severity, "subcontractor_id": subID},
		IP:    m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

type findingTransitionReq struct {
	Note       string `json:"note"`
	RowVersion int    `json:"row_version"`
}

// StartFinding — Open → InProgress.
func (h *Handler) StartFinding(w http.ResponseWriter, r *http.Request) {
	h.transitionFinding(w, r, "InProgress", false)
}

// CloseFinding — Open/InProgress → Closed. Kapatma notu ZORUNLU (izlenebilirlik).
func (h *Handler) CloseFinding(w http.ResponseWriter, r *http.Request) {
	h.transitionFinding(w, r, "Closed", true)
}

func (h *Handler) transitionFinding(w http.ResponseWriter, r *http.Request, to string, noteRequired bool) {
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
	var req findingTransitionReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if noteRequired && strDeref(&req.Note) == "" {
		httpx.ValidationFailed(w, r, map[string]string{"note": "kapatma notu zorunludur"})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var from string
	var subID *uuid.UUID
	var rowVersion int
	err = tx.QueryRow(r.Context(), `
		SELECT status, subcontractor_id, row_version FROM ohs_findings
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, pid).
		Scan(&from, &subID, &rowVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Bulgu bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	// Taşeron temsilcisi yalnızca kendi firmasının bulgusunu İLERLETEBİLİR
	// (InProgress); kapatma saha tarafının onayıdır.
	if scope != nil {
		if subID == nil || *scope != *subID || to == "Closed" {
			httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden, "Bu bulgu üzerinde işlem yetkiniz yok.", nil)
			return
		}
	}
	if !CanTransitionFinding(from, to) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Geçersiz durum geçişi.", map[string]string{"from": from, "to": to})
		return
	}
	if req.RowVersion != 0 && req.RowVersion != rowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": rowVersion})
		return
	}

	var q string
	if to == "Closed" {
		q = `UPDATE ohs_findings SET status='Closed', closed_by=$2, closed_at=now(),
		     close_note=$3, row_version=row_version+1 WHERE id=$1`
		_, err = tx.Exec(r.Context(), q, id, uid, strDeref(&req.Note))
	} else {
		q = `UPDATE ohs_findings SET status=$2, row_version=row_version+1 WHERE id=$1`
		_, err = tx.Exec(r.Context(), q, id, to)
	}
	if err != nil {
		if isLockViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Kapanmış bulgu değiştirilemez.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('ohs_finding', $1, $2, $3, $4, NULLIF($5,''))`,
		id, from, to, uid, req.Note); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_findings", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": from}, After: map[string]interface{}{"status": to},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": to})
}

// ---------------------------------------------------------------------------
// Termin taraması (worker saatlik çağırır — satınalma PO taramasıyla aynı desen)
// ---------------------------------------------------------------------------

// NotifyOverdueFindings — termini geçmiş, kapanmamış ve henüz bildirilmemiş
// bulgular için raporlayana + İSG uzmanlarına/PY'ye bildirim üretir.
// overdue_notified_at ile işaretlenir; termin değişmediği sürece tekrarlanmaz.
func NotifyOverdueFindings(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service, log *slog.Logger) {
	rows, err := pool.Query(ctx, `
		SELECT f.id, f.project_id, f.severity, f.description,
		       to_char(f.due_date,'YYYY-MM-DD'), f.reported_by
		FROM ohs_findings f
		WHERE f.deleted_at IS NULL AND f.status <> 'Closed'
		  AND f.due_date IS NOT NULL AND f.due_date < CURRENT_DATE
		  AND f.overdue_notified_at IS NULL
		LIMIT 200`)
	if err != nil {
		log.Error("İSG termin taraması başarısız", "err", err)
		return
	}
	type row struct {
		id, pid        uuid.UUID
		sev, desc, due string
		reportedBy     uuid.UUID
	}
	var overdue []row
	for rows.Next() {
		var x row
		if rows.Scan(&x.id, &x.pid, &x.sev, &x.desc, &x.due, &x.reportedBy) == nil {
			overdue = append(overdue, x)
		}
	}
	rows.Close()

	for _, x := range overdue {
		desc := x.desc
		if len(desc) > 80 {
			desc = desc[:80] + "…"
		}
		id := x.id
		pid := x.pid
		nt.Send(ctx, notify.Input{
			UserIDs:    []uuid.UUID{x.reportedBy},
			Type:       notify.TypeOHSFindingOverdue,
			Title:      "İSG bulgusu termini geçti (" + x.sev + ")",
			Body:       "Termin: " + x.due + " — " + desc,
			EntityType: "ohs_findings", EntityID: &id, ProjectID: &pid,
		})
		if _, err := pool.Exec(ctx, `
			UPDATE ohs_findings SET overdue_notified_at=now(), row_version=row_version+1
			WHERE id=$1`, x.id); err != nil {
			log.Error("İSG termin işareti yazılamadı", "id", x.id, "err", err)
		}
	}
	if len(overdue) > 0 {
		log.Info("İSG termin bildirimi üretildi", "adet", len(overdue))
	}
}
