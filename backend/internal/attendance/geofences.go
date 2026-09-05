package attendance

import (
	"crypto/rand"
	"encoding/base64"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// Şantiye sınırı (geofence)
// ---------------------------------------------------------------------------

type geofenceDTO struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name      string    `json:"name"`
	CenterLat float64   `json:"center_lat"`
	CenterLng float64   `json:"center_lng"`
	RadiusM   int       `json:"radius_m"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

const geofenceCols = `id, project_id, name, center_lat, center_lng, radius_m, is_active, created_at`

func (h *Handler) ListGeofences(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+geofenceCols+` FROM site_geofences
		WHERE project_id=$1 ORDER BY name`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []geofenceDTO{}
	for rows.Next() {
		var g geofenceDTO
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Name, &g.CenterLat, &g.CenterLng, &g.RadiusM, &g.IsActive, &g.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, g)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"geofences": out})
}

type geofenceReq struct {
	Name      string  `json:"name"`
	CenterLat float64 `json:"center_lat"`
	CenterLng float64 `json:"center_lng"`
	RadiusM   int     `json:"radius_m"`
}

func (h *Handler) CreateGeofence(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req geofenceReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	f := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		f["name"] = "Zorunlu."
	}
	if req.CenterLat < -90 || req.CenterLat > 90 {
		f["center_lat"] = "Geçerli bir enlem girin (-90..90)."
	}
	if req.CenterLng < -180 || req.CenterLng > 180 {
		f["center_lng"] = "Geçerli bir boylam girin (-180..180)."
	}
	if req.RadiusM <= 0 {
		req.RadiusM = 300
	}
	if len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}

	var g geofenceDTO
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO site_geofences (project_id, name, center_lat, center_lng, radius_m)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+geofenceCols,
		pid, strings.TrimSpace(req.Name), req.CenterLat, req.CenterLng, req.RadiusM,
	).Scan(&g.ID, &g.ProjectID, &g.Name, &g.CenterLat, &g.CenterLng, &g.RadiusM, &g.IsActive, &g.CreatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: uid.String(), Entity: "site_geofences", EntityID: g.ID.String(), Action: audit.ActionInsert,
		After: map[string]any{"name": g.Name, "radius_m": g.RadiusM}, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]any{"geofence": g})
}

// ---------------------------------------------------------------------------
// QR token — 60 saniyelik, tek kullanımlık. Statik QR ÜRETİLMEZ: her istek
// yeni, rastgele bir token döner; panonun ekranı bunu 60 saniyede bir yeniden
// çeker (frontend sorumluluğu).
// ---------------------------------------------------------------------------

const qrTokenTTL = 60 * time.Second

type qrTokenDTO struct {
	Token      string    `json:"token"`
	GeofenceID uuid.UUID `json:"geofence_id"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func newTokenString() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (h *Handler) IssueQRToken(w http.ResponseWriter, r *http.Request) {
	gid, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var isActive bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_active FROM site_geofences WHERE id=$1`, gid,
	).Scan(&isActive); err != nil {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Şantiye sınırı bulunamadı.", nil)
		return
	}
	if !isActive {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu şantiye sınırı devre dışı.", nil)
		return
	}

	token, err := newTokenString()
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	now := time.Now().UTC()
	expires := now.Add(qrTokenTTL)
	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO attendance_qr_tokens (token, geofence_id, issued_at, expires_at)
		VALUES ($1,$2,$3,$4)`, token, gid, now, expires); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Ev sahipliği: 1 saatten eski token'ları fırsatçı şekilde temizle (yalnızca
	// hijyen — güvenlik expires_at kontrolüne dayanır, bu silme işine değil).
	_, _ = h.pool.Exec(r.Context(),
		`DELETE FROM attendance_qr_tokens WHERE expires_at < now() - interval '1 hour'`)

	httpx.JSON(w, http.StatusOK, map[string]any{"qr_token": qrTokenDTO{
		Token: token, GeofenceID: gid, IssuedAt: now, ExpiresAt: expires,
	}})
}

// ---------------------------------------------------------------------------
// Haversine — iki koordinat arası büyük daire mesafesi (metre).
// ---------------------------------------------------------------------------

const earthRadiusM = 6371000.0

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}
