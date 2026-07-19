package payments

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/rbac"
)

// Handler — Faz 3 taşeron + hakediş uçları. Faz 2 projects/documents deseniyle
// birebir aynı: audit'li yazımlar, optimistic locking, proje kapsamlı izin.
type Handler struct {
	pool *pgxpool.Pool
	eval *rbac.Evaluator
	rec  *audit.Recorder
	log  *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder, log *slog.Logger) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, log: log}
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

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func isUniqueViolation(err error) bool { return sqlState(err) == "23505" }
func isFKViolation(err error) bool     { return sqlState(err) == "23503" }

// isLockViolation — Finalized kilit trigger'ının (restrict_violation, 23001)
// yükselttiği hata. Kullanıcıya 409 döneriz.
func isLockViolation(err error) bool { return sqlState(err) == "23001" }

func sqlState(err error) string {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState()
	}
	return ""
}

// canFin — kullanıcı bu projede finansal kolonları (birim fiyat/tutar) görebilir mi?
// (Plan §4: view ≠ view_financials.) Görünürlük hakediş, sözleşme, birim fiyat
// cetveli ve dashboard genelinde tutarlı uygulanır.
func (h *Handler) canFin(r *http.Request, pid uuid.UUID) bool {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		return false
	}
	allowed, err := h.eval.Can(r.Context(), uid, &pid, "progress_payments.view_financials")
	return err == nil && allowed
}

// scopedSub — satır seviyesi güvenlik (Plan §4/§86): kullanıcı bu projede bir
// taşerona bağlıysa (SubcontractorRep) o taşeronun id'si döner; aksi halde nil
// (kısıtsız = PM/Admin/SiteEngineer). Filtre backend'de ZORUNLU uygulanır.
func (h *Handler) scopedSub(ctx context.Context, uid, pid uuid.UUID) (*uuid.UUID, error) {
	var sub *uuid.UUID
	err := h.pool.QueryRow(ctx, `
		SELECT subcontractor_id FROM project_members
		WHERE user_id=$1 AND project_id=$2 AND deleted_at IS NULL
		  AND subcontractor_id IS NOT NULL
		LIMIT 1`, uid, pid).Scan(&sub)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// requireScope — istek için (uid, scopedSub) çözer; hata durumunda yanıtı yazıp
// false döner. subID nil ise kullanıcı kısıtsızdır.
func (h *Handler) requireScope(w http.ResponseWriter, r *http.Request, pid uuid.UUID) (uuid.UUID, *uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, nil, false
	}
	sub, err := h.scopedSub(r.Context(), uid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return uuid.Nil, nil, false
	}
	return uid, sub, true
}

// parseDatePtr — "YYYY-MM-DD" biçimli opsiyonel tarihi *time.Time'a çevirir.
// Boş/geçersiz girdide nil döner (dönem kontrolleri tarih yoksa uygulanmaz).
func parseDatePtr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil
	}
	return &t
}

// nullIfEmpty — boş dizeyi SQL NULL'a çevirir (opsiyonel metin kolonları için).
func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
