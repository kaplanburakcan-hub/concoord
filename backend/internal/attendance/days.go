package attendance

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// Günlük türetilmiş puantaj
// ---------------------------------------------------------------------------

type dayDTO struct {
	ID             uuid.UUID  `json:"id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	PersonID       uuid.UUID  `json:"person_id"`
	PersonName     string     `json:"person_name"`
	WorkDate       string     `json:"work_date"`
	DerivedHours   *float64   `json:"derived_hours"`
	AdjustedHours  *float64   `json:"adjusted_hours"`
	OvertimeHours  float64    `json:"overtime_hours"`
	Status         string     `json:"status"`
	AdjustedReason *string    `json:"adjusted_reason,omitempty"`
	ApprovedAt     *time.Time `json:"approved_at,omitempty"`
	// HasFlag — o gün içinde geofence_ok=false işaretli en az bir olay var mı.
	// Ham koordinat İÇERMEZ, yalnızca bir uyarı bayrağıdır; bu yüzden
	// attendance.view_location izni gerektirmez.
	HasFlag bool `json:"has_flag"`
}

func (h *Handler) ListDays(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	q := r.URL.Query()
	var from, to, personID *string
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		from = &v
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		to = &v
	}
	if v := strings.TrimSpace(q.Get("person_id")); v != "" {
		if _, err := uuid.Parse(v); err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"person_id": "geçersiz UUID"})
			return
		}
		personID = &v
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT d.id, d.project_id, d.person_id, p.ad_soyad, to_char(d.work_date,'YYYY-MM-DD'),
		       d.derived_hours::float8, d.adjusted_hours::float8, d.overtime_hours::float8,
		       d.status, d.adjusted_reason, d.approved_at,
		       EXISTS(
		         SELECT 1 FROM attendance_events e
		         WHERE e.project_id = d.project_id AND e.person_id = d.person_id
		           AND (e.captured_at AT TIME ZONE 'Europe/Istanbul')::date = d.work_date
		           AND e.geofence_ok = false
		       ) AS has_flag
		FROM attendance_days d
		JOIN project_personnel p ON p.id = d.person_id
		WHERE d.project_id = $1
		  AND ($2::date IS NULL OR d.work_date >= $2::date)
		  AND ($3::date IS NULL OR d.work_date <= $3::date)
		  AND ($4::uuid IS NULL OR d.person_id = $4::uuid)
		ORDER BY d.work_date DESC, p.ad_soyad`,
		pid, from, to, personID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []dayDTO{}
	for rows.Next() {
		var d dayDTO
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.PersonID, &d.PersonName, &d.WorkDate,
			&d.DerivedHours, &d.AdjustedHours, &d.OvertimeHours,
			&d.Status, &d.AdjustedReason, &d.ApprovedAt, &d.HasFlag); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"days": out})
}

type adjustReq struct {
	AdjustedHours  float64 `json:"adjusted_hours"`
	AdjustedReason string  `json:"adjusted_reason"`
}

func (h *Handler) AdjustDay(w http.ResponseWriter, r *http.Request) {
	dayID, ok := parseID(w, r, "dayId")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req adjustReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	f := map[string]string{}
	if req.AdjustedHours < 0 || req.AdjustedHours > 24 {
		f["adjusted_hours"] = "0 ile 24 arasında olmalı."
	}
	if strings.TrimSpace(req.AdjustedReason) == "" {
		f["adjusted_reason"] = "Düzeltme gerekçesi zorunlu."
	}
	if len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}

	var d dayDTO
	err := h.pool.QueryRow(r.Context(), `
		WITH upd AS (
			UPDATE attendance_days
			SET adjusted_hours=$1, adjusted_by=$2, adjusted_reason=$3, status='adjusted', updated_at=now()
			WHERE id=$4 AND status <> 'approved'
			RETURNING id, project_id, person_id, work_date, derived_hours, adjusted_hours,
			          overtime_hours, status, adjusted_reason, approved_at
		)
		SELECT upd.id, upd.project_id, upd.person_id, to_char(upd.work_date,'YYYY-MM-DD'),
		       upd.derived_hours::float8, upd.adjusted_hours::float8, upd.overtime_hours::float8,
		       upd.status, upd.adjusted_reason, upd.approved_at,
		       EXISTS(
		         SELECT 1 FROM attendance_events e
		         WHERE e.project_id = upd.project_id AND e.person_id = upd.person_id
		           AND (e.captured_at AT TIME ZONE 'Europe/Istanbul')::date = upd.work_date
		           AND e.geofence_ok = false
		       ) AS has_flag
		FROM upd`,
		req.AdjustedHours, uid, strings.TrimSpace(req.AdjustedReason), dayID,
	).Scan(&d.ID, &d.ProjectID, &d.PersonID, &d.WorkDate,
		&d.DerivedHours, &d.AdjustedHours, &d.OvertimeHours, &d.Status, &d.AdjustedReason, &d.ApprovedAt, &d.HasFlag)
	if err != nil {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt bulunamadı ya da onaylanmış bir dönem, düzeltilemez.", nil)
		return
	}

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: uid.String(), Entity: "attendance_days", EntityID: d.ID.String(), Action: audit.ActionUpdate,
		After: map[string]any{"adjusted_hours": req.AdjustedHours, "adjusted_reason": req.AdjustedReason},
		IP:    m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"day": d})
}

