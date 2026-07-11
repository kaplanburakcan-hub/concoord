package procurement

// Sipariş (PO) yaşam döngüsü, kısmi teslimat ve tedarik durum panosu.
//
//   Ordered → PartiallyDelivered → Delivered
//   Ordered/PartiallyDelivered → Cancelled
//
// Teslimat eklendikçe statü otomatik ilerler: mark_delivered=false ise
// PartiallyDelivered, true ise Delivered. İrsaliye fotoğrafı Faz 2 doküman
// motorundan gelen document_id ile bağlanır (doc_category='Delivery').

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
)

type deliveryDTO struct {
	ID             uuid.UUID  `json:"id"`
	DeliveryNoteNo string     `json:"delivery_note_no"`
	DeliveredAt    time.Time  `json:"delivered_at"`
	ReceivedBy     uuid.UUID  `json:"received_by"`
	ReceivedByName string     `json:"received_by_name"`
	DocumentID     *uuid.UUID `json:"document_id,omitempty"`
	Note           *string    `json:"note,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type poDTO struct {
	ID            uuid.UUID     `json:"id"`
	ProjectID     uuid.UUID     `json:"project_id"`
	PRID          *uuid.UUID    `json:"pr_id,omitempty"`
	PRNo          *string       `json:"pr_no,omitempty"`
	PONo          string        `json:"po_no"`
	SupplierName  string        `json:"supplier_name"`
	Amount        *float64      `json:"amount,omitempty"`
	Currency      string        `json:"currency"`
	Status        string        `json:"status"`
	ExpectedDate  *string       `json:"expected_date,omitempty"`
	Note          *string       `json:"note,omitempty"`
	CreatedBy     uuid.UUID     `json:"created_by"`
	CreatedByName string        `json:"created_by_name"`
	Overdue       bool          `json:"overdue"` // beklenen tarih geçti, kapanmadı
	DeliveryCount int           `json:"delivery_count"`
	Deliveries    []deliveryDTO `json:"deliveries,omitempty"`
	RowVersion    int           `json:"row_version"`
	CreatedAt     time.Time     `json:"created_at"`
}

const poSelect = `
	SELECT o.id, o.project_id, o.pr_id, pr.pr_no, o.po_no, o.supplier_name,
	       o.amount, o.currency, o.status,
	       to_char(o.expected_date,'YYYY-MM-DD'), o.note,
	       o.created_by, cu.full_name,
	       (o.expected_date IS NOT NULL AND o.expected_date < CURRENT_DATE
	        AND o.status IN ('Ordered','PartiallyDelivered')),
	       (SELECT count(*) FROM deliveries d
	         WHERE d.po_id = o.id AND d.deleted_at IS NULL),
	       o.row_version, o.created_at
	FROM purchase_orders o
	JOIN users cu ON cu.id = o.created_by
	LEFT JOIN purchase_requests pr ON pr.id = o.pr_id`

func scanPO(row pgx.Row, o *poDTO) error {
	var cnt int64
	err := row.Scan(&o.ID, &o.ProjectID, &o.PRID, &o.PRNo, &o.PONo, &o.SupplierName,
		&o.Amount, &o.Currency, &o.Status, &o.ExpectedDate, &o.Note,
		&o.CreatedBy, &o.CreatedByName, &o.Overdue, &cnt, &o.RowVersion, &o.CreatedAt)
	o.DeliveryCount = int(cnt)
	return err
}

func (h *Handler) getPO(ctx context.Context, pid, id uuid.UUID, withDeliveries bool) (*poDTO, error) {
	var o poDTO
	err := scanPO(h.pool.QueryRow(ctx, poSelect+`
		WHERE o.id=$1 AND o.project_id=$2 AND o.deleted_at IS NULL`, id, pid), &o)
	if err != nil {
		return nil, err
	}
	if !withDeliveries {
		return &o, nil
	}
	rows, err := h.pool.Query(ctx, `
		SELECT d.id, d.delivery_note_no, d.delivered_at, d.received_by, u.full_name,
		       d.document_id, d.note, d.created_at
		FROM deliveries d JOIN users u ON u.id = d.received_by
		WHERE d.po_id=$1 AND d.deleted_at IS NULL
		ORDER BY d.delivered_at DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d deliveryDTO
		if err := rows.Scan(&d.ID, &d.DeliveryNoteNo, &d.DeliveredAt, &d.ReceivedBy,
			&d.ReceivedByName, &d.DocumentID, &d.Note, &d.CreatedAt); err != nil {
			return nil, err
		}
		o.Deliveries = append(o.Deliveries, d)
	}
	return &o, rows.Err()
}

