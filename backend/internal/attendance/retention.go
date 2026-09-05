package attendance

import (
	"net/http"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// PurgeStaleLocations — KVKK saklama süresi işi. Dışarıdan (Render Cron Job,
// aynı desen bkz. internal/machines/rental_reminder.go) günlük tetiklenmesi
// beklenir; oturum GEREKTİRMEZ, /internal/cron altında cronSecretGuard ile
// korunur (server.go). attendance_retention_settings'teki (varsayılan 730
// gün) süreyi geçen attendance_events kayıtlarında lat/lng/accuracy_m/
// device_id alanlarını NULL'a çeker. attendance_days (türetilmiş saatler)
// KESİNLİKLE ETKİLENMEZ — yalnızca ham konum silinir, çalışma süresi kaydı
// (derived_hours/adjusted_hours) kalıcı kalır.
func (h *Handler) PurgeStaleLocations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var retentionDays int
	if err := h.pool.QueryRow(ctx,
		`SELECT retention_days FROM attendance_retention_settings WHERE id = true`,
	).Scan(&retentionDays); err != nil {
		httpx.Internal(w, r)
		return
	}

	rows, err := h.pool.Query(ctx, `
		UPDATE attendance_events
		SET lat = NULL, lng = NULL, accuracy_m = NULL, device_id = NULL
		WHERE captured_at < now() - make_interval(days => $1)
		  AND (lat IS NOT NULL OR lng IS NOT NULL OR accuracy_m IS NOT NULL OR device_id IS NOT NULL)
		RETURNING id`, retentionDays)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()

	if count > 0 {
		meta := audit.MetaFrom(ctx)
		h.rec.Record(ctx, audit.Entry{
			// Bu gerçekten bir UPDATE'tir (lat/lng/accuracy_m/device_id NULL'a
			// çekilir) — audit_logs.action CHECK kısıtına yeni bir değer
			// eklemeye gerek yok.
			Entity: "attendance_events", Action: audit.ActionUpdate,
			After: map[string]any{"purged_count": count, "retention_days": retentionDays},
			IP:    meta.IP, ReqID: meta.ReqID,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"purged": count, "retention_days": retentionDays})
}