type approveReq struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	PersonID *string `json:"person_id"`
}

func (h *Handler) ApproveDays(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req approveReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if _, err := time.Parse("2006-01-02", req.From); err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"from": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	if _, err := time.Parse("2006-01-02", req.To); err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"to": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	if req.PersonID != nil {
		if _, err := uuid.Parse(*req.PersonID); err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"person_id": "geçersiz UUID"})
			return
		}
	}

	rows, err := h.pool.Query(r.Context(), `
		UPDATE attendance_days
		SET status='approved', approved_by=$1, approved_at=now(), updated_at=now()
		WHERE project_id=$2 AND work_date BETWEEN $3::date AND $4::date
		  AND ($5::uuid IS NULL OR person_id = $5::uuid)
		  AND status <> 'approved'
		RETURNING id`,
		uid, pid, req.From, req.To, req.PersonID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: uid.String(), Entity: "attendance_days", EntityID: pid.String(), Action: audit.ActionUpdate,
		After: map[string]any{"bulk_approve_count": count, "from": req.From, "to": req.To},
		IP:    m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"approved_count": count})
}

// ---------------------------------------------------------------------------
// Ham olaylar — şef ekranının bir günü/kişiyi incelemesi için. Konum
// alanları burada mevcuttur; izin bazlı maskeleme serileştirme katmanında
// uygulanır (bkz. Adım 3).
// ---------------------------------------------------------------------------

type eventDTO struct {
	ID         uuid.UUID `json:"id"`
	PersonID   uuid.UUID `json:"person_id"`
	EventType  string    `json:"event_type"`
	Source     string    `json:"source"`
	Lat        *float64  `json:"lat,omitempty"`
	Lng        *float64  `json:"lng,omitempty"`
	AccuracyM  *float64  `json:"accuracy_m,omitempty"`
	DistanceM  *float64  `json:"distance_m,omitempty"`
	GeofenceOK *bool     `json:"geofence_ok"`
	CapturedAt time.Time `json:"captured_at"`
	DeviceID   *string   `json:"device_id,omitempty"`
	Note       *string   `json:"note,omitempty"`
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	q := r.URL.Query()
	var personID *string
	var workDate *string
	if v := strings.TrimSpace(q.Get("person_id")); v != "" {
		if _, err := uuid.Parse(v); err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"person_id": "geçersiz UUID"})
			return
		}
		personID = &v
	}
	if v := strings.TrimSpace(q.Get("work_date")); v != "" {
		if _, err := time.Parse("2006-01-02", v); err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"work_date": "geçerli bir tarih (YYYY-MM-DD) girin"})
			return
		}
		workDate = &v
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, person_id, event_type, source, lat, lng, accuracy_m::float8, distance_m::float8,
		       geofence_ok, captured_at, device_id, note
		FROM attendance_events
		WHERE project_id=$1
		  AND ($2::uuid IS NULL OR person_id = $2::uuid)
		  AND ($3::date IS NULL OR (captured_at AT TIME ZONE 'Europe/Istanbul')::date = $3::date)
		ORDER BY captured_at DESC
		LIMIT 500`,
		pid, personID, workDate)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []eventDTO{}
	for rows.Next() {
		var e eventDTO
		if err := rows.Scan(&e.ID, &e.PersonID, &e.EventType, &e.Source, &e.Lat, &e.Lng,
			&e.AccuracyM, &e.DistanceM, &e.GeofenceOK, &e.CapturedAt, &e.DeviceID, &e.Note); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, e)
	}

	maskables := make([]locationMasker, len(out))
	for i := range out {
		maskables[i] = &out[i]
	}
	h.writeLocationAwareJSON(w, r, http.StatusOK, "attendance_events", pid.String(), maskables, map[string]any{"events": out})
}
