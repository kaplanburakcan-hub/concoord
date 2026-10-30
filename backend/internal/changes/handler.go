// Package changes — değişiklik olayı kaydı (Plan E1, ilk adım).
//
// Bu aşamada yalnızca TASLAK kayıt vardır: onay akışı, sözleşme bedeli ve
// bütçe/hakediş etkisi sonraki adımlarda eklenecektir. Kayıt silinmez.
package changes

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder) *Handler {
	return &Handler{pool: pool, rec: rec}
}

// EventDTO — bir değişiklik olayı taslağı.
type EventDTO struct {
	ID              uuid.UUID `json:"id"`
	No              int       `json:"no"`
	Title           string    `json:"title"`
	Source          string    `json:"source"`
	Description     *string   `json:"description,omitempty"`
	EstimatedAmount *float64  `json:"estimated_amount,omitempty"`
	EstimatedDays   *int      `json:"estimated_days,omitempty"`
	Status          string    `json:"status"`
	CreatedByName   string    `json:"created_by_name"`
	CreatedAt       string    `json:"created_at"`
}

var sources = map[string]bool{"Sahip": true, "Tasarim": true, "SahaKosulu": true, "RFI": true, "HakTalebi": true, "Diger": true}

type eventInput struct {
	Title           string   `json:"title"`
	Source          string   `json:"source"`
	Description     *string  `json:"description"`
	EstimatedAmount *float64 `json:"estimated_amount"`
	EstimatedDays   *int     `json:"estimated_days"`
}

func validate(in eventInput) map[string]string {
	errs := map[string]string{}
	t := strings.TrimSpace(in.Title)
	if t == "" {
		errs["title"] = "başlık zorunlu"
	} else if len([]rune(t)) > 200 {
		errs["title"] = "en fazla 200 karakter"
	}
	if !sources[in.Source] {
		errs["source"] = "geçerli bir kaynak seçin"
	}
	if in.EstimatedAmount != nil && *in.EstimatedAmount < 0 {
		errs["estimated_amount"] = "negatif olamaz"
	}
	if in.EstimatedDays != nil && (*in.EstimatedDays < 0 || *in.EstimatedDays > 3650) {
		errs["estimated_days"] = "0 ile 3650 gün arasında olmalı"
	}
	return errs
}

const eventSelect = `
	SELECT e.id, e.event_no, e.title, e.source, e.description, e.estimated_amount::float8,
	       e.estimated_days, e.status, u.full_name, to_char(e.created_at,'YYYY-MM-DD')
	FROM change_events e
	JOIN users u ON u.id = e.created_by`

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), eventSelect+`
		WHERE e.project_id=$1 AND e.deleted_at IS NULL
		ORDER BY e.event_no DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []EventDTO{}
	for rows.Next() {
		var d EventDTO
		if err := rows.Scan(&d.ID, &d.No, &d.Title, &d.Source, &d.Description, &d.EstimatedAmount,
			&d.EstimatedDays, &d.Status, &d.CreatedByName, &d.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"events": out})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	var in eventInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := validate(in); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO change_events (project_id, event_no, title, source, description,
		                           estimated_amount, estimated_days, created_by)
		VALUES ($1,
		        (SELECT COALESCE(max(event_no),0)+1 FROM change_events WHERE project_id=$1),
		        $2,$3,NULLIF(btrim(coalesce($4,'')),''),$5,$6,$7)
		RETURNING id`,
		pid, strings.TrimSpace(in.Title), in.Source, in.Description,
		in.EstimatedAmount, in.EstimatedDays, uid).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "change_events", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"title": in.Title, "source": in.Source, "status": "Taslak"},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	var d EventDTO
	if err := h.pool.QueryRow(r.Context(), eventSelect+` WHERE e.id=$1`, id).Scan(&d.ID, &d.No, &d.Title,
		&d.Source, &d.Description, &d.EstimatedAmount, &d.EstimatedDays, &d.Status, &d.CreatedByName, &d.CreatedAt); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"event": d})
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}
