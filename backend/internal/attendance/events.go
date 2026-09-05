package attendance

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// POST /api/v1/attendance/events — kimlik doğrulaması GEREKTİRMEZ (bkz.
// server.go route kaydı). Güvenlik, işçinin oturumundan değil, taşınan
// QR token'ın kendisinden gelir: token 60 saniyelik ve tek geofence'e
// bağlıdır, sunucu her olayın captured_at'ini token'ın geçerlilik
// penceresiyle (issued_at..expires_at) karşılaştırır — SUNUCUNUN O ANKİ
// saatiyle değil. Bu bilinçli bir tasarım kararı: uçak modunda kuyruğa
// alınan olaylar bağlantı geri geldiğinde geç senkronize olur (dakikalar/
// saatler sonra) — token'ı "isteğin geldiği an" ile karşılaştırmak bu
// senaryoyu (kabul kriteri 8) imkansız kılardı. Kötüye kullanılan/çok eski
// bir token yine de reddedilir çünkü olayın captured_at'i token'ın kendi
// 60 saniyelik penceresinin dışında kalır.
//
// 'manual'/'import' kaynakları şema düzeyinde ayrılmıştır (bkz. migration
// 000060) ama bu ADIMDA hiçbir uç bunları üretmiyor — PuantajPage yalnızca
// türetilmiş saatleri DÜZELTİYOR (PATCH .../days/{id}), ham bir "manuel
// olay" oluşturmuyor. attendance.record izni sözlükte tanımlı, gelecekte
// böyle bir uç eklenirse kullanılacak.
// ---------------------------------------------------------------------------

type eventReq struct {
	ClientUUID string   `json:"client_uuid"`
	QRToken    string   `json:"qr_token"`
	PersonID   string   `json:"person_id"`
	EventType  string   `json:"event_type"`
	CapturedAt string   `json:"captured_at"`
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	AccuracyM  *float64 `json:"accuracy_m"`
	DeviceID   string   `json:"device_id"`
	Note       string   `json:"note"`
}

type eventResult struct {
	ClientUUID string   `json:"client_uuid"`
	OK         bool     `json:"ok"`
	Duplicate  bool     `json:"duplicate,omitempty"`
	Error      string   `json:"error,omitempty"`
	GeofenceOK *bool    `json:"geofence_ok,omitempty"`
	DistanceM  *float64 `json:"distance_m,omitempty"`
}

// tokenExpiredOrMissing — sonucun 401'e mi yoksa 200 (kısmi başarı)'ye mi
// karşılık geleceğine karar vermek için kullanılır.
const (
	errBadPayload   = "gecersiz_alan"
	errBadToken     = "gecersiz_kod"
	errTokenWindow  = "kod_suresi_gecersiz"
	errPersonNotFnd = "personel_bulunamadi"
)

const clockSkewGrace = 5 * time.Second

