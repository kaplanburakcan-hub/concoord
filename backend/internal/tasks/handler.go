package tasks

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
)

// Handler — görev uçları. Yazımlar audit'li; statü geçişleri
// workflow_transitions'a yazılır (Plan §5.1); bildirimler notify servisiyle.
type Handler struct {
	pool *pgxpool.Pool
	eval *rbac.Evaluator
	rec  *audit.Recorder
	nt   *notify.Service
	log  *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder, nt *notify.Service, log *slog.Logger) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, nt: nt, log: log}
}

type taskDTO struct {
	ID          uuid.UUID     `json:"id"`
	ProjectID   uuid.UUID     `json:"project_id"`
	Title       string        `json:"title"`
	Description *string       `json:"description,omitempty"`
	Status      string        `json:"status"`
	Priority    string        `json:"priority"`
	DueDate     *time.Time    `json:"due_date,omitempty"`
	CreatedBy   uuid.UUID     `json:"created_by"`
	KanbanOrder float64       `json:"kanban_order"`
	RowVersion  int           `json:"row_version"`
	CreatedAt   time.Time     `json:"created_at"`
	Assignees   []assigneeDTO `json:"assignees"`
}

type assigneeDTO struct {
	UserID   uuid.UUID `json:"user_id"`
	FullName string    `json:"full_name"`
	Username string    `json:"username"`
}

const taskCols = `t.id, t.project_id, t.title, t.description, t.status, t.priority,
	t.due_date, t.created_by, t.kanban_order, t.row_version, t.created_at`

func scanTask(row pgx.Row, t *taskDTO) error {
	return row.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.DueDate, &t.CreatedBy, &t.KanbanOrder, &t.RowVersion, &t.CreatedAt)
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.",
			map[string]string{param: "geçersiz UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, false
	}
	return uid, true
}

// can — proje kapsamlı izin (handler içi ek kontroller için).
func (h *Handler) can(ctx context.Context, uid, pid uuid.UUID, code string) bool {
	ok, err := h.eval.Can(ctx, uid, &pid, code)
	return err == nil && ok
}

// canEdit — Plan §4: edit_all → her görev; edit_own → yalnızca oluşturduğu
// ya da kendisine atanmış görev.
func (h *Handler) canEdit(ctx context.Context, uid, pid uuid.UUID, t *taskDTO) bool {
	if h.can(ctx, uid, pid, "tasks.edit_all") {
		return true
	}
	if !h.can(ctx, uid, pid, "tasks.edit_own") {
		return false
	}
	if t.CreatedBy == uid {
		return true
	}
	for _, a := range t.Assignees {
		if a.UserID == uid {
			return true
		}
	}
	return false
}

