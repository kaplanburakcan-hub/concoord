package ohs

import (
	"encoding/json"
	"errors"
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
// Checklist şablonları (global; admin `ohs.manage_checklists` ile yönetir)
// ---------------------------------------------------------------------------

type templateDTO struct {
	ID         uuid.UUID      `json:"id"`
	Name       string         `json:"name"`
	Category   string         `json:"category"`
	Items      []TemplateItem `json:"items"`
	IsActive   bool           `json:"is_active"`
	RowVersion int            `json:"row_version"`
	CreatedAt  time.Time      `json:"created_at"`
}

func scanTemplate(row pgx.Row, t *templateDTO) error {
	var itemsJSON []byte
	if err := row.Scan(&t.ID, &t.Name, &t.Category, &itemsJSON, &t.IsActive, &t.RowVersion, &t.CreatedAt); err != nil {
		return err
	}
	if err := json.Unmarshal(itemsJSON, &t.Items); err != nil {
		t.Items = nil
	}
	return nil
}

const templateCols = `id, name, category, items, is_active, row_version, created_at`

// ListTemplates — ?active=true yalnız aktifleri döner (denetim ekranı bunu kullanır).
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	onlyActive := r.URL.Query().Get("active") == "true"
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+templateCols+` FROM ohs_checklist_templates
		WHERE deleted_at IS NULL AND ($1 = false OR is_active)
		ORDER BY category, name`, onlyActive)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	list := []templateDTO{}
	for rows.Next() {
		var t templateDTO
		if err := scanTemplate(rows, &t); err != nil {
			httpx.Internal(w, r)
			return
		}
		list = append(list, t)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"templates": list})
}

type templateReq struct {
	Name     string         `json:"name"`
	Category string         `json:"category"`
	Items    []TemplateItem `json:"items"`
	IsActive *bool          `json:"is_active"`
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req templateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.ValidationFailed(w, r, map[string]string{"name": "şablon adı zorunludur"})
		return
	}
	if errs := validateTemplateItems(req.Items); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	if strings.TrimSpace(req.Category) == "" {
		req.Category = "Genel"
	}
	itemsJSON, _ := json.Marshal(req.Items)
	uid, _ := auth.UserIDFrom(r.Context())

	var t templateDTO
	err := scanTemplate(h.pool.QueryRow(r.Context(), `
		INSERT INTO ohs_checklist_templates (name, category, items, created_by)
		VALUES ($1,$2,$3,$4) RETURNING `+templateCols,
		req.Name, strings.TrimSpace(req.Category), itemsJSON, uid), &t)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu adla bir şablon zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_checklist_templates", EntityID: t.ID.String(),
		Action: audit.ActionInsert, After: map[string]interface{}{"name": t.Name, "items": len(t.Items)},
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"template": t})
}

type templateUpdateReq struct {
	Name       *string         `json:"name"`
	Category   *string         `json:"category"`
	Items      *[]TemplateItem `json:"items"`
	IsActive   *bool           `json:"is_active"`
	RowVersion int             `json:"row_version"`
}

// UpdateTemplate — şablon güncellenir; GEÇMİŞ denetimler etkilenmez (denetim,
// yanıtladığı maddeleri results içinde taşır; şablona geri dönmez).
func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req templateUpdateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Items != nil {
		if errs := validateTemplateItems(*req.Items); len(errs) > 0 {
			httpx.ValidationFailed(w, r, errs)
			return
		}
	}
	var itemsJSON []byte
	if req.Items != nil {
		itemsJSON, _ = json.Marshal(*req.Items)
	}
	var t templateDTO
	err := scanTemplate(h.pool.QueryRow(r.Context(), `
		UPDATE ohs_checklist_templates SET
			name       = COALESCE(NULLIF(TRIM($2),''), name),
			category   = COALESCE(NULLIF(TRIM($3),''), category),
			items      = COALESCE($4, items),
			is_active  = COALESCE($5, is_active),
			row_version = row_version + 1
		WHERE id=$1 AND deleted_at IS NULL AND row_version=$6
		RETURNING `+templateCols,
		id, strDeref(req.Name), strDeref(req.Category), itemsJSON, req.IsActive, req.RowVersion), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Şablon bulunamadı ya da başkası tarafından güncellendi.", nil)
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu adla bir şablon zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_checklist_templates", EntityID: id.String(),
		Action: audit.ActionUpdate, After: map[string]interface{}{"name": t.Name, "is_active": t.IsActive},
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"template": t})
}

// DeleteTemplate — soft delete; geçmiş denetimlerin FK'si RESTRICT olduğundan
// yalnızca deleted_at işaretlenir, fiziksel silme yoktur (Plan §5.1).
func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE ohs_checklist_templates SET deleted_at=now(), row_version=row_version+1
		WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Şablon bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_checklist_templates", EntityID: id.String(),
		Action: audit.ActionDelete, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Denetimler
// ---------------------------------------------------------------------------

