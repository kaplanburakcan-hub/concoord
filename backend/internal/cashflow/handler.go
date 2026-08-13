package cashflow

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/fixedexpenses"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

// dateRange — ?from=&to= çözer; verilmezse varsayılan bugünden 30 gün
// öncesi ile 60 gün sonrası (geçmiş kasa hareketleri + öngörülen çek/ödeme
// tarihleri birlikte görünsün diye ileri tarihli varsayılan daha geniş).
func dateRange(r *http.Request) (from, to time.Time, ok bool) {
	q := r.URL.Query()
	now := time.Now()
	from = now.AddDate(0, 0, -30)
	to = now.AddDate(0, 0, 60)
	if s := strings.TrimSpace(q.Get("from")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return from, to, false
		}
		from = t
	}
	if s := strings.TrimSpace(q.Get("to")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return from, to, false
		}
		to = t
	}
	return from, to, true
}

// ── Nakit Akış Raporu ────────────────────────────────────────────────────

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	from, to, ok := dateRange(r)
	if !ok {
		httpx.ValidationFailed(w, r, map[string]string{"from": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	group := strings.TrimSpace(r.URL.Query().Get("group"))
	if group != "daily" && group != "weekly" && group != "monthly" {
		group = "monthly"
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT direction, amount::float8, to_char(event_date,'YYYY-MM-DD')
		FROM cash_events
		WHERE project_id=$1 AND deleted_at IS NULL AND event_date BETWEEN $2 AND $3`,
		pid, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Direction, &e.Amount, &e.EventDate); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		events = append(events, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}

	expenses, err := fixedexpenses.ListActive(r.Context(), h.pool, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for _, v := range fixedexpenses.Expand(expenses, from, to) {
		events = append(events, Event{Direction: v.Direction, Amount: v.Amount, EventDate: v.EventDate})
	}

	periods := BuildPeriods(events, from, to, group)
	var totalIn, totalOut float64
	for _, p := range periods {
		totalIn += p.In
		totalOut += p.Out
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"periods": periods,
		"summary": map[string]float64{
			"total_in": totalIn, "total_out": totalOut, "net": totalIn - totalOut,
		},
		"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"), "group": group,
	})
}

// ── Ödeme Planları (toplu liste) ─────────────────────────────────────────

type PlanRow struct {
	ID              string  `json:"id"`
	SourceType      string  `json:"source_type"`
	Direction       string  `json:"direction"`
	Description     string  `json:"description"`
	Amount          float64 `json:"amount"`
	PaymentMethod   *string `json:"payment_method,omitempty"`
	EventDate       string  `json:"event_date"`
	Link            string  `json:"link"`
	PendingApproval bool    `json:"pending_approval"`
}

func (h *Handler) PaymentPlans(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	from, to, ok := dateRange(r)
	if !ok {
		httpx.ValidationFailed(w, r, map[string]string{"from": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	fromS, toS := from.Format("2006-01-02"), to.Format("2006-01-02")

	out := []PlanRow{}

	// 1) Hakediş ödeme planı (progress_payment_disbursements).
	rows, err := h.pool.Query(r.Context(), `
		SELECT d.id, d.amount::float8, d.payment_method,
		       to_char(CASE WHEN d.payment_method='cek' AND d.cek_keside_tarihi IS NOT NULL
		                    THEN d.cek_keside_tarihi ELSE d.event_date END,'YYYY-MM-DD'),
		       'Hakediş #'||pp.period_no||' ödemesi', pp.id::text,
		       EXISTS(SELECT 1 FROM payment_plan_change_requests c
		              WHERE c.source_entity='progress_payment_disbursement'
		                AND c.source_id=d.id AND c.status='pending')
		FROM progress_payment_disbursements d
		JOIN progress_payments pp ON pp.id = d.progress_payment_id
		WHERE pp.project_id=$1 AND d.deleted_at IS NULL
		  AND COALESCE(d.cek_keside_tarihi, d.event_date) BETWEEN $2 AND $3`,
		pid, fromS, toS)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var p PlanRow
		var ppID string
		if err := rows.Scan(&p.ID, &p.Amount, &p.PaymentMethod, &p.EventDate, &p.Description, &ppID, &p.PendingApproval); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		p.SourceType, p.Direction, p.Link = "progress_payment_disbursement", "out", "/hakedis/"+ppID
		out = append(out, p)
	}
	rows.Close()

	// 2) Tedarikçi ekstresi ödeme planı.
	rows, err = h.pool.Query(r.Context(), `
		SELECT p.id, p.amount::float8, p.payment_method,
		       to_char(CASE WHEN p.payment_method='cek' AND p.cek_keside_tarihi IS NOT NULL
		                    THEN p.cek_keside_tarihi ELSE p.event_date END,'YYYY-MM-DD'),
		       s.tedarikci_adi||' — '||s.ekstre_no, s.tedarikci_adi,
		       EXISTS(SELECT 1 FROM payment_plan_change_requests c
		              WHERE c.source_entity='supplier_payment'
		                AND c.source_id=p.id AND c.status='pending')
		FROM supplier_statement_payments p
		JOIN supplier_statements s ON s.id = p.statement_id
		WHERE s.project_id=$1 AND p.deleted_at IS NULL
		  AND COALESCE(p.cek_keside_tarihi, p.event_date) BETWEEN $2 AND $3`,
		pid, fromS, toS)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var p PlanRow
		var tedarikciAdi string
		if err := rows.Scan(&p.ID, &p.Amount, &p.PaymentMethod, &p.EventDate, &p.Description, &tedarikciAdi, &p.PendingApproval); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		p.SourceType, p.Direction = "supplier_payment", "out"
		p.Link = "/tedarikci-ekstreler?tedarikci=" + tedarikciAdi
		out = append(out, p)
	}
	rows.Close()

	// 3) Sipariş (PO) ödeme planı.
	rows, err = h.pool.Query(r.Context(), `
		SELECT p.id, p.amount::float8, p.payment_method,
		       to_char(CASE WHEN p.payment_method='cek' AND p.cek_keside_tarihi IS NOT NULL
		                    THEN p.cek_keside_tarihi ELSE p.event_date END,'YYYY-MM-DD'),
		       o.po_no||' — '||o.supplier_name, o.id::text,
		       EXISTS(SELECT 1 FROM payment_plan_change_requests c
		              WHERE c.source_entity='po_payment'
		                AND c.source_id=p.id AND c.status='pending')
		FROM purchase_order_payments p
		JOIN purchase_orders o ON o.id = p.po_id
		WHERE o.project_id=$1 AND p.deleted_at IS NULL
		  AND COALESCE(p.cek_keside_tarihi, p.event_date) BETWEEN $2 AND $3`,
		pid, fromS, toS)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var p PlanRow
		var poID string
		if err := rows.Scan(&p.ID, &p.Amount, &p.PaymentMethod, &p.EventDate, &p.Description, &poID, &p.PendingApproval); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		p.SourceType, p.Direction, p.Link = "po_payment", "out", "/satinalma/siparisler/"+poID
		out = append(out, p)
	}
	rows.Close()

	// 4) İdari hakediş tahsilatı (yalnızca tarihi girilmiş olanlar).
	rows, err = h.pool.Query(r.Context(), `
		SELECT id::text, tutar::float8, to_char(gelen_odeme_tarihi,'YYYY-MM-DD'),
		       'İdari Hakediş — '||aciklama
		FROM idari_hakedisler
		WHERE project_id=$1 AND deleted_at IS NULL AND gelen_odeme_tarihi IS NOT NULL
		  AND gelen_odeme_tarihi BETWEEN $2 AND $3`,
		pid, fromS, toS)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var p PlanRow
		if err := rows.Scan(&p.ID, &p.Amount, &p.EventDate, &p.Description); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		p.SourceType, p.Direction, p.Link = "idari_hakedis", "in", "/hakedis/idari"
		out = append(out, p)
	}
	rows.Close()

	// Tarihe göre (en yeni önce) sırala.
	sort.Slice(out, func(i, j int) bool { return out[i].EventDate > out[j].EventDate })

	httpx.JSON(w, http.StatusOK, map[string]any{"payments": out, "from": fromS, "to": toS})
}
