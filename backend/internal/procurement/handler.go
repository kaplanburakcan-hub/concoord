package procurement

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
	"github.com/ipks/ipks/backend/internal/validate"
)

type Handler struct {
	pool *pgxpool.Pool
	eval *rbac.Evaluator
	rec  *audit.Recorder
	nt   *notify.Service
	log  *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder, nt *notify.Service, log *slog.Logger) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, nt: nt, log: log}
}

// ---------------------------------------------------------------------------
// DTO'lar
// ---------------------------------------------------------------------------

type prItemDTO struct {
	ID           uuid.UUID `json:"id"`
	MaterialName string    `json:"material_name"`
	Spec         *string   `json:"spec,omitempty"`
	Qty          float64   `json:"qty"`
	Unit         string    `json:"unit"`
	Note         *string   `json:"note,omitempty"`
}

type prDTO struct {
	ID              uuid.UUID   `json:"id"`
	ProjectID       uuid.UUID   `json:"project_id"`
	PRNo            string      `json:"pr_no"`
	Status          string      `json:"status"`
	NeededByDate    string      `json:"needed_by_date"`
	Note            *string     `json:"note,omitempty"`
	RequestedBy     uuid.UUID   `json:"requested_by"`
	RequestedByName string      `json:"requested_by_name"`
	SubmittedAt     *time.Time  `json:"submitted_at,omitempty"`
	DecidedBy       *uuid.UUID  `json:"decided_by,omitempty"`
	DecidedByName   *string     `json:"decided_by_name,omitempty"`
	DecidedAt       *time.Time  `json:"decided_at,omitempty"`
	DecisionNote    *string     `json:"decision_note,omitempty"`
	POID            *uuid.UUID  `json:"po_id,omitempty"` // Converted ise bağlı sipariş
	PONo            *string     `json:"po_no,omitempty"`
	Overdue         bool        `json:"overdue"` // ihtiyaç tarihi geçti, sipariş yok
	ItemCount       int         `json:"item_count"`
	Items           []prItemDTO `json:"items,omitempty"`
	RowVersion      int         `json:"row_version"`
	CreatedAt       time.Time   `json:"created_at"`
}

const prSelect = `
	SELECT p.id, p.project_id, p.pr_no, p.status, to_char(p.needed_by_date,'YYYY-MM-DD'),
	       p.note, p.requested_by, ru.full_name, p.submitted_at,
	       p.decided_by, du.full_name, p.decided_at, p.decision_note,
	       po.id, po.po_no,
	       (p.needed_by_date < CURRENT_DATE AND p.status IN ('Submitted','Approved')),
	       (SELECT count(*) FROM purchase_request_items i
	         WHERE i.pr_id = p.id AND i.deleted_at IS NULL),
	       p.row_version, p.created_at
	FROM purchase_requests p
	JOIN users ru ON ru.id = p.requested_by
	LEFT JOIN users du ON du.id = p.decided_by
	LEFT JOIN purchase_orders po ON po.pr_id = p.id AND po.deleted_at IS NULL`