// ---------------------------------------------------------------------------
// PO uçları
// ---------------------------------------------------------------------------

// ListPOs — proje siparişleri; ?status= ve ?overdue=1 filtreleri.
func (h *Handler) ListPOs(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	onlyOverdue := r.URL.Query().Get("overdue") == "1"
	rows, err := h.pool.Query(r.Context(), poSelect+`
		WHERE o.project_id=$1 AND o.deleted_at IS NULL
		  AND ($2='' OR o.status=$2)
		  AND (NOT $3 OR (o.expected_date IS NOT NULL AND o.expected_date < CURRENT_DATE
		                  AND o.status IN ('Ordered','PartiallyDelivered')))
		ORDER BY o.created_at DESC
		LIMIT 500`, pid, status, onlyOverdue)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []poDTO{}
	for rows.Next() {
		var o poDTO
		if err := scanPO(rows, &o); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, o)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"purchase_orders": out})
}

// CreatePO — bağımsız sipariş (PR'sız acil alımlar için). PR'dan dönüşüm için
// ConvertPR kullanılır.
func (h *Handler) CreatePO(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req poReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	expected, errs := validatePO(req)
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	id, poNo, err := h.insertPO(r.Context(), tx, pid, nil, uid, req, expected)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_orders", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"po_no": poNo, "supplier": req.SupplierName, "status": "Ordered"},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	out, err := h.getPO(r.Context(), pid, id, true)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": id.String(), "po_no": poNo})
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// insertPO — numaralı PO satırı + açılış workflow kaydı (tx içinde).
func (h *Handler) insertPO(ctx context.Context, tx pgx.Tx, pid uuid.UUID, prID *uuid.UUID,
	uid uuid.UUID, req poReq, expected *time.Time) (uuid.UUID, string, error) {
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext('po_no:' || $1::text))`, pid); err != nil {
		return uuid.Nil, "", err
	}
	var poNo string
	if err := tx.QueryRow(ctx, `
		SELECT 'PO-' || lpad((count(*)+1)::text, 3, '0')
		FROM purchase_orders WHERE project_id=$1`, pid).Scan(&poNo); err != nil {
		return uuid.Nil, "", err
	}
	currency := "TRY"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO purchase_orders
			(project_id, pr_id, po_no, supplier_name, amount, currency, expected_date, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF(btrim(coalesce($8,'')),''),$9)
		RETURNING id`,
		pid, prID, poNo, strings.TrimSpace(req.SupplierName), req.Amount, currency,
		expected, req.Note, uid).Scan(&id)
	if err != nil {
		return uuid.Nil, "", err
	}
	_, _ = tx.Exec(ctx, `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
		VALUES ('purchase_order', $1, '', 'Ordered', $2)`, id, uid)
	return id, poNo, nil
}

// ConvertPR — Approved PR → Converted + yeni PO (aynı tx; kabul kriteri:
// PR→PO zinciri izlenebilir — PO.pr_id ile bağ kurulur).
func (h *Handler) ConvertPR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	prID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req poReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	expected, errs := validatePO(req)
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// Yalnızca Approved dönüştürülebilir (trigger da yalnız bu geçişe izin verir).
	ct, err := tx.Exec(r.Context(), `
		UPDATE purchase_requests SET status='Converted', row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='Approved'`, prID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca onaylanmış PR siparişe dönüştürülebilir.", nil)
		return
	}
	id, poNo, err := h.insertPO(r.Context(), tx, pid, &prID, uid, req, expected)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	_, _ = tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('purchase_request', $1, 'Approved', 'Converted', $2, $3)`, prID, uid, poNo)

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_orders", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"po_no": poNo, "pr_id": prID.String(), "supplier": req.SupplierName},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	out, err := h.getPO(r.Context(), pid, id, true)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": id.String(), "po_no": poNo})
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// GetPO — teslimatlarıyla birlikte tekil sipariş.
func (h *Handler) GetPO(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	out, err := h.getPO(r.Context(), pid, id, true)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Sipariş bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// UpdatePO — kapanmamış (Ordered/PartiallyDelivered) sipariş künyesi düzenlenir.
func (h *Handler) UpdatePO(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req poReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	expected, errs := validatePO(req)
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	currency := "TRY"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE purchase_orders
		SET supplier_name=$3, amount=$4, currency=$5, expected_date=$6,
		    note=NULLIF(btrim(coalesce($7,'')),''),
		    overdue_notified_at=CASE WHEN expected_date IS DISTINCT FROM $6 THEN NULL
		                             ELSE overdue_notified_at END,
		    row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL
		  AND status IN ('Ordered','PartiallyDelivered')`,
		id, pid, strings.TrimSpace(req.SupplierName), req.Amount, currency, expected, req.Note)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kapanmış (teslim edilmiş/iptal) sipariş düzenlenemez.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_orders", EntityID: id.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"supplier": req.SupplierName, "expected_date": req.ExpectedDate},
		IP:    m.IP, ReqID: m.ReqID,
	})
	out, _ := h.getPO(r.Context(), pid, id, true)
	httpx.JSON(w, http.StatusOK, out)
}

