package tasks

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
)

type commentDTO struct {
	ID         uuid.UUID   `json:"id"`
	TaskID     uuid.UUID   `json:"task_id"`
	AuthorID   uuid.UUID   `json:"author_id"`
	AuthorName string      `json:"author_name"`
	Body       string      `json:"body"`
	Mentions   []uuid.UUID `json:"mentions"`
	CreatedAt  time.Time   `json:"created_at"`
}

// ListComments — GET /projects/{projectID}/tasks/{id}/comments
func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.getTask(r.Context(), pid, id); errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Görev bulunamadı.", nil)
		return
	} else if err != nil {
		httpx.Internal(w, r)
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, c.task_id, c.author_id, u.full_name, c.body, c.mentions, c.created_at
		FROM task_comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.task_id=$1 AND c.deleted_at IS NULL
		ORDER BY c.created_at`, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []commentDTO{}
	for rows.Next() {
		var c commentDTO
		if err := rows.Scan(&c.ID, &c.TaskID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Mentions, &c.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		if c.Mentions == nil {
			c.Mentions = []uuid.UUID{}
		}
		out = append(out, c)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"comments": out})
}

type createCommentReq struct {
	Body string `json:"body"`
}

// CreateComment — POST /projects/{projectID}/tasks/{id}/comments
// @mention'lar proje üyeleri arasında kullanıcı adına göre çözümlenir;
// bahsedilenlere task_mention, (bahsedilmeyen) atananlara task_comment gider.
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
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
	var req createCommentReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	body, okB := ValidateCommentBody(req.Body)
	if !okB {
		httpx.ValidationFailed(w, r, map[string]string{"body": "zorunlu, en fazla 4000 karakter"})
		return
	}
	task, err := h.getTask(r.Context(), pid, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Görev bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	// @mention → proje üyesi kullanıcı id'leri. Üye olmayan adlar sessizce
	// yoksayılır (yanlış yazım bildirim üretmez, yorum yine kaydedilir).
	mentionIDs := []uuid.UUID{}
	if usernames := notify.ParseMentions(body); len(usernames) > 0 {
		rows, err := h.pool.Query(r.Context(), `
			SELECT DISTINCT u.id FROM users u
			JOIN project_members pm ON pm.user_id = u.id AND pm.project_id=$1 AND pm.deleted_at IS NULL
			WHERE u.username = ANY($2) AND u.deleted_at IS NULL AND u.is_active`, pid, usernames)
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		for rows.Next() {
			var u uuid.UUID
			if err := rows.Scan(&u); err != nil {
				rows.Close()
				httpx.Internal(w, r)
				return
			}
			mentionIDs = append(mentionIDs, u)
		}
		rows.Close()
	}

	mentionJSON, _ := json.Marshal(mentionIDs)
	var c commentDTO
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO task_comments (task_id, author_id, body, mentions)
		VALUES ($1,$2,$3,$4::jsonb)
		RETURNING id, task_id, author_id, body, mentions, created_at`,
		id, uid, body, string(mentionJSON)).
		Scan(&c.ID, &c.TaskID, &c.AuthorID, &c.Body, &c.Mentions, &c.CreatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	_ = h.pool.QueryRow(r.Context(), `SELECT full_name FROM users WHERE id=$1`, uid).Scan(&c.AuthorName)
	if c.Mentions == nil {
		c.Mentions = []uuid.UUID{}
	}

	// Bildirimler — yazarın kendisine gitmez; mention alan kişiye tek bildirim
	// (task_mention, task_comment'in yerine geçer).
	mentioned := map[uuid.UUID]bool{}
	mentionTargets := []uuid.UUID{}
	for _, u := range mentionIDs {
		if u != uid {
			mentioned[u] = true
			mentionTargets = append(mentionTargets, u)
		}
	}
	commentTargets := []uuid.UUID{}
	for _, a := range task.Assignees {
		if a.UserID != uid && !mentioned[a.UserID] {
			commentTargets = append(commentTargets, a.UserID)
		}
	}
	if task.CreatedBy != uid && !mentioned[task.CreatedBy] {
		commentTargets = append(commentTargets, task.CreatedBy)
	}
	h.nt.Send(r.Context(), notify.Input{
		UserIDs: mentionTargets, Type: notify.TypeTaskMention,
		Title: "Bir yorumda sizden bahsedildi: " + task.Title,
		Body:  c.AuthorName + ": " + truncate(body, 200),
		EntityType: "tasks", EntityID: &id, ProjectID: &pid,
	})
	h.nt.Send(r.Context(), notify.Input{
		UserIDs: commentTargets, Type: notify.TypeTaskComment,
		Title: "Göreve yeni yorum: " + task.Title,
		Body:  c.AuthorName + ": " + truncate(body, 200),
		EntityType: "tasks", EntityID: &id, ProjectID: &pid,
	})

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "task_comments", EntityID: c.ID.String(),
		Action: audit.ActionInsert, After: c, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"comment": c})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
