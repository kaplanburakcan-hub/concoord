package attendance

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// GET /api/v1/attendance/personnel?token=... — herkese açık (PdksCheckinPage
// kimlik doğrulaması gerektirmez, bkz. events.go üstündeki not). Aynı QR
// token güvenlik modeliyle korunur: token GEÇERLİ (süresi dolmamış) olmalı.
// Yalnızca ad/görev döner — firma, sıra gibi başka alan sızdırılmaz.
// ---------------------------------------------------------------------------

type personnelOptionDTO struct {
	ID      uuid.UUID `json:"id"`
	AdSoyad string    `json:"ad_soyad"`
	Gorev   string    `json:"gorev"`
}

func (h *Handler) ListPersonnelByToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.ValidationFailed(w, r, map[string]string{"token": "Zorunlu."})
		return
	}

	var geofenceID uuid.UUID
	var expiresAt time.Time
	if err := h.pool.QueryRow(r.Context(),
		`SELECT geofence_id, expires_at FROM attendance_qr_tokens WHERE token=$1`, token,
	).Scan(&geofenceID, &expiresAt); err != nil || time.Now().UTC().After(expiresAt.Add(clockSkewGrace)) {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kod geçersiz veya süresi dolmuş.", nil)
		return
	}

	var projectID uuid.UUID
	if err := h.pool.QueryRow(r.Context(),
		`SELECT project_id FROM site_geofences WHERE id=$1`, geofenceID,
	).Scan(&projectID); err != nil {
		httpx.Internal(w, r)
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, ad_soyad, gorev FROM project_personnel
		WHERE project_id=$1 AND is_aktif=true
		ORDER BY sira, ad_soyad`, projectID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []personnelOptionDTO{}
	for rows.Next() {
		var p personnelOptionDTO
		if err := rows.Scan(&p.ID, &p.AdSoyad, &p.Gorev); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"personnel": out})
}