// CancelPO — Ordered/PartiallyDelivered → Cancelled.
func (h *Handler) CancelPO(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())
	var from string
	err := h.pool.QueryRow(r.Context(), `
		WITH old AS (
			SELECT status FROM purchase_orders
			WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL
			  AND status IN ('Ordered','PartiallyDelivered')
			FOR UPDATE
		)
		UPDATE purchase_orders o SET status='Cancelled', row_version=o.row_version+1
		FROM old WHERE o.id=$1
		RETURNING old.status`, id, pid).Scan(&from)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca açık siparişler iptal edilebilir.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	_, _ = h.pool.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
		VALUES ('purchase_order', $1, $2, 'Cancelled', $3)`, id, from, uid)
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_orders", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": from},
		After:  map[string]interface{}{"status": "Cancelled"},
		IP:     m.IP, ReqID: m.ReqID,
	})
	out, _ := h.getPO(r.Context(), pid, id, true)
	httpx.JSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Teslimatlar (kısmi teslimat + irsaliye fotoğrafı)
// ---------------------------------------------------------------------------

// AddDelivery — açık siparişe teslimat kaydı ekler. mark_delivered=true
// siparişi kapatır (Delivered), aksi halde PartiallyDelivered olur. İrsaliye
// fotoğrafı document_id ile bağlanır (mobil kamera → Faz 2 doküman motoru).
func (h *Handler) AddDelivery(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	poID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())
	var req deliveryReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	at, errs := validateDelivery(req)
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	var docID *uuid.UUID
	if req.DocumentID != nil && strings.TrimSpace(*req.DocumentID) != "" {
		d, err := uuid.Parse(strings.TrimSpace(*req.DocumentID))
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"document_id": "geçersiz UUID"})
			return
		}
		// İrsaliye dokümanı bu projeye ait olmalı.
		var one int
		err = h.pool.QueryRow(r.Context(), `
			SELECT 1 FROM documents WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
			d, pid).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.ValidationFailed(w, r, map[string]string{"document_id": "bu projede kayıtlı doküman değil"})
			return
		}
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		docID = &d
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// Sipariş açık mı? (satır kilidiyle statü yarışını engelle)
	var from string
	err = tx.QueryRow(r.Context(), `
		SELECT status FROM purchase_orders
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, poID, pid).Scan(&from)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Sipariş bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if from != "Ordered" && from != "PartiallyDelivered" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kapanmış siparişe teslimat eklenemez.", nil)
		return
	}

	var did uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO deliveries (po_id, delivery_note_no, delivered_at, received_by, document_id, note)
		VALUES ($1,$2,$3,$4,$5,NULLIF(btrim(coalesce($6,'')),''))
		RETURNING id`,
		poID, strings.TrimSpace(req.DeliveryNoteNo), at, uid, docID, req.Note).Scan(&did)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	to := "PartiallyDelivered"
	if req.MarkDelivered {
		to = "Delivered"
	}
	if to != from {
		if _, err := tx.Exec(r.Context(), `
			UPDATE purchase_orders SET status=$2, row_version=row_version+1
			WHERE id=$1`, poID, to); err != nil {
			httpx.Internal(w, r)
			return
		}
		_, _ = tx.Exec(r.Context(), `
			INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
			VALUES ('purchase_order', $1, $2, $3, $4, $5)`,
			poID, from, to, uid, "irsaliye: "+strings.TrimSpace(req.DeliveryNoteNo))
	}

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "deliveries", EntityID: did.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"po_id": poID.String(),
			"delivery_note_no": req.DeliveryNoteNo, "po_status": to},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	out, err := h.getPO(r.Context(), pid, poID, true)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": did.String()})
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// ---------------------------------------------------------------------------
// Tedarik durum panosu
// ---------------------------------------------------------------------------

type boardOut struct {
	PRCounts   map[string]int `json:"pr_counts"`
	POCounts   map[string]int `json:"po_counts"`
	OverduePRs []prDTO        `json:"overdue_prs"` // ihtiyaç tarihi geçmiş, siparişsiz
	OverduePOs []poDTO        `json:"overdue_pos"` // beklenen tarihi geçmiş, kapanmamış
}