type inspectionDTO struct {
	ID            uuid.UUID    `json:"id"`
	TemplateID    uuid.UUID    `json:"template_id"`
	TemplateName  string       `json:"template_name"`
	InspectorID   uuid.UUID    `json:"inspector_id"`
	InspectorName string       `json:"inspector_name"`
	InspectedAt   time.Time    `json:"inspected_at"`
	LocationText  *string      `json:"location_text,omitempty"`
	GpsLat        *float64     `json:"gps_lat,omitempty"`
	GpsLng        *float64     `json:"gps_lng,omitempty"`
	Results       []ResultItem `json:"results,omitempty"`
	Score         *float64     `json:"score,omitempty"`
	FailCount     int          `json:"fail_count"`
	CreatedAt     time.Time    `json:"created_at"`
}

func (h *Handler) ListInspections(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT i.id, i.template_id, t.name, i.inspector_id, u.full_name,
		       i.inspected_at, i.location_text, i.gps_lat, i.gps_lng,
		       i.score::float8,
		       (SELECT count(*) FROM jsonb_array_elements(i.results) e
		         WHERE e->>'answer' = 'fail'),
		       i.created_at
		FROM ohs_inspections i
		JOIN ohs_checklist_templates t ON t.id = i.template_id
		JOIN users u ON u.id = i.inspector_id
		WHERE i.project_id=$1 AND i.deleted_at IS NULL
		ORDER BY i.inspected_at DESC
		LIMIT 500`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	list := []inspectionDTO{}
	for rows.Next() {
		var d inspectionDTO
		if err := rows.Scan(&d.ID, &d.TemplateID, &d.TemplateName, &d.InspectorID, &d.InspectorName,
			&d.InspectedAt, &d.LocationText, &d.GpsLat, &d.GpsLng, &d.Score, &d.FailCount, &d.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		list = append(list, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"inspections": list})
}

func (h *Handler) GetInspection(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var d inspectionDTO
	var resultsJSON []byte
	err := h.pool.QueryRow(r.Context(), `
		SELECT i.id, i.template_id, t.name, i.inspector_id, u.full_name,
		       i.inspected_at, i.location_text, i.gps_lat, i.gps_lng, i.results,
		       i.score::float8, i.created_at
		FROM ohs_inspections i
		JOIN ohs_checklist_templates t ON t.id = i.template_id
		JOIN users u ON u.id = i.inspector_id
		WHERE i.id=$1 AND i.project_id=$2 AND i.deleted_at IS NULL`, id, pid).
		Scan(&d.ID, &d.TemplateID, &d.TemplateName, &d.InspectorID, &d.InspectorName,
			&d.InspectedAt, &d.LocationText, &d.GpsLat, &d.GpsLng, &resultsJSON, &d.Score, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Denetim bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	_ = json.Unmarshal(resultsJSON, &d.Results)
	d.FailCount = 0
	for _, res := range d.Results {
		if res.Answer == "fail" {
			d.FailCount++
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"inspection": d})
}

type inspectionReq struct {
	TemplateID   string       `json:"template_id"`
	InspectedAt  *time.Time   `json:"inspected_at"` // offline girişte cihaz saati
	LocationText *string      `json:"location_text"`
	GpsLat       *float64     `json:"gps_lat"`
	GpsLng       *float64     `json:"gps_lng"`
	Results      []ResultItem `json:"results"`
}

// CreateInspection — mobil denetim gönderimi. Denetim tek adımda oluşur
// (Submitted): sahada tamamlanır, sunucuda taslak yaşamaz — offline kuyruk
// zaten cihaz tarafında taslağı taşır. Skor sunucuda hesaplanır (güvenilir).
func (h *Handler) CreateInspection(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())
	var req inspectionReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tmplID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"template_id": "geçersiz UUID"})
		return
	}

	// Şablon maddeleri — sonuç doğrulaması şablona karşı yapılır.
	var itemsJSON []byte
	var active bool
	err = h.pool.QueryRow(r.Context(), `
		SELECT items, is_active FROM ohs_checklist_templates
		WHERE id=$1 AND deleted_at IS NULL`, tmplID).Scan(&itemsJSON, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Checklist şablonu bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if !active {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Pasif şablonla denetim yapılamaz.", nil)
		return
	}
	var tmplItems []TemplateItem
	_ = json.Unmarshal(itemsJSON, &tmplItems)
	if errs := validateResults(tmplItems, req.Results); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}

	inspectedAt := time.Now()
	if req.InspectedAt != nil && !req.InspectedAt.IsZero() {
		inspectedAt = *req.InspectedAt
	}
	resultsJSON, _ := json.Marshal(req.Results)
	score := Score(req.Results)

	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO ohs_inspections
			(project_id, template_id, inspector_id, inspected_at, location_text, gps_lat, gps_lng, results, score)
		VALUES ($1,$2,$3,$4,NULLIF(TRIM($5),''),$6,$7,$8,$9)
		RETURNING id`,
		pid, tmplID, uid, inspectedAt, strDeref(req.LocationText),
		req.GpsLat, req.GpsLng, resultsJSON, score).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_inspections", EntityID: id.String(),
		Action: audit.ActionInsert,
		After:  map[string]interface{}{"template_id": tmplID, "score": score},
		IP:     m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"id": id, "score": score})
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
