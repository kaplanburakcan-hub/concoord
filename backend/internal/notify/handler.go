package notify

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// Handler — kullanıcının KENDİ bildirimleri ve kanal tercihleri. Ayrı izin
// gerekmez: veri zaten kişiye özel filtrelenir (user_id = oturum sahibi).
type Handler struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, log *slog.Logger) *Handler {
	return &Handler{pool: pool, log: log}
}

type notificationDTO struct {
	ID         uuid.UUID  `json:"id"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Body       *string    `json:"body,omitempty"`
	EntityType *string    `json:"entity_type,omitempty"`
	EntityID   *uuid.UUID `json:"entity_id,omitempty"`
	ProjectID  *uuid.UUID `json:"project_id,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// List — GET /notifications?limit=&unread=1 (yalnızca InApp kanalı; e-posta/SMS
// kopyaları zil listesinde tekrarlanmaz) + okunmamış sayacı.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	limit := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	onlyUnread := r.URL.Query().Get("unread") == "1"

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, type, title, body, entity_type, entity_id, project_id, read_at, created_at
		FROM notifications
		WHERE user_id=$1 AND channel='InApp' AND deleted_at IS NULL
		  AND ($2 = false OR read_at IS NULL)
		ORDER BY created_at DESC
		LIMIT $3`, uid, onlyUnread, limit)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []notificationDTO{}
	for rows.Next() {
		var n notificationDTO
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.EntityType,
			&n.EntityID, &n.ProjectID, &n.ReadAt, &n.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, n)
	}

	var unread int
	if err := h.pool.QueryRow(r.Context(), `
		SELECT count(*) FROM notifications
		WHERE user_id=$1 AND channel='InApp' AND deleted_at IS NULL AND read_at IS NULL`,
		uid).Scan(&unread); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"notifications": out,
		"unread_count":  unread,
	})
}

// MarkRead — POST /notifications/{id}/read
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE notifications SET read_at=now()
		WHERE id=$1 AND user_id=$2 AND read_at IS NULL AND deleted_at IS NULL`, id, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		// Zaten okundu ya da bu kullanıcıya ait değil — idempotent 204.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead — POST /notifications/read-all
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	if _, err := h.pool.Exec(r.Context(), `
		UPDATE notifications SET read_at=now()
		WHERE user_id=$1 AND channel='InApp' AND read_at IS NULL AND deleted_at IS NULL`, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPreferences — GET /notification-preferences (varsayılanlarla birleşik)
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	svc := &Service{pool: h.pool, log: h.log}
	prefs, err := svc.preferences(r.Context(), uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"preferences": prefs})
}

type prefReq struct {
	Channel string `json:"channel"`
	Enabled *bool  `json:"enabled"`
}

// SetPreference — PUT /notification-preferences
func (h *Handler) SetPreference(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	var req prefReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Enabled == nil ||
		(req.Channel != ChannelInApp && req.Channel != ChannelEmail && req.Channel != ChannelSMS) {
		httpx.ValidationFailed(w, r, map[string]string{
			"channel": "InApp | Email | SMS olmalı",
			"enabled": "zorunlu",
		})
		return
	}
	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO notification_preferences (user_id, channel, enabled)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id, channel) DO UPDATE SET enabled=$3, updated_at=now()`,
		uid, req.Channel, *req.Enabled); err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