func scanPR(row pgx.Row, p *prDTO) error {
	var cnt int64
	err := row.Scan(&p.ID, &p.ProjectID, &p.PRNo, &p.Status, &p.NeededByDate,
		&p.Note, &p.RequestedBy, &p.RequestedByName, &p.SubmittedAt,
		&p.DecidedBy, &p.DecidedByName, &p.DecidedAt, &p.DecisionNote,
		&p.POID, &p.PONo, &p.Overdue, &cnt, &p.RowVersion, &p.CreatedAt)
	p.ItemCount = int(cnt)
	return err
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.",
			map[string]string{param: "geçersiz UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) getPR(ctx context.Context, pid, id uuid.UUID, withItems bool) (*prDTO, error) {
	var p prDTO
	err := scanPR(h.pool.QueryRow(ctx, prSelect+`
		WHERE p.id=$1 AND p.project_id=$2 AND p.deleted_at IS NULL`, id, pid), &p)
	if err != nil {
		return nil, err
	}
	if !withItems {
		return &p, nil
	}
	rows, err := h.pool.Query(ctx, `
		SELECT id, material_name, spec, qty, unit, note
		FROM purchase_request_items
		WHERE pr_id=$1 AND deleted_at IS NULL
		ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it prItemDTO
		if err := rows.Scan(&it.ID, &it.MaterialName, &it.Spec, &it.Qty, &it.Unit, &it.Note); err != nil {
			return nil, err
		}
		p.Items = append(p.Items, it)
	}
	return &p, rows.Err()
}

// ---------------------------------------------------------------------------
// PR uçları
// ---------------------------------------------------------------------------

// ListPRs — proje PR listesi; ?status= filtresi. Gecikenler overdue=true ile
// işaretlenir (kabul kriteri: ihtiyaç tarihi geçmiş kalemler uyarı üretir).
func (h *Handler) ListPRs(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	rows, err := h.pool.Query(r.Context(), prSelect+`
		WHERE p.project_id=$1 AND p.deleted_at IS NULL
		  AND ($2='' OR p.status=$2)
		ORDER BY p.created_at DESC
		LIMIT 500`, pid, status)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []prDTO{}
	for rows.Next() {
		var p prDTO
		if err := scanPR(rows, &p); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"purchase_requests": out})
}

// CreatePR — yeni taslak PR + kalemleri. PR numarası proje içinde sıralıdır;
// numaralamadaki yarış koşulu tx kapsamlı advisory lock ile engellenir.
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())
	var req prReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	needed, errs := validatePR(req)
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	if kerrs, kerr := validate.NotAfterKesinKabul(r.Context(), h.pool, pid, needed, "needed_by_date"); kerr != nil {
		httpx.Internal(w, r)
		return
	} else if len(kerrs) > 0 {
		httpx.ValidationFailed(w, r, kerrs)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(),
		`SELECT pg_advisory_xact_lock(hashtext('pr_no:' || $1::text))`, pid); err != nil {
		httpx.Internal(w, r)
		return
	}
	var prNo string
	if err := tx.QueryRow(r.Context(), `
		SELECT 'PR-' || lpad((count(*)+1)::text, 3, '0')
		FROM purchase_requests WHERE project_id=$1`, pid).Scan(&prNo); err != nil {
		httpx.Internal(w, r)
		return
	}

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO purchase_requests (project_id, pr_no, requested_by, needed_by_date, note)
		VALUES ($1,$2,$3,$4,NULLIF(btrim(coalesce($5,'')),''))
		RETURNING id`, pid, prNo, uid, needed, req.Note).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := insertItems(r.Context(), tx, id, req.Items); err != nil {
		httpx.Internal(w, r)
		return
	}

	_, _ = tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
		VALUES ('purchase_request', $1, '', 'Draft', $2)`, id, uid)

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_requests", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"pr_no": prNo, "status": "Draft", "item_count": len(req.Items)},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	out, err := h.getPR(r.Context(), pid, id, true)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": id.String(), "pr_no": prNo})
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func insertItems(ctx context.Context, tx pgx.Tx, prID uuid.UUID, items []prItemReq) error {
	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO purchase_request_items (pr_id, material_name, spec, qty, unit, note)
			VALUES ($1,$2,NULLIF(btrim(coalesce($3,'')),''),$4,$5,NULLIF(btrim(coalesce($6,'')),''))`,
			prID, strings.TrimSpace(it.MaterialName), it.Spec, it.Qty,
			strings.TrimSpace(it.Unit), it.Note); err != nil {
			return err
		}
	}
	return nil
}

// GetPR — kalemleriyle birlikte tekil PR.
func (h *Handler) GetPR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	out, err := h.getPR(r.Context(), pid, id, true)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Satınalma talebi bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// UpdatePR — yalnızca Draft düzenlenebilir (Submitted geri çekilmez; ret
// sonrası yeni PR açılır). Kalemler tümden değiştirilir (replace).
func (h *Handler) UpdatePR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	cur, err := h.getPR(r.Context(), pid, id, false)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Satınalma talebi bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if cur.Status != "Draft" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca taslak PR düzenlenebilir.", nil)
		return
	}
	var req prReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	needed, errs := validatePR(req)
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	if kerrs, kerr := validate.NotAfterKesinKabul(r.Context(), h.pool, pid, needed, "needed_by_date"); kerr != nil {
		httpx.Internal(w, r)
		return
	} else if len(kerrs) > 0 {
		httpx.ValidationFailed(w, r, kerrs)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE purchase_requests
		SET needed_by_date=$3, note=NULLIF(btrim(coalesce($4,'')),''), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='Draft'`,
		id, pid, needed, req.Note)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Internal(w, r)
		return
	}
	// Taslakta kalem seti "replace" edilir: eskiler soft-delete, yeniler eklenir.
	if _, err := tx.Exec(r.Context(), `
		UPDATE purchase_request_items SET deleted_at=now()
		WHERE pr_id=$1 AND deleted_at IS NULL`, id); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := insertItems(r.Context(), tx, id, req.Items); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_requests", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"needed_by_date": cur.NeededByDate, "item_count": cur.ItemCount},
		After:  map[string]interface{}{"needed_by_date": req.NeededByDate, "item_count": len(req.Items)},
		IP:     m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	out, _ := h.getPR(r.Context(), pid, id, true)
	httpx.JSON(w, http.StatusOK, out)
}

// DeletePR — yalnızca taslak silinebilir (soft delete; DB trigger son sigorta).
func (h *Handler) DeletePR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE purchase_requests SET deleted_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='Draft'`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca taslak PR silinebilir.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_requests", EntityID: id.String(),
		Action: audit.ActionDelete, IP: m.IP, ReqID: m.ReqID,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// PR iş akışı: submit → approve | reject → convert (PO)
// ---------------------------------------------------------------------------

// SubmitPR — Draft → Submitted; onay yetkililerine bildirim gider.
func (h *Handler) SubmitPR(w http.ResponseWriter, r *http.Request) {
	h.prTransition(w, r, "Draft", "Submitted", func(p *prDTO, actor uuid.UUID) {
		pid, id := p.ProjectID, p.ID
		h.notifyByCapability(r.Context(), pid, "procurement.approve_pr", actor, notify.Input{
			Type:       notify.TypePRSubmitted,
			Title:      p.PRNo + " — satınalma talebi onay bekliyor",
			Body:       "İhtiyaç tarihi: " + p.NeededByDate,
			EntityType: "purchase_requests", EntityID: &id, ProjectID: &pid,
		})
	})
}

type decideReq struct {
	DecisionNote string `json:"decision_note"`
}

// ApprovePR — Submitted → Approved.
func (h *Handler) ApprovePR(w http.ResponseWriter, r *http.Request) {
	var req decideReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	h.decidePR(w, r, "Approved", req.DecisionNote, false)
}

// RejectPR — Submitted → Rejected. Karar notu ZORUNLU (izlenebilirlik).
func (h *Handler) RejectPR(w http.ResponseWriter, r *http.Request) {
	var req decideReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.DecisionNote) == "" {
		httpx.ValidationFailed(w, r, map[string]string{"decision_note": "ret gerekçesi zorunludur"})
		return
	}
	h.decidePR(w, r, "Rejected", req.DecisionNote, true)
}

