// Package calendarfeed — kişiye özel, salt okunur takvim (ICS) bağlantıları (Plan E12).
//
// Güvenlik:
//   - Belirteç 32 bayt rastgele değerdir; veritabanında yalnızca SHA-256 özeti tutulur.
//   - Bağlantı 90 gün geçerlidir ve iptal edilebilir.
//   - Veri, bağlantı çağrıldığı anda yeniden kontrol edilir: kullanıcının proje
//     üyeliği yoksa ya da proje silinmişse hiçbir olay dönmez.
//   - Yanıt önbelleğe alınmaz (Cache-Control: no-store).
package calendarfeed

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

const feedLifetime = 90 * 24 * time.Hour

type Handler struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder) *Handler {
	return &Handler{pool: pool, rec: rec}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create — kullanıcı için proje takvim bağlantısı üretir. Belirteç yanıtta bir kez döner.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	token, err := newToken()
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	expires := time.Now().UTC().Add(feedLifetime)
	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO calendar_feeds (user_id, project_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4) RETURNING id`, uid, pid, hashToken(token), expires).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "calendar_feeds", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid.String(), "expires_at": expires.Format(time.RFC3339)},
		IP:    m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":         id,
		"path":       "/api/v1/calendar/" + token + ".ics",
		"expires_at": expires.Format(time.RFC3339),
	})
}

// Revoke — yalnızca bağlantının sahibi kendi bağlantısını iptal edebilir.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	fid, ok := parseID(w, r, "feedID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE calendar_feeds SET revoked_at=now()
		WHERE id=$1 AND project_id=$2 AND user_id=$3 AND revoked_at IS NULL`, fid, pid, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Bağlantı bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// Serve — ICS akışını döner. Geçersiz, süresi dolmuş, iptal edilmiş ya da
// üyeliği bitmiş bağlantılar aynı 404 yanıtını alır (bilgi sızmaz).
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSuffix(chi.URLParam(r, "token"), ".ics")
	notFound := func() { http.NotFound(w, r) }
	if raw == "" || len(raw) > 128 {
		notFound()
		return
	}
	var pid, uid uuid.UUID
	var projectName string
	err := h.pool.QueryRow(r.Context(), `
		SELECT f.project_id, f.user_id, p.name
		FROM calendar_feeds f
		JOIN projects p ON p.id = f.project_id AND p.deleted_at IS NULL
		JOIN project_members pm ON pm.project_id = f.project_id AND pm.user_id = f.user_id
		     AND pm.deleted_at IS NULL
		WHERE f.token_hash=$1 AND f.revoked_at IS NULL AND f.expires_at > now()`,
		hashToken(raw)).Scan(&pid, &uid, &projectName)
	if err == pgx.ErrNoRows {
		notFound()
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	events, err := h.events(r, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//IPKS//Concoord//TR\r\nCALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString(foldLine("X-WR-CALNAME:" + escapeText(projectName+" — İPKS")))
	b.WriteString("\r\n")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, e := range events {
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(foldLine("UID:" + e.uid + "@ipks"))
		b.WriteString("\r\n")
		b.WriteString("DTSTAMP:" + stamp + "\r\n")
		b.WriteString("DTSTART;VALUE=DATE:" + e.date + "\r\n")
		b.WriteString(foldLine("SUMMARY:" + escapeText(e.summary)))
		b.WriteString("\r\n")
		b.WriteString("TRANSP:TRANSPARENT\r\n")
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `inline; filename="ipks.ics"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

type event struct {
	uid, date, summary string
}

// events — milestone planlanan tarihleri, hakediş dönem sonları, ödeme olayları
// ve çek keside tarihleri. Yalnızca silinmemiş kayıtlar.
func (h *Handler) events(r *http.Request, pid uuid.UUID) ([]event, error) {
	var out []event
	add := func(rows pgx.Rows, mk func(rows pgx.Rows) (event, error)) error {
		defer rows.Close()
		for rows.Next() {
			e, err := mk(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id::text, to_char(planned_date,'YYYY-MM-DD'), name FROM milestones
		WHERE project_id=$1 AND deleted_at IS NULL AND planned_date IS NOT NULL`, pid)
	if err != nil {
		return nil, err
	}
	if err := add(rows, func(rr pgx.Rows) (event, error) {
		var e event
		err := rr.Scan(&e.uid, &e.date, &e.summary)
		e.uid = "milestone-" + e.uid
		e.summary = "Milestone: " + e.summary
		return e, err
	}); err != nil {
		return nil, err
	}

	rows, err = h.pool.Query(r.Context(), `
		SELECT id::text, to_char(period_end,'YYYY-MM-DD'), 'Hakediş dönem sonu #' || period_no
		FROM progress_payments
		WHERE project_id=$1 AND deleted_at IS NULL AND period_end IS NOT NULL`, pid)
	if err != nil {
		return nil, err
	}
	if err := add(rows, func(rr pgx.Rows) (event, error) {
		var e event
		var label string
		err := rr.Scan(&e.uid, &e.date, &label)
		e.uid = "hakedis-" + e.uid
		e.summary = label
		return e, err
	}); err != nil {
		return nil, err
	}

	rows, err = h.pool.Query(r.Context(), `
		SELECT d.id::text, to_char(d.event_date,'YYYY-MM-DD'), d.amount::float8, d.payment_method
		FROM progress_payment_disbursements d
		JOIN progress_payments pp ON pp.id = d.progress_payment_id AND pp.deleted_at IS NULL
		WHERE pp.project_id=$1 AND d.deleted_at IS NULL`, pid)
	if err != nil {
		return nil, err
	}
	if err := add(rows, func(rr pgx.Rows) (event, error) {
		var e event
		var amount float64
		var method string
		err := rr.Scan(&e.uid, &e.date, &amount, &method)
		e.uid = "odeme-" + e.uid
		e.summary = fmt.Sprintf("Ödeme (%s): %.2f", method, amount)
		return e, err
	}); err != nil {
		return nil, err
	}

	rows, err = h.pool.Query(r.Context(), `
		SELECT d.id::text, to_char(d.cek_keside_tarihi,'YYYY-MM-DD'), d.amount::float8
		FROM progress_payment_disbursements d
		JOIN progress_payments pp ON pp.id = d.progress_payment_id AND pp.deleted_at IS NULL
		WHERE pp.project_id=$1 AND d.deleted_at IS NULL AND d.payment_method='cek'
		  AND d.cek_keside_tarihi IS NOT NULL`, pid)
	if err != nil {
		return nil, err
	}
	if err := add(rows, func(rr pgx.Rows) (event, error) {
		var e event
		var amount float64
		err := rr.Scan(&e.uid, &e.date, &amount)
		e.uid = "cek-" + e.uid
		e.summary = fmt.Sprintf("Çek keside tarihi: %.2f", amount)
		return e, err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// escapeText — ICS metin kuralları: ters eğik çizgi, noktalı virgül, virgül ve satır sonu.
func escapeText(s string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\r\n", "\\n", "\n", "\\n", "\r", "")
	return r.Replace(s)
}

// foldLine — 75 bayttan uzun satırları RFC 5545 uyarınca böler (karakter sınırında).
func foldLine(line string) string {
	const limit = 75
	if len(line) <= limit {
		return line
	}
	var b strings.Builder
	cur := 0
	for _, rn := range line {
		n := utf8.RuneLen(rn)
		max := limit
		if cur > 0 {
			max = limit - 1 // devam satırları bir boşlukla başlar
		}
		if cur+n > max {
			b.WriteString("\r\n ")
			cur = 1
		}
		b.WriteRune(rn)
		cur += n
	}
	return b.String()
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}

// FeedDTO — kullanıcının etkin bağlantısı (belirteç dönmez; yalnız kimlik ve tarihler).
type FeedDTO struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt string    `json:"created_at"`
	ExpiresAt string    `json:"expires_at"`
}

// List — kullanıcının bu projedeki süresi dolmamış, iptal edilmemiş bağlantıları.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, to_char(created_at,'YYYY-MM-DD HH24:MI'), to_char(expires_at,'YYYY-MM-DD HH24:MI')
		FROM calendar_feeds
		WHERE project_id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC`, pid, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []FeedDTO{}
	for rows.Next() {
		var d FeedDTO
		if err := rows.Scan(&d.ID, &d.CreatedAt, &d.ExpiresAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"feeds": out})
}
