package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/rbac"
)

type Handler struct {
	svc  *Service
	eval *rbac.Evaluator
	log  *slog.Logger
	dev  bool // development'ta reset jetonunu yanıt/logda göster
}

func NewHandler(svc *Service, eval *rbac.Evaluator, log *slog.Logger, dev bool) *Handler {
	return &Handler{svc: svc, eval: eval, log: log, dev: dev}
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return v
	}
	return r.RemoteAddr
}

type loginReq struct {
	Identifier string `json:"identifier"` // e-posta veya kullanıcı adı
	Password   string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Identifier = strings.TrimSpace(req.Identifier)
	if req.Identifier == "" || req.Password == "" {
		httpx.ValidationFailed(w, r, map[string]string{"identifier": "zorunlu", "password": "zorunlu"})
		return
	}
	user, pair, err := h.svc.Login(r.Context(), req.Identifier, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrUserInactive):
			httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "E-posta/kullanıcı adı veya parola hatalı.", nil)
		default:
			h.log.Error("login hatası", "err", err)
			httpx.Internal(w, r)
		}
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"user": user, "tokens": pair})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.RefreshToken == "" {
		httpx.ValidationFailed(w, r, map[string]string{"refresh_token": "zorunlu"})
		return
	}
	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken, r.UserAgent(), clientIP(r))
	if err != nil {
		if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrUserInactive) {
			httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum yenilenemedi.", nil)
			return
		}
		h.log.Error("refresh hatası", "err", err)
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"tokens": pair})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.RefreshToken != "" {
		_ = h.svc.Logout(r.Context(), req.RefreshToken)
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type forgotReq struct {
	Email string `json:"email"`
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	raw, err := h.svc.ForgotPassword(r.Context(), strings.TrimSpace(req.Email))
	if err != nil {
		h.log.Error("forgot-password hatası", "err", err)
		httpx.Internal(w, r)
		return
	}
	// Faz 4 bildirim motoruna kadar: jeton geliştirmede loglanır/yanıtlanır.
	resp := map[string]interface{}{"status": "ok"}
	if raw != "" {
		h.log.Info("şifre sıfırlama jetonu üretildi (Faz 4'e kadar e-posta yerine log)",
			"email", req.Email)
		if h.dev {
			resp["reset_token"] = raw // yalnızca development
		}
	}
	httpx.JSON(w, http.StatusOK, resp)
}

type resetReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Token == "" || len(req.NewPassword) < 8 {
		httpx.ValidationFailed(w, r, map[string]string{"new_password": "en az 8 karakter", "token": "zorunlu"})
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		if errors.Is(err, ErrInvalidToken) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Sıfırlama jetonu geçersiz veya süresi dolmuş.", nil)
			return
		}
		h.log.Error("reset-password hatası", "err", err)
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type changeReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	var req changeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.CurrentPassword == "" || len(req.NewPassword) < 8 {
		httpx.ValidationFailed(w, r, map[string]string{"new_password": "en az 8 karakter", "current_password": "zorunlu"})
		return
	}
	if err := h.svc.ChangePassword(r.Context(), uid, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Mevcut parola hatalı.", nil)
			return
		}
		h.log.Error("change-password hatası", "err", err)
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me — kimliği doğrulanmış kullanıcı + (opsiyonel proje kapsamında) etkin
// izinleri. Frontend <Can> bu izin listesini tüketir.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	user, err := h.svc.GetUser(r.Context(), uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	projectID := ProjectFromRequest(r)
	perms, err := h.eval.EffectivePermissions(r.Context(), uid, projectID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if perms == nil {
		perms = []string{}
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"user":        user,
		"permissions": perms,
		"project_id":  projectID,
	})
}
