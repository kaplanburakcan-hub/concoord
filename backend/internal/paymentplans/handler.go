// Package paymentplans — Nakit Akış Faz C: ödeme planı değişikliği onay
// akışı. Bir hakediş/ekstre/PO ödemesi sözleşme/tedarikçi varsayılan ödeme
// şeklinden farklı bir şekille girilirse burada bekleyen bir onay talebi
// oluşur; onaylanırsa istenen şekille, reddedilirse varsayılan şekille
// cash_events satırı üretilir. Talebi AÇMA işi (varsayılanla karşılaştırma)
// her kaynak paketinde (payments/statements/procurement) kendi Create
// handler'ında yapılır — bu paket yalnızca ortak onay/red akışını yönetir.
package paymentplans

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
)

type Handler struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
	nt   *notify.Service
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder, nt *notify.Service) *Handler {
	return &Handler{pool: pool, rec: rec, nt: nt}
}

// sourceTable — source_entity → alttaki ödeme planı tablosu. Üçü de aynı
// kolon şeklini paylaşır (id, <fk>, amount, payment_method, event_date,
// cek_keside_tarihi, note, created_by, created_at, deleted_at, row_version).
var sourceTable = map[string]string{
	"progress_payment_disbursement": "progress_payment_disbursements",
	"supplier_payment":              "supplier_statement_payments",
	"po_payment":                    "purchase_order_payments",
}

type changeDTO struct {
	ID              uuid.UUID `json:"id"`
	SourceEntity    string    `json:"source_entity"`
	SourceID        uuid.UUID `json:"source_id"`
	Description     string    `json:"description"`
	Amount          float64   `json:"amount"`
	DefaultMethod   string    `json:"default_method"`
	RequestedMethod string    `json:"requested_method"`
	RequestedBy     uuid.UUID `json:"requested_by"`
	RequestedByName string    `json:"requested_by_name"`
	Status          string    `json:"status"`
	CreatedAt       string    `json:"created_at"`
}

// ListPending — onay bekleyen ödeme planı değişiklik talepleri.
func (h *Handler) ListPending(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "pending"
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, c.source_entity, c.source_id, c.amount_snapshot, c.default_method,
		       c.requested_method, c.requested_by, u.full_name, c.status,
		       to_char(c.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'), c.description_snapshot
		FROM payment_plan_change_requests c
		JOIN users u ON u.id = c.requested_by
		WHERE c.project_id=$1 AND c.status=$2
		ORDER BY c.created_at DESC`, pid, status)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []changeDTO{}
	for rows.Next() {
		var c changeDTO
		if err := rows.Scan(&c.ID, &c.SourceEntity, &c.SourceID, &c.Amount, &c.DefaultMethod,
			&c.RequestedMethod, &c.RequestedBy, &c.RequestedByName, &c.Status,
			&c.CreatedAt, &c.Description); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, c)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"changes": out})
}

type decideReq struct {
	DecisionNote string `json:"decision_note"`
}

// Approve — talep edilen (varsayılan dışı) ödeme şekliyle cash_events satırı üretir.
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, true)
}

// Reject — sistem otomatik olarak varsayılan ödeme şekliyle cash_events satırı üretir;
// altındaki ödeme planı satırının payment_method'u da varsayılana çevrilir.
// Karar notu (red gerekçesi) zorunludur.
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, false)
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, approve bool) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz talep ID.", nil)
		return
	}
	var req decideReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if !approve && strings.TrimSpace(req.DecisionNote) == "" {
		httpx.ValidationFailed(w, r, map[string]string{"decision_note": "ret gerekçesi zorunludur"})
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var sourceEntity string
	var sourceID uuid.UUID
	var defaultMethod, requestedMethod string
	var requestedBy uuid.UUID
	var amount float64
	var description string
	err = tx.QueryRow(r.Context(), `
		SELECT source_entity, source_id, default_method, requested_method, requested_by,
		       amount_snapshot, description_snapshot
		FROM payment_plan_change_requests
		WHERE id=$1 AND project_id=$2 AND status='pending' FOR UPDATE`,
		id, pid).Scan(&sourceEntity, &sourceID, &defaultMethod, &requestedMethod, &requestedBy,
		&amount, &description)
	if err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Bekleyen talep bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	table, ok := sourceTable[sourceEntity]
	if !ok {
		httpx.Internal(w, r)
		return
	}

	finalMethod := requestedMethod
	if !approve {
		finalMethod = defaultMethod
	}

	// Onaylanırsa satırın ödeme şekli zaten istenen şekildeydi (değişmez).
	// Reddedilirse satır varsayılan şekle çevrilir; çek değilse keşide
	// tarihi temizlenir (varsayılan yalnızca yöntemi bilir, tarihi değil).
	if !approve {
		if _, err := tx.Exec(r.Context(), `
			UPDATE `+table+` SET payment_method=$2,
			       cek_keside_tarihi=CASE WHEN $2='cek' THEN cek_keside_tarihi ELSE NULL END
			WHERE id=$1`, sourceID, finalMethod); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	var eventDate string
	if err := tx.QueryRow(r.Context(), `
		SELECT to_char(CASE WHEN payment_method='cek' AND cek_keside_tarihi IS NOT NULL
		                    THEN cek_keside_tarihi ELSE event_date END,'YYYY-MM-DD')
		FROM `+table+` WHERE id=$1`, sourceID).Scan(&eventDate); err != nil {
		httpx.Internal(w, r)
		return
	}

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, payment_method, created_by)
		VALUES ($1,'out',$2,$3,$4,$5,$6::date,$7,$8)`,
		pid, sourceEntity, sourceID, description, amount, eventDate, finalMethod, uid); err != nil {
		httpx.Internal(w, r)
		return
	}

	status := "approved"
	if !approve {
		status = "rejected"
	}
	if _, err := tx.Exec(r.Context(), `
		UPDATE payment_plan_change_requests
		SET status=$2, decided_by=$3, decided_at=now(), decision_note=NULLIF(btrim($4),''), row_version=row_version+1
		WHERE id=$1`, id, status, uid, req.DecisionNote); err != nil {
		httpx.Internal(w, r)
		return
	}

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "payment_plan_change_requests", EntityID: id.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"status": status, "final_method": finalMethod}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}

	title := description + " — ödeme planı onaylandı"
	if !approve {
		title = description + " — ödeme değişiklik talebiniz reddedildi, varsayılan ödeme planı kullanıldı"
	}
	h.nt.Send(r.Context(), notify.Input{
		UserIDs: []uuid.UUID{requestedBy}, Type: notify.TypePaymentPlanDecided,
		Title: title, Body: req.DecisionNote,
		EntityType: sourceEntity, EntityID: &sourceID, ProjectID: &pid,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": status})
}

// NotifyApprovers — talep açılınca `payments.approve_plan_change` yetkisi
// olan proje üyelerine bildirim gönderir. Kaynak paketler (payments/
// statements/procurement), onay gerektiren bir ödeme planı satırı
// oluşturduktan SONRA (kendi tx'leri commit olduktan sonra) bunu çağırır.
func NotifyApprovers(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service, pid uuid.UUID, actor uuid.UUID, in notify.Input) {
	rows, err := pool.Query(ctx, `
		SELECT pm.user_id, r.code FROM project_members pm
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL`, pid)
	if err != nil {
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
		if rbac.RoleHasDefault(role, "payments.approve_plan_change") {
			targets = append(targets, uid)
		}
	}
	if len(targets) == 0 {
		return
	}
	in.UserIDs = targets
	nt.Send(ctx, in)
}