func (h *Handler) loadAssignees(ctx context.Context, taskIDs []uuid.UUID) (map[uuid.UUID][]assigneeDTO, error) {
	out := map[uuid.UUID][]assigneeDTO{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	rows, err := h.pool.Query(ctx, `
		SELECT ta.task_id, u.id, u.full_name, u.username
		FROM task_assignees ta
		JOIN users u ON u.id = ta.user_id AND u.deleted_at IS NULL
		WHERE ta.task_id = ANY($1)
		ORDER BY u.full_name`, taskIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tid uuid.UUID
		var a assigneeDTO
		if err := rows.Scan(&tid, &a.UserID, &a.FullName, &a.Username); err != nil {
			return nil, err
		}
		out[tid] = append(out[tid], a)
	}
	return out, rows.Err()
}

// getTask — tek görev + atananlar; yoksa (nil, pgx.ErrNoRows).
func (h *Handler) getTask(ctx context.Context, pid, id uuid.UUID) (*taskDTO, error) {
	var t taskDTO
	err := scanTask(h.pool.QueryRow(ctx, `
		SELECT `+taskCols+` FROM tasks t
		WHERE t.id=$1 AND t.project_id=$2 AND t.deleted_at IS NULL`, id, pid), &t)
	if err != nil {
		return nil, err
	}
	as, err := h.loadAssignees(ctx, []uuid.UUID{t.ID})
	if err != nil {
		return nil, err
	}
	t.Assignees = as[t.ID]
	if t.Assignees == nil {
		t.Assignees = []assigneeDTO{}
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// Liste (Kanban kaynağı)
// ---------------------------------------------------------------------------

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+taskCols+` FROM tasks t
		WHERE t.project_id=$1 AND t.deleted_at IS NULL
		ORDER BY t.status, t.kanban_order, t.created_at`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []taskDTO{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var t taskDTO
		if err := scanTask(rows, &t); err != nil {
			httpx.Internal(w, r)
			return
		}
		t.Assignees = []assigneeDTO{}
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	as, err := h.loadAssignees(r.Context(), ids)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for i := range out {
		if a := as[out[i].ID]; a != nil {
			out[i].Assignees = a
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"tasks":        out,
		"status_order": StatusOrder,
	})
}

// AssignableUsers — atama ve @mention için proje üyeleri.
func (h *Handler) AssignableUsers(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT DISTINCT u.id, u.full_name, u.username
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id AND u.deleted_at IS NULL AND u.is_active
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL
		ORDER BY u.full_name`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []assigneeDTO{}
	for rows.Next() {
		var a assigneeDTO
		if err := rows.Scan(&a.UserID, &a.FullName, &a.Username); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, a)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"users": out})
}

// ---------------------------------------------------------------------------
// Oluşturma
// ---------------------------------------------------------------------------

type createTaskReq struct {
	Title       string   `json:"title"`
	Description *string  `json:"description"`
	Status      *string  `json:"status"`
	Priority    *string  `json:"priority"`
	DueDate     *string  `json:"due_date"` // YYYY-MM-DD
	AssigneeIDs []string `json:"assignee_ids"`
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var req createTaskReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	title, okT := ValidateTitle(req.Title)
	if !okT {
		httpx.ValidationFailed(w, r, map[string]string{"title": "zorunlu, en fazla 300 karakter"})
		return
	}
	status := "Backlog"
	if req.Status != nil {
		if !ValidStatus(*req.Status) {
			httpx.ValidationFailed(w, r, map[string]string{"status": "geçersiz statü"})
			return
		}
		status = *req.Status
	}
	priority := "Normal"
	if req.Priority != nil {
		if !ValidPriority(*req.Priority) {
			httpx.ValidationFailed(w, r, map[string]string{"priority": "Low|Normal|High|Urgent"})
			return
		}
		priority = *req.Priority
	}
	var due *time.Time
	if req.DueDate != nil && *req.DueDate != "" {
		d, err := time.Parse("2006-01-02", *req.DueDate)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"due_date": "YYYY-MM-DD biçiminde olmalı"})
			return
		}
		due = &d
	}
	if len(req.AssigneeIDs) > 0 && !h.can(r.Context(), uid, pid, "tasks.assign") {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Görev atamak için tasks.assign izni gerekli.", nil)
		return
	}

	var id uuid.UUID
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO tasks (project_id, title, description, status, priority, due_date, created_by, kanban_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,
		        COALESCE((SELECT max(kanban_order)+1 FROM tasks
		                  WHERE project_id=$1 AND status=$4 AND deleted_at IS NULL), 1))
		RETURNING id`,
		pid, title, req.Description, status, priority, due, uid).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	assigned := h.setAssignees(w, r, pid, id, uid, req.AssigneeIDs, true)
	if !assigned {
		return
	}

	t, err := h.getTask(r.Context(), pid, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "tasks", EntityID: id.String(),
		Action: audit.ActionInsert, After: t, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"task": t})
}

// setAssignees — atama listesini uygular ve YENİ atananlara bildirim yollar.
// Hata durumunda yanıtı yazar ve false döner.
func (h *Handler) setAssignees(w http.ResponseWriter, r *http.Request, pid, taskID, actor uuid.UUID, ids []string, isNew bool) bool {
	if ids == nil {
		return true
	}
	want := map[uuid.UUID]bool{}
	for _, s := range ids {
		u, err := uuid.Parse(s)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"assignee_ids": "geçersiz UUID: " + s})
			return false
		}
		want[u] = true
	}
	// Atanacaklar proje üyesi olmalı — görevi göremeyecek birine atama yapılmaz.
	for u := range want {
		var n int
		if err := h.pool.QueryRow(r.Context(), `
			SELECT count(*) FROM project_members
			WHERE project_id=$1 AND user_id=$2 AND deleted_at IS NULL`, pid, u).Scan(&n); err != nil {
			httpx.Internal(w, r)
			return false
		}
		if n == 0 {
			httpx.ValidationFailed(w, r, map[string]string{"assignee_ids": "kullanıcı proje üyesi değil: " + u.String()})
			return false
		}
	}

	current := map[uuid.UUID]bool{}
	rows, err := h.pool.Query(r.Context(), `SELECT user_id FROM task_assignees WHERE task_id=$1`, taskID)
	if err != nil {
		httpx.Internal(w, r)
		return false
	}
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return false
		}
		current[u] = true
	}
	rows.Close()

	newOnes := []uuid.UUID{}
	for u := range want {
		if !current[u] {
			newOnes = append(newOnes, u)
		}
	}
	for u := range current {
		if !want[u] {
			if _, err := h.pool.Exec(r.Context(),
				`DELETE FROM task_assignees WHERE task_id=$1 AND user_id=$2`, taskID, u); err != nil {
				httpx.Internal(w, r)
				return false
			}
		}
	}
	for _, u := range newOnes {
		if _, err := h.pool.Exec(r.Context(), `
			INSERT INTO task_assignees (task_id, user_id) VALUES ($1,$2)
			ON CONFLICT DO NOTHING`, taskID, u); err != nil {
			httpx.Internal(w, r)
			return false
		}
	}

	if len(newOnes) > 0 {
		var title string
		_ = h.pool.QueryRow(r.Context(), `SELECT title FROM tasks WHERE id=$1`, taskID).Scan(&title)
		// Atamayı yapan kişiye kendi bildirimi gitmez.
		recipients := []uuid.UUID{}
		for _, u := range newOnes {
			if u != actor {
				recipients = append(recipients, u)
			}
		}
		h.nt.Send(r.Context(), notify.Input{
			UserIDs: recipients, Type: notify.TypeTaskAssigned,
			Title: "Yeni görev atandı: " + title,
			Body:  "Size bir görev atandı. Detay için görev panosuna bakın.",
			EntityType: "tasks", EntityID: &taskID, ProjectID: &pid,
		})
	}
	_ = isNew
	return true
}