func (h *Handler) CreateEvents(w http.ResponseWriter, r *http.Request) {
	var reqs []eventReq
	if !httpx.DecodeJSON(w, r, &reqs) {
		return
	}
	if len(reqs) == 0 {
		httpx.ValidationFailed(w, r, map[string]string{"_": "En az bir olay gerekli."})
		return
	}

	type touched struct {
		projectID uuid.UUID
		personID  uuid.UUID
	}
	seenTouched := map[touched]bool{}
	var touchedList []touched

	results := make([]eventResult, len(reqs))
	tokenFailures := 0

	for i, req := range reqs {
		res := &results[i]
		res.ClientUUID = req.ClientUUID

		clientUUID, err1 := uuid.Parse(req.ClientUUID)
		personID, err2 := uuid.Parse(req.PersonID)
		capturedAt, err3 := time.Parse(time.RFC3339, req.CapturedAt)
		if err1 != nil || err2 != nil || err3 != nil || (req.EventType != "in" && req.EventType != "out") {
			res.Error = errBadPayload
			continue
		}

		// Idempotency: aynı client_uuid daha önce işlendiyse mevcut satırı döndür,
		// yeniden işleme.
		var existingID uuid.UUID
		var existingGeofenceOK *bool
		var existingDistance *float64
		err := h.pool.QueryRow(r.Context(),
			`SELECT id, geofence_ok, distance_m::float8 FROM attendance_events WHERE client_uuid=$1`,
			clientUUID,
		).Scan(&existingID, &existingGeofenceOK, &existingDistance)
		if err == nil {
			res.OK = true
			res.Duplicate = true
			res.GeofenceOK = existingGeofenceOK
			res.DistanceM = existingDistance
			continue
		}
		if err != pgx.ErrNoRows {
			httpx.Internal(w, r)
			return
		}

		var geofenceID uuid.UUID
		var issuedAt, expiresAt time.Time
		err = h.pool.QueryRow(r.Context(),
			`SELECT geofence_id, issued_at, expires_at FROM attendance_qr_tokens WHERE token=$1`,
			req.QRToken,
		).Scan(&geofenceID, &issuedAt, &expiresAt)
		if err != nil {
			res.Error = errBadToken
			tokenFailures++
			continue
		}
		if !withinTokenWindow(capturedAt, issuedAt, expiresAt) {
			res.Error = errTokenWindow
			tokenFailures++
			continue
		}

		var projectID uuid.UUID
		var centerLat, centerLng float64
		var radiusM int
		if err := h.pool.QueryRow(r.Context(),
			`SELECT project_id, center_lat, center_lng, radius_m FROM site_geofences WHERE id=$1`,
			geofenceID,
		).Scan(&projectID, &centerLat, &centerLng, &radiusM); err != nil {
			res.Error = errBadToken
			tokenFailures++
			continue
		}

		var personExists bool
		if err := h.pool.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM project_personnel WHERE id=$1 AND project_id=$2)`,
			personID, projectID,
		).Scan(&personExists); err != nil {
			httpx.Internal(w, r)
			return
		}
		if !personExists {
			res.Error = errPersonNotFnd
			continue
		}

		var distanceM *float64
		if req.Lat != nil && req.Lng != nil {
			d := haversineMeters(*req.Lat, *req.Lng, centerLat, centerLng)
			distanceM = &d
		}
		geofenceOK := evaluateGeofence(distanceM, radiusM, req.AccuracyM)

		var newID uuid.UUID
		err = h.pool.QueryRow(r.Context(), `
			INSERT INTO attendance_events
				(client_uuid, project_id, geofence_id, person_id, event_type, source,
				 lat, lng, accuracy_m, distance_m, geofence_ok, captured_at, device_id, note)
			VALUES ($1,$2,$3,$4,$5,'qr',$6,$7,$8,$9,$10,$11,NULLIF($12,''),NULLIF($13,''))
			ON CONFLICT (client_uuid) DO NOTHING
			RETURNING id`,
			clientUUID, projectID, geofenceID, personID, req.EventType,
			req.Lat, req.Lng, req.AccuracyM, distanceM, geofenceOK, capturedAt, req.DeviceID, req.Note,
		).Scan(&newID)
		if err != nil {
			if err == pgx.ErrNoRows {
				// Eşzamanlı bir istek arada aynı client_uuid'i işledi — idempotent kabul et.
				res.OK = true
				res.Duplicate = true
				continue
			}
			httpx.Internal(w, r)
			return
		}

		res.OK = true
		res.GeofenceOK = &geofenceOK
		res.DistanceM = distanceM

		m := audit.MetaFrom(r.Context())
		h.rec.Record(r.Context(), audit.Entry{
			ActorID: m.ActorID, Entity: "attendance_events", EntityID: newID.String(), Action: audit.ActionInsert,
			After: map[string]any{"person_id": personID, "event_type": req.EventType, "geofence_ok": geofenceOK},
			IP:    m.IP, ReqID: m.ReqID,
		})

		key := touched{projectID: projectID, personID: personID}
		if !seenTouched[key] {
			seenTouched[key] = true
			touchedList = append(touchedList, key)
		}
	}

	for _, t := range touchedList {
		if err := h.recomputeDays(r.Context(), t.projectID, t.personID); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	status := http.StatusOK
	if tokenFailures == len(reqs) {
		status = http.StatusUnauthorized
	}
	httpx.JSON(w, status, map[string]any{"results": results})
}

// recomputeDays — bir kişinin, olay eklenen tüm günleri için attendance_days'i
// yeniden hesaplar. Ham attendance_events kayıtlarına DOKUNMAZ, yalnızca
// türetilmiş satırı (derived_hours) günceller — 'approved' durumundaki
// günler kilitlidir, üzerine yazılmaz.
// Not: work_date her zaman Go string ("YYYY-MM-DD") olarak taşınır, asla
// time.Time olarak değil — pgx bir time.Time parametresini varsayılan olarak
// timestamptz kodlar, sunucu bunu $N::date'e cast ederken SESSION saat
// dilimine göre yeniden yorumlar ve gün kayması riski doğurur. Düz metin
// ("2026-08-30") ise text→date olarak doğrudan, hiçbir saat dilimi
// belirsizliği olmadan ayrıştırılır.
func (h *Handler) recomputeDays(ctx context.Context, projectID, personID uuid.UUID) error {
	rows, err := h.pool.Query(ctx,
		`SELECT DISTINCT to_char((captured_at AT TIME ZONE 'Europe/Istanbul')::date, 'YYYY-MM-DD')
		 FROM attendance_events WHERE project_id=$1 AND person_id=$2`,
		projectID, personID)
	if err != nil {
		return err
	}
	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			rows.Close()
			return err
		}
		dates = append(dates, d)
	}
	rows.Close()

	for _, workDate := range dates {
		if err := h.recomputeOneDay(ctx, projectID, personID, workDate); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) recomputeOneDay(ctx context.Context, projectID, personID uuid.UUID, workDate string) error {
	rows, err := h.pool.Query(ctx, `
		SELECT event_type, captured_at FROM attendance_events
		WHERE project_id=$1 AND person_id=$2
		  AND (captured_at AT TIME ZONE 'Europe/Istanbul')::date = $3::date
		ORDER BY captured_at`, projectID, personID, workDate)
	if err != nil {
		return err
	}
	var events []rawEvent
	for rows.Next() {
		var e rawEvent
		if err := rows.Scan(&e.Type, &e.At); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	rows.Close()

	derivedHours, _ := deriveDayHours(events)

	_, err = h.pool.Exec(ctx, `
		INSERT INTO attendance_days (project_id, person_id, work_date, derived_hours, status)
		VALUES ($1,$2,$3,$4,'derived')
		ON CONFLICT (project_id, person_id, work_date) DO UPDATE SET
			derived_hours = EXCLUDED.derived_hours,
			updated_at = now()
		WHERE attendance_days.status <> 'approved'`,
		projectID, personID, workDate, derivedHours)
	return err
}

// rawEvent — bir günün ham giriş/çıkış olayı (deriveDayHours'a girdi).
type rawEvent struct {
	Type string
	At   time.Time
}

// deriveDayHours — bir günün ham giriş/çıkış olaylarından toplam çalışma
// saatini türetir (saf fonksiyon, DB'siz test edilebilir). Sıralama bozuksa
// (iki ardışık "in", sahipsiz "out", ya da kapanmamış son "in") malformed
// true döner ve hours nil kalır — ham kayıt korunur, yalnızca türetilmiş
// değer hesaplanamaz; şef ekranında bu "boş hücre" olarak görünüp incelemeyi
// işaret eder (kabul kriteri 6 ile aynı ruhta: reddetme, işaretle).
func deriveDayHours(events []rawEvent) (hours *float64, malformed bool) {
	var totalMinutes float64
	var pendingIn *time.Time
	for _, e := range events {
		switch e.Type {
		case "in":
			if pendingIn != nil {
				malformed = true
			}
			t := e.At
			pendingIn = &t
		case "out":
			if pendingIn == nil {
				malformed = true
				continue
			}
			totalMinutes += e.At.Sub(*pendingIn).Minutes()
			pendingIn = nil
		}
	}
	if pendingIn != nil {
		malformed = true
	}
	if !malformed && len(events) > 0 {
		h := totalMinutes / 60
		hours = &h
	}
	return hours, malformed
}

// withinTokenWindow — kabul kriteri 8 (uçak modu senkronizasyonu) ile
// kabul kriteri "eski/kötüye kullanılan token reddedilir" arasındaki
// gerilimi çözer: captured_at, sunucunun O ANKİ saatiyle değil, token'ın
// KENDİ geçerlilik penceresiyle (issued_at..expires_at) karşılaştırılır —
// ±clockSkewGrace toleransıyla (kiosk ve işçi telefonu farklı cihazlar,
// küçük saat kayması normaldir).
func withinTokenWindow(capturedAt, issuedAt, expiresAt time.Time) bool {
	return !capturedAt.Before(issuedAt.Add(-clockSkewGrace)) && !capturedAt.After(expiresAt.Add(clockSkewGrace))
}

// evaluateGeofence — kabul kriteri 6: geofence dışından gelen kayıt
// REDDEDİLMEZ, yalnızca bu fonksiyonun döndürdüğü false ile işaretlenir.
// Çağıran taraf satırı HER KOŞULDA ekler, yalnızca geofence_ok alanını bu
// sonuca göre doldurur.
func evaluateGeofence(distanceM *float64, radiusM int, accuracyM *float64) bool {
	if distanceM == nil {
		return false
	}
	if *distanceM > float64(radiusM) {
		return false
	}
	if accuracyM != nil && *accuracyM > 100 {
		return false
	}
	return true
}
