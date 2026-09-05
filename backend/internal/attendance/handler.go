// Package attendance — PDKS/GPS Puantaj (Blok 2, Aşama 2).
//
// Bu, mevcut manuel haftalık puantaj sisteminden (internal/personnel,
// project_puantaj tablosu) BİLİNÇLİ olarak AYRI bir sistemdir — ikisi de
// project_personnel'i paylaşır ama birbirinin yerini almaz (kullanıcı
// kararı). Konum SADECE giriş/çıkış anında, tek atış olarak alınır;
// watchPosition veya arka plan konumu YOK. Tarayıcı tabanlı çözüm sahte
// konumu (mock location) tespit EDEMEZ — bu paketin hiçbir yerinde "GPS ile
// kesin doğrulama" gibi bir iddia yoktur; kayıt konum/zaman/cihaz kimliğiyle
// saklanır ve şantiye şefi onayına sunulur.
package attendance

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
)

type Handler struct {
	pool   *pgxpool.Pool
	rec    *audit.Recorder
	notify *notify.Service
	eval   *rbac.Evaluator
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder, notifySvc *notify.Service, eval *rbac.Evaluator) *Handler {
	return &Handler{pool: pool, rec: rec, notify: notifySvc, eval: eval}
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", map[string]string{key: "geçersiz UUID"})
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
