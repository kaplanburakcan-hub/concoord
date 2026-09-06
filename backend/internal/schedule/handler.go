// Package schedule — İş Programı (WBS + Gantt), Blok 2 Aşama 1.
//
// Fiziksel ilerleme elle girilmez, onaylı (Finalized) hakedişten türetilir.
// İlerleme hesabı (progress.go) internal/payments/calc.go ile AYNI ilkeyi
// izler: saf, DB'siz bir fonksiyon (ComputeProgress) — istek anında çağrılır,
// materialized view veya cache YOKTUR (Plan gereği). Handler yalnızca
// girdiyi DB'den toplar, sonucu döner.
package schedule

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder) *Handler {
	return &Handler{pool: pool, rec: rec}
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