// Board — tedarik durum panosu (Plan Faz 7: geciken siparişler vurgulu).
func (h *Handler) Board(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	out := boardOut{PRCounts: map[string]int{}, POCounts: map[string]int{},
		OverduePRs: []prDTO{}, OverduePOs: []poDTO{}}

	rows, err := h.pool.Query(r.Context(), `
		SELECT status, count(*) FROM purchase_requests
		WHERE project_id=$1 AND deleted_at IS NULL GROUP BY status`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var s string
		var c int
		if rows.Scan(&s, &c) == nil {
			out.PRCounts[s] = c
		}
	}
	rows.Close()

	rows, err = h.pool.Query(r.Context(), `
		SELECT status, count(*) FROM purchase_orders
		WHERE project_id=$1 AND deleted_at IS NULL GROUP BY status`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var s string
		var c int
		if rows.Scan(&s, &c) == nil {
			out.POCounts[s] = c
		}
	}
	rows.Close()

	rows, err = h.pool.Query(r.Context(), prSelect+`
		WHERE p.project_id=$1 AND p.deleted_at IS NULL
		  AND p.needed_by_date < CURRENT_DATE AND p.status IN ('Submitted','Approved')
		ORDER BY p.needed_by_date LIMIT 100`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var p prDTO
		if scanPR(rows, &p) == nil {
			out.OverduePRs = append(out.OverduePRs, p)
		}
	}
	rows.Close()

	rows, err = h.pool.Query(r.Context(), poSelect+`
		WHERE o.project_id=$1 AND o.deleted_at IS NULL
		  AND o.expected_date IS NOT NULL AND o.expected_date < CURRENT_DATE
		  AND o.status IN ('Ordered','PartiallyDelivered')
		ORDER BY o.expected_date LIMIT 100`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for rows.Next() {
		var o poDTO
		if scanPO(rows, &o) == nil {
			out.OverduePOs = append(out.OverduePOs, o)
		}
	}
	rows.Close()

	httpx.JSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// Worker: geciken sipariş uyarısı (kabul kriteri — uyarı üretimi)
// ---------------------------------------------------------------------------

// NotifyOverduePOs — beklenen teslim tarihi geçmiş, kapanmamış ve daha önce
// uyarılmamış siparişler için siparişi açan + (varsa) PR sahibine bildirim
// üretir. Worker periyodik çağırır; overdue_notified_at ile tekrarlanmaz
// (tarih güncellenirse alan sıfırlanır ve uyarı yeniden kurulur).
func NotifyOverduePOs(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service, log *slog.Logger) {
	rows, err := pool.Query(ctx, `
		SELECT o.id, o.project_id, o.po_no, o.supplier_name,
		       to_char(o.expected_date,'DD.MM.YYYY'), o.created_by, pr.requested_by
		FROM purchase_orders o
		LEFT JOIN purchase_requests pr ON pr.id = o.pr_id
		WHERE o.deleted_at IS NULL
		  AND o.status IN ('Ordered','PartiallyDelivered')
		  AND o.expected_date IS NOT NULL AND o.expected_date < CURRENT_DATE
		  AND o.overdue_notified_at IS NULL
		LIMIT 200`)
	if err != nil {
		log.Error("geciken sipariş taraması başarısız", "err", err)
		return
	}
	type row struct {
		id, pid   uuid.UUID
		poNo, sup string
		expected  string
		createdBy uuid.UUID
		requester *uuid.UUID
	}
	items := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.pid, &x.poNo, &x.sup, &x.expected, &x.createdBy, &x.requester); err != nil {
			rows.Close()
			log.Error("geciken sipariş satırı okunamadı", "err", err)
			return
		}
		items = append(items, x)
	}
	rows.Close()

	for _, x := range items {
		targets := []uuid.UUID{x.createdBy}
		if x.requester != nil && *x.requester != x.createdBy {
			targets = append(targets, *x.requester)
		}
		poID, pid := x.id, x.pid
		nt.Send(ctx, notify.Input{
			UserIDs: targets, Type: notify.TypePOOverdue,
			Title: x.poNo + " — sipariş teslim tarihi geçti",
			Body:  x.sup + " · beklenen: " + x.expected,
			EntityType: "purchase_orders", EntityID: &poID, ProjectID: &pid,
		})
		if _, err := pool.Exec(ctx,
			`UPDATE purchase_orders SET overdue_notified_at=now() WHERE id=$1`, x.id); err != nil {
			log.Error("gecikme işareti yazılamadı", "po", x.id, "err", err)
		}
	}
	if len(items) > 0 {
		log.Info("geciken sipariş uyarıları üretildi", "count", len(items))
	}
}
