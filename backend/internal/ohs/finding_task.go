package ohs

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// CreateTaskFromFinding — İSG bulgusundan düzeltici görev (B11). Bir bulgu en
// fazla bir görevle eşleşir; ikinci istek 409 ile mevcut görev kimliğini döner.
// Bulgu kaydı değişmez. Kapalı bulgudan görev açılmaz.
func (h *Handler) CreateTaskFromFinding(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}

	var description, severity, status string
	var location *string
	var due *string
	err = h.pool.QueryRow(r.Context(), `
		SELECT description, severity, status, location, to_char(due_date,'YYYY-MM-DD')
		FROM ohs_findings WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, fid, pid).
		Scan(&description, &severity, &status, &location, &due)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Bulgu bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if status == "Closed" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Kapalı bulgudan görev açılamaz.", nil)
		return
	}

	title := strings.TrimSpace(description)
	if title == "" {
		httpx.ValidationFailed(w, r, map[string]string{"description": "bulgu açıklaması boş"})
		return
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	desc := "İSG bulgusu (" + severity + ")."
	if location != nil && strings.TrimSpace(*location) != "" {
		desc += " Konum: " + strings.TrimSpace(*location) + "."
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var taskID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		WITH t AS (
		  INSERT INTO tasks (project_id, title, description, due_date, created_by, source_module)
		  VALUES ($1, $2, $3, $4::date, $5, 'isg')
		  RETURNING id
		)
		INSERT INTO ohs_finding_tasks (finding_id, task_id, created_by)
		SELECT $6, t.id, $5 FROM t
		RETURNING task_id`, pid, title, desc, due, uid, fid).Scan(&taskID)
	if err != nil {
		var existing uuid.UUID
		if qerr := h.pool.QueryRow(r.Context(), `
			SELECT task_id FROM ohs_finding_tasks WHERE finding_id=$1`, fid).Scan(&existing); qerr == nil {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Bu bulgu için görev zaten açılmış.", map[string]interface{}{"task_id": existing})
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "ohs_finding_tasks", EntityID: fid.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"task_id": taskID.String()}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"task_id": taskID})
}