// ---------------------------------------------------------------------------
// Detay / Güncelleme / Taşıma / Silme / Atama
// ---------------------------------------------------------------------------

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	t, err := h.getTask(r.Context(), pid, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Görev bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"task": t})
}

type updateTaskReq struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Priority    *string  `json:"priority"`
	DueDate     *string  `json:"due_date"` // "" = temizle
	AssigneeIDs []string `json:"assignee_ids"`
	RowVersion  int      `json:"row_version"`
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var req updateTaskReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	before, err := h.getTask(r.Context(), pid, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Görev bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if !h.canEdit(r.Context(), uid, pid, before) {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Bu görevi düzenleme yetkiniz yok (edit_own yalnızca kendi görevlerinizi kapsar).", nil)
		return
	}

	title := before.Title
	if req.Title != nil {
		var okT bool
		title, okT = ValidateTitle(*req.Title)
		if !okT {
			httpx.ValidationFailed(w, r, map[string]string{"title": "zorunlu, en fazla 300 karakter"})
			return
		}
	}
	priority := before.Priority
	if req.Priority != nil {
		if !ValidPriority(*req.Priority) {
			httpx.ValidationFailed(w, r, map[string]string{"priority": "Low|Normal|High|Urgent"})
			return
		}
		priority = *req.Priority
	}
	desc := before.Description
	if req.Description != nil {
		desc = req.Description
	}
	due := before.DueDate
	if req.DueDate != nil {
		if *req.DueDate == "" {
			due = nil
		} else {
			d, err := time.Parse("2006-01-02", *req.DueDate)
			if err != nil {
				httpx.ValidationFailed(w, r, map[string]string{"due_date": "YYYY-MM-DD biçiminde olmalı"})
				return
			}
			due = &d
			// Termin değişti → hatırlatma sıfırlanır (yeni termine göre tekrar üretilir).
		}
	}

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE tasks SET title=$3, description=$4, priority=$5, due_date=$6,
		       deadline_notified_at = CASE WHEN due_date IS DISTINCT FROM $6::date THEN NULL ELSE deadline_notified_at END,
		       row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND row_version=$7`,
		id, pid, title, desc, priority, due, req.RowVersion)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başka bir kullanıcı tarafından değiştirildi. Sayfayı yenileyin.", nil)
		return
	}

	if req.AssigneeIDs != nil {
		if !h.can(r.Context(), uid, pid, "tasks.assign") {
			httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
				"Atama değişikliği için tasks.assign izni gerekli.", nil)
			return
		}
		if !h.setAssignees(w, r, pid, id, uid, req.AssigneeIDs, false) {
			return
		}
	}

	after, err := h.getTask(r.Context(), pid, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "tasks", EntityID: id.String(),
		Action: audit.ActionUpdate, Before: before, After: after, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"task": after})
}

type moveTaskReq struct {
	Status      string  `json:"status"`
	KanbanOrder float64 `json:"kanban_order"`
	RowVersion  int     `json:"row_version"`
}

// MoveTask — sürükle-bırak: kolon (statü) + kolon içi sıra. Statü değişimi
// workflow_transitions'a yazılır.
func (h *Handler) MoveTask(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	var req moveTaskReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !ValidStatus(req.Status) {
		httpx.ValidationFailed(w, r, map[string]string{"status": "geçersiz statü"})
		return
	}
	before, err := h.getTask(r.Context(), pid, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Görev bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if !h.canEdit(r.Context(), uid, pid, before) {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Bu görevi taşıma yetkiniz yok.", nil)
		return
	}

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE tasks SET status=$3, kanban_order=$4, row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND row_version=$5`,
		id, pid, req.Status, req.KanbanOrder, req.RowVersion)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başka bir kullanıcı tarafından değiştirildi. Panoyu yenileyin.", nil)
		return
	}

	if before.Status != req.Status {
		if _, err := h.pool.Exec(r.Context(), `
			INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
			VALUES ('tasks', $1, $2, $3, $4)`, id, before.Status, req.Status, uid); err != nil {
			h.log.Error("workflow geçişi yazılamadı", "err", err, "task", id)
		}
	}

	after, err := h.getTask(r.Context(), pid, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "tasks", EntityID: id.String(),
		Action: audit.ActionUpdate, Before: before, After: after, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"task": after})
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := h.userID(w, r)
	if !ok {
		return
	}
	before, err := h.getTask(r.Context(), pid, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Görev bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	// Silme: edit_all, ya da (edit_own + oluşturan).
	if !h.can(r.Context(), uid, pid, "tasks.edit_all") &&
		!(h.can(r.Context(), uid, pid, "tasks.edit_own") && before.CreatedBy == uid) {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Bu görevi silme yetkiniz yok.", nil)
		return
	}
	if _, err := h.pool.Exec(r.Context(), `
		UPDATE tasks SET deleted_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "tasks", EntityID: id.String(),
		Action: audit.ActionDelete, Before: before, IP: m.IP, ReqID: m.ReqID,
	})
	w.WriteHeader(http.StatusNoContent)
}
