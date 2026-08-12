// Package customreports — Proje İzleme Raporları sayfasındaki "özel rapor
// ekle" özelliği. Önceden tamamen React state'indeydi (sayfa yenilenince
// kaybolan bir liste); bu paket onu gerçek, projeler ve kullanıcılar
// arasında paylaşılan bir varlığa taşır.
package customreports

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

type CustomReport struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"project_id"`
	Label       string    `json:"label"`
	Description *string   `json:"description,omitempty"`
	RowVersion  int       `json:"row_version"`
	CreatedAt   time.Time `json:"created_at"`
}

const cols = `id, project_id, label, description, row_version, created_at`

func scanRow(row pgx.Row, c *CustomReport) error {
	return row.Scan(&c.ID, &c.ProjectID, &c.Label, &c.Description, &c.RowVersion, &c.CreatedAt)
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

func requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, false
	}
	return uid, true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+cols+` FROM custom_reports
		WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY created_at`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []CustomReport{}
	for rows.Next() {
		var c CustomReport
		if err := scanRow(rows, &c); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, c)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"custom_reports": out})
}

type createReq struct {
	Label       string  `json:"label"`
	Description *string `json:"description"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req createReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	if req.Label == "" {
		httpx.ValidationFailed(w, r, map[string]string{"label": "zorunlu"})
		return
	}
	var desc *string
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if d != "" {
			desc = &d
		}
	}
	var c CustomReport
	err := scanRow(h.pool.QueryRow(r.Context(), `
		INSERT INTO custom_reports (project_id, label, description, created_by)
		VALUES ($1,$2,$3,$4)
		RETURNING `+cols,
		pid, req.Label, desc, uid), &c)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"custom_report": c})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE custom_reports SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Rapor bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
