package correspondence

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// DocLinkDTO — yazışmaya bağlı bir belge.
type DocLinkDTO struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	LinkedBy string    `json:"linked_by_name"`
	LinkedAt string    `json:"linked_at"`
}

// ListDocLinks — yazışmaya bağlı belgeler (silinmemiş belgeler).
func (h *Handler) ListDocLinks(w http.ResponseWriter, r *http.Request) {
	pid, cid, ok := ids(w, r)
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT d.id, d.title, u.full_name, to_char(l.created_at,'YYYY-MM-DD')
		FROM correspondence_document_links l
		JOIN correspondences c ON c.id = l.correspondence_id AND c.project_id = $2
		JOIN documents d ON d.id = l.document_id AND d.deleted_at IS NULL
		JOIN users u ON u.id = l.created_by
		WHERE l.correspondence_id = $1
		ORDER BY l.created_at`, cid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []DocLinkDTO{}
	for rows.Next() {
		var d DocLinkDTO
		if err := rows.Scan(&d.ID, &d.Title, &d.LinkedBy, &d.LinkedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"documents": out})
}

// AddDocLink — mevcut bir proje belgesini yazışmaya bağlar. Aynı bağ ikinci kez eklenemez (409).
func (h *Handler) AddDocLink(w http.ResponseWriter, r *http.Request) {
	pid, cid, ok := ids(w, r)
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	var in struct {
		DocumentID string `json:"document_id"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	did, err := uuid.Parse(in.DocumentID)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"document_id": "geçersiz kimlik"})
		return
	}
	// Yazışma, URL'deki projeye ait olmalı.
	var one int
	if err := h.pool.QueryRow(r.Context(), `
		SELECT 1 FROM correspondences WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, cid, pid).Scan(&one); err != nil {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Yazışma bulunamadı.", nil)
		return
	}
	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO correspondence_document_links (correspondence_id, document_id, created_by)
		VALUES ($1,$2,$3)`, cid, did, uid); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "duplicate key"):
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu belge zaten bağlı.", nil)
		case strings.Contains(msg, "aynı projeye ait değil"):
			httpx.ValidationFailed(w, r, map[string]string{"document_id": "bu projede kayıtlı belge değil"})
		default:
			httpx.Internal(w, r)
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{"status": "linked"})
}

// RemoveDocLink — bağı kaldırır; belge ve yazışma kaydı kalır.
func (h *Handler) RemoveDocLink(w http.ResponseWriter, r *http.Request) {
	pid, cid, ok := ids(w, r)
	if !ok {
		return
	}
	did, err := uuid.Parse(chi.URLParam(r, "docID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		DELETE FROM correspondence_document_links l
		USING correspondences c
		WHERE c.id = l.correspondence_id AND c.project_id = $3
		  AND l.correspondence_id=$1 AND l.document_id=$2`, cid, did, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Bağlantı bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

func ids(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, uuid.Nil, false
	}
	cid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, uuid.Nil, false
	}
	return pid, cid, true
}