func (h *Handler) decidePR(w http.ResponseWriter, r *http.Request, to, note string, rejected bool) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())

	ct, err := h.pool.Exec(r.Context(), `
		UPDATE purchase_requests
		SET status=$3, decided_by=$4, decided_at=now(),
		    decision_note=NULLIF(btrim($5),''), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='Submitted'`,
		id, pid, to, uid, note)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca 'Onay Bekliyor' durumundaki PR karara bağlanabilir.", nil)
		return
	}
	_, _ = h.pool.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('purchase_request', $1, 'Submitted', $2, $3, NULLIF(btrim($4),''))`, id, to, uid, note)

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_requests", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": "Submitted"},
		After:  map[string]interface{}{"status": to, "decision_note": note},
		IP:     m.IP, ReqID: m.ReqID,
	})

	out, err := h.getPR(r.Context(), pid, id, true)
	if err == nil {
		title := out.PRNo + " — satınalma talebi onaylandı"
		if rejected {
			title = out.PRNo + " — satınalma talebi reddedildi"
		}
		oid := out.ID
		h.nt.Send(r.Context(), notify.Input{
			UserIDs: []uuid.UUID{out.RequestedBy},
			Type:    notify.TypePRDecided, Title: title, Body: note,
			EntityType: "purchase_requests", EntityID: &oid, ProjectID: &pid,
		})
		httpx.JSON(w, http.StatusOK, out)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": to})
}

// prTransition — genel amaçlı statü geçişi (Draft→Submitted).
func (h *Handler) prTransition(w http.ResponseWriter, r *http.Request, from, to string,
	after func(p *prDTO, actor uuid.UUID)) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, _ := auth.UserIDFrom(r.Context())

	// Kalemsiz PR onaya gönderilemez.
	if to == "Submitted" {
		var cnt int
		if err := h.pool.QueryRow(r.Context(), `
			SELECT count(*) FROM purchase_request_items
			WHERE pr_id=$1 AND deleted_at IS NULL`, id).Scan(&cnt); err != nil {
			httpx.Internal(w, r)
			return
		}
		if cnt == 0 {
			httpx.ValidationFailed(w, r, map[string]string{"items": "en az bir kalem girin"})
			return
		}
	}

	ct, err := h.pool.Exec(r.Context(), `
		UPDATE purchase_requests
		SET status=$4, submitted_at=CASE WHEN $4='Submitted' THEN now() ELSE submitted_at END,
		    row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status=$3`,
		id, pid, from, to)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Geçersiz durum geçişi.", nil)
		return
	}
	_, _ = h.pool.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
		VALUES ('purchase_request', $1, $2, $3, $4)`, id, from, to, uid)

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "purchase_requests", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": from},
		After:  map[string]interface{}{"status": to},
		IP:     m.IP, ReqID: m.ReqID,
	})

	out, err := h.getPR(r.Context(), pid, id, true)
	if err != nil {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": to})
		return
	}
	if after != nil {
		after(out, uid)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// notifyByCapability — proje üyeleri arasında ilgili izne (rol varsayılanına)
// sahip olanlara bildirim gönderir; aktör hariç tutulur (MAR ile aynı desen).
func (h *Handler) notifyByCapability(ctx context.Context, pid uuid.UUID, permCode string, actor uuid.UUID, in notify.Input) {
	rows, err := h.pool.Query(ctx, `
		SELECT pm.user_id, r.code FROM project_members pm
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL`, pid)
	if err != nil {
		h.log.Error("satınalma bildirim hedefleri okunamadı", "err", err)
		return
	}
	defer rows.Close()
	var targets []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		var role string
		if rows.Scan(&uid, &role) != nil {
			continue
		}
		if uid == actor {
			continue
		}
		if rbac.RoleHasDefault(role, permCode) {
			targets = append(targets, uid)
		}
	}
	if len(targets) == 0 {
		return
	}
	in.UserIDs = targets
	h.nt.Send(ctx, in)
}
