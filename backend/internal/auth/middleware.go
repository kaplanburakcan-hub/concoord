package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/rbac"
)

type ctxKey int

const ctxKeyUserID ctxKey = iota

// UserIDFrom — kimliği doğrulanmış kullanıcının id'si (Authenticate sonrası).
func UserIDFrom(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return v, ok
}

// Middleware — auth + yetki middleware üreticileri.
type Middleware struct {
	signer *TokenSigner
	eval   *rbac.Evaluator
}

func NewMiddleware(signer *TokenSigner, eval *rbac.Evaluator) *Middleware {
	return &Middleware{signer: signer, eval: eval}
}

// Authenticate — Bearer access token'ı doğrular, aktörü context'e yazar.
// Başarısızsa 401 döner. Audit meta aktörü de böylece dolar (httpx.WithActorID).
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		claims, err := m.signer.Parse(token)
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum jetonu geçersiz veya süresi dolmuş.", nil)
			return
		}
		uid, err := uuid.Parse(claims.Sub)
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Jeton öznesi geçersiz.", nil)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUserID, uid)
		ctx = httpx.WithActorID(ctx, uid.String()) // audit ve loglar için aktör
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission — verilen izin kodunu zorunlu kılar. Proje kapsamı isteğe
// bağlıdır: önce URL parametresi {projectID}, sonra X-Project-Id başlığı, sonra
// project_id sorgu parametresi denenir; hiçbiri yoksa global kontrol yapılır.
// Karar motoru DENY > GRANT > rol önceliğini uygular (Plan §4).
func (m *Middleware) RequirePermission(permCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, ok := UserIDFrom(r.Context())
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
				return
			}
			projectID := ProjectFromRequest(r)
			allowed, err := m.eval.Can(r.Context(), uid, projectID, permCode)
			if err != nil {
				httpx.Internal(w, r)
				return
			}
			if !allowed {
				httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
					"Bu işlem için yetkiniz yok.", map[string]string{"required": permCode})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ProjectFromRequest — isteğin proje kapsamını çözer (URL > başlık > sorgu).
func ProjectFromRequest(r *http.Request) *uuid.UUID {
	candidates := []string{
		chi.URLParam(r, "projectID"),
		r.Header.Get("X-Project-Id"),
		r.URL.Query().Get("project_id"),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if id, err := uuid.Parse(c); err == nil {
			return &id
		}
	}
	return nil
}
