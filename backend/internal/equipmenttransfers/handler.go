// Package equipmenttransfers — Makine/Ekipman/Araç Envanteri Faz B/F:
// proje-arası transfer talebi + çok kademeli onay hiyerarşisi
// (bkz. internal/approvals — entity_type="equipment_transfer",
// approval_chain_steps: SiteManager -> ProjectManager -> Admin, migration
// 000054). Herhangi bir kademe reddederse zincir durur, escalate yok.
package equipmenttransfers

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/approvals"
	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
)

const entityType = "equipment_transfer"

type Handler struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
	nt   *notify.Service
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder, nt *notify.Service) *Handler {
	return &Handler{pool: pool, rec: rec, nt: nt}
}

type decisionDTO struct {
	StepOrder  int    `json:"step_order"`
	RoleName   string `json:"role_name"`
	DecidedBy  string `json:"decided_by_name"`
	Decision   string `json:"decision"`
	Note       string `json:"note,omitempty"`
	DecidedAt  string `json:"decided_at"`
}

type transferDTO struct {
	ID                  uuid.UUID     `json:"id"`
	CompanyEquipmentID  uuid.UUID     `json:"company_equipment_id"`
	EquipmentAd         string        `json:"equipment_ad"`
	EquipmentTip        string        `json:"equipment_tip"`
	FromProjectID       uuid.UUID     `json:"from_project_id"`
	ToProjectID         uuid.UUID     `json:"to_project_id"`
	ToProjectName       string        `json:"to_project_name"`
	RequestedBy         uuid.UUID     `json:"requested_by"`
	RequestedByName     string        `json:"requested_by_name"`
	Status              string        `json:"status"`
	CreatedAt           string        `json:"created_at"`
	CurrentStep         int           `json:"current_step"`
	TotalSteps          int           `json:"total_steps"`
	CurrentStepRoleName string        `json:"current_step_role_name"`
	CanDecide           bool          `json:"can_decide"`
	Decisions           []decisionDTO `json:"decisions"`
}

// Create — talep açar: company_equipment_id'nin GÜNCEL current_project_id'si
// (sunucu tarafında okunur, istemciden alınmaz — sahtecilik önlenir)
// from_project_id olur, URL'deki projectID to_project_id olur. Aynı
// transaction'da approvals.Start ile onay süreci açılır (kademe 1).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	var b struct {
		CompanyEquipmentID uuid.UUID `json:"company_equipment_id"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}

	ctx := r.Context()
	var fromProjectID *uuid.UUID
	if err := h.pool.QueryRow(ctx,
		`SELECT current_project_id FROM company_equipment WHERE id=$1`,
		b.CompanyEquipmentID).Scan(&fromProjectID); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Envanter kaydı bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if fromProjectID == nil {
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation,
			"Bu kayıt hiçbir projeye atanmamış — transfer talebi gerekmez, doğrudan projenize ekleyebilirsiniz.", nil)
		return
	}
	if *fromProjectID == pid {
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation,
			"Bu kayıt zaten sizin projenizde.", nil)
		return
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO equipment_transfer_requests (company_equipment_id, from_project_id, to_project_id, requested_by)
		 VALUES ($1,$2,$3,$4) RETURNING id`,
		b.CompanyEquipmentID, *fromProjectID, pid, uid).Scan(&id); err != nil {
		if strings.Contains(err.Error(), "idx_etr_equipment_pending") {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Bu kayıt için zaten bekleyen bir transfer talebi var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	_, firstRole, err := approvals.Start(ctx, tx, entityType, id, *fromProjectID, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r)
		return
	}

	var toProjectName, equipmentAd string
	_ = h.pool.QueryRow(ctx, `SELECT name FROM projects WHERE id=$1`, pid).Scan(&toProjectName)
	_ = h.pool.QueryRow(ctx, `SELECT ad FROM company_equipment WHERE id=$1`, b.CompanyEquipmentID).Scan(&equipmentAd)
	notifyStepRole(ctx, h.pool, h.nt, *fromProjectID, firstRole, uid, notify.Input{
		Type:       notify.TypeEquipmentTransferRequested,
		Title:      equipmentAd + " — " + toProjectName + " projesine transfer talebi (" + rbac.RoleName(firstRole) + " onayı bekliyor)",
		EntityType: "equipment_transfer_requests", EntityID: &id, ProjectID: fromProjectID,
	})

	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

// ListPending — from_project_id = URL'deki projectID olan talepler
// ("bu projeden gitmesi istenen ekipmanlar"). Her talebin GEÇERLİ kademesi
// için gerekli rolü viewer TAŞIMIYORSA can_decide=false döner (Onayla/Reddet
// butonları frontend'de gizlenir) — talep yine de görünür kalır ki akış
// şeffaf olsun (sırada olmayan kademe de görülebilsin).
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
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	ctx := r.Context()

	viewerRoles, err := loadUserRoles(ctx, h.pool, pid, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	rows, err := h.pool.Query(ctx, `
		SELECT t.id, t.company_equipment_id, ce.ad, ce.tip, t.from_project_id, t.to_project_id,
		       p.name, t.requested_by, u.full_name, t.status,
		       to_char(t.created_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       ar.id, ar.current_step, ar.total_steps
		FROM equipment_transfer_requests t
		JOIN company_equipment ce ON ce.id = t.company_equipment_id
		JOIN projects p ON p.id = t.to_project_id
		JOIN users u ON u.id = t.requested_by
		LEFT JOIN approval_requests ar ON ar.entity_type=$3 AND ar.entity_id = t.id
		WHERE t.from_project_id=$1 AND t.status=$2
		ORDER BY t.created_at DESC`, pid, status, entityType)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []transferDTO{}
	for rows.Next() {
		var t transferDTO
		var approvalID *uuid.UUID
		if err := rows.Scan(&t.ID, &t.CompanyEquipmentID, &t.EquipmentAd, &t.EquipmentTip,
			&t.FromProjectID, &t.ToProjectID, &t.ToProjectName, &t.RequestedBy, &t.RequestedByName,
			&t.Status, &t.CreatedAt, &approvalID, &t.CurrentStep, &t.TotalSteps); err != nil {
			httpx.Internal(w, r)
			return
		}
		if t.CurrentStep > 0 {
			role, _ := approvals.StepRole(ctx, h.pool, entityType, t.CurrentStep)
			t.CurrentStepRoleName = rbac.RoleName(role)
			t.CanDecide = viewerRoles[role]
		}
		if approvalID != nil {
			t.Decisions = loadDecisions(ctx, h.pool, *approvalID)
		}
		out = append(out, t)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"transfers": out})
}

type decideReq struct {
	DecisionNote string `json:"decision_note"`
}

func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) { h.decide(w, r, true) }
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request)  { h.decide(w, r, false) }

// decide — çağıranın bu talebin GEÇERLİ kademesinin rolüne sahip olduğu
// doğrulanır, approvals.Decide ile karar kaydedilip zincir ilerletilir/durdurulur.
// Yalnızca zincir TAMAMEN onaylandığında (isFinal && approved) fiili taşıma
// (aktif atamayı kapat, hedef projede yeni atama aç, company_equipment
// current_project_id güncelle) uygulanır — ara kademelerde ekipman kaynak
// projede kalmaya devam eder.
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

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(ctx)

	var equipmentID, toProjectID, requestedBy uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT company_equipment_id, to_project_id, requested_by
		 FROM equipment_transfer_requests
		 WHERE id=$1 AND from_project_id=$2 AND status='pending' FOR UPDATE`,
		id, pid).Scan(&equipmentID, &toProjectID, &requestedBy); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Bekleyen talep bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	ar, err := approvals.Get(ctx, tx, entityType, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	stepRole, err := approvals.StepRole(ctx, tx, entityType, ar.CurrentStep)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	allowed, err := approvals.ActorRoleAllowed(ctx, tx, pid, uid, stepRole)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if !allowed {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Bu talebin sırası "+rbac.RoleName(stepRole)+" onayında — bu kademeyi karara bağlama yetkiniz yok.", nil)
		return
	}

	chainStatus, isFinal, nextRole, err := approvals.Decide(ctx, tx, ar, uid, stepRole, approve, req.DecisionNote)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	if isFinal {
		status := chainStatus // "approved" | "rejected"
		if status == "approved" {
			// Ekipman talep açıldığından beri hâlâ bu projede mi? (yarış durumu koruması)
			var stillHere bool
			if err := tx.QueryRow(ctx,
				`SELECT current_project_id=$2 FROM company_equipment WHERE id=$1`,
				equipmentID, pid).Scan(&stillHere); err != nil {
				httpx.Internal(w, r)
				return
			}
			if !stillHere {
				httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
					"Ekipman bu talep açıldığından beri başka bir yere taşınmış görünüyor.", nil)
				return
			}
			if _, err := tx.Exec(ctx,
				`UPDATE project_machines SET ayrilma_tarihi=CURRENT_DATE
				 WHERE company_equipment_id=$1 AND project_id=$2 AND ayrilma_tarihi IS NULL`,
				equipmentID, pid); err != nil {
				httpx.Internal(w, r)
				return
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO project_machines (project_id, company_equipment_id, from_transfer_id) VALUES ($1,$2,$3)`,
				toProjectID, equipmentID, id); err != nil {
				httpx.Internal(w, r)
				return
			}
			if _, err := tx.Exec(ctx,
				`UPDATE company_equipment SET current_project_id=$2, updated_at=now() WHERE id=$1`,
				equipmentID, toProjectID); err != nil {
				httpx.Internal(w, r)
				return
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE equipment_transfer_requests
			 SET status=$2, decided_by=$3, decided_at=now(), decision_note=NULLIF(btrim($4),''), row_version=row_version+1
			 WHERE id=$1`, id, status, uid, req.DecisionNote); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	m := audit.MetaFrom(ctx)
	_ = h.rec.RecordTx(ctx, tx, audit.Entry{
		ActorID: m.ActorID, Entity: "equipment_transfer_requests", EntityID: id.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"step": ar.CurrentStep, "role": stepRole, "chain_status": chainStatus}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r)
		return
	}

	switch {
	case chainStatus == "approved":
		h.nt.Send(ctx, notify.Input{
			UserIDs: []uuid.UUID{requestedBy}, Type: notify.TypeEquipmentTransferDecided,
			Title: "Transfer talebiniz tüm kademelerden onaylandı", Body: req.DecisionNote,
			EntityType: "equipment_transfer_requests", EntityID: &id, ProjectID: &toProjectID,
		})
	case chainStatus == "rejected":
		h.nt.Send(ctx, notify.Input{
			UserIDs: []uuid.UUID{requestedBy}, Type: notify.TypeEquipmentTransferDecided,
			Title: "Transfer talebiniz " + rbac.RoleName(stepRole) + " kademesinde reddedildi", Body: req.DecisionNote,
			EntityType: "equipment_transfer_requests", EntityID: &id, ProjectID: &toProjectID,
		})
	default: // "pending" — bir sonraki kademeye geçti
		var equipmentAd, toProjectName string
		_ = h.pool.QueryRow(ctx, `SELECT ad FROM company_equipment WHERE id=$1`, equipmentID).Scan(&equipmentAd)
		_ = h.pool.QueryRow(ctx, `SELECT name FROM projects WHERE id=$1`, toProjectID).Scan(&toProjectName)
		notifyStepRole(ctx, h.pool, h.nt, pid, nextRole, uid, notify.Input{
			Type:       notify.TypeEquipmentTransferRequested,
			Title:      equipmentAd + " — " + toProjectName + " projesine transfer talebi (" + rbac.RoleName(nextRole) + " onayı bekliyor)",
			EntityType: "equipment_transfer_requests", EntityID: &id, ProjectID: &pid,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": chainStatus})
}

// loadUserRoles — projede uid'nin taşıdığı rol kodlarının kümesi.
func loadUserRoles(ctx context.Context, pool *pgxpool.Pool, pid, uid uuid.UUID) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `
		SELECT r.code FROM project_members pm JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.user_id=$2 AND pm.deleted_at IS NULL`, pid, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var code string
		if rows.Scan(&code) == nil {
			out[code] = true
		}
	}
	return out, nil
}

// loadDecisions — bir onay sürecinde bugüne kadar verilen kararlar (geçmiş
// gösterimi için, en eski kademeden en yeniye).
func loadDecisions(ctx context.Context, pool *pgxpool.Pool, approvalRequestID uuid.UUID) []decisionDTO {
	rows, err := pool.Query(ctx, `
		SELECT d.step_order, d.role_code, u.full_name, d.decision, COALESCE(d.note,''),
		       to_char(d.decided_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM approval_decisions d JOIN users u ON u.id = d.decided_by
		WHERE d.approval_request_id=$1 ORDER BY d.step_order ASC`, approvalRequestID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []decisionDTO{}
	for rows.Next() {
		var d decisionDTO
		var roleCode string
		if rows.Scan(&d.StepOrder, &roleCode, &d.DecidedBy, &d.Decision, &d.Note, &d.DecidedAt) != nil {
			continue
		}
		d.RoleName = rbac.RoleName(roleCode)
		out = append(out, d)
	}
	return out
}

// NotifyApprovers — equipment.approve_transfer İZNİNE sahip TÜM proje
// üyelerine bildirim gönderir (chain'in HANGİ kademesinde olduğuna bakmaz —
// "teslim alındı" gibi zincirin dışındaki genel olaylar için, bkz.
// machines/handler.go markDelivered ve rental_reminder.go). Zincirin
// GEÇERLİ kademesine özel bildirim için notifyStepRole kullanılır.
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
		if rbac.RoleHasDefault(role, "equipment.approve_transfer") {
			targets = append(targets, uid)
		}
	}
	if len(targets) == 0 {
		return
	}
	in.UserIDs = targets
	nt.Send(ctx, in)
}

// notifyStepRole — projede roleCode rolüne sahip üyelere (actor hariç) bildirim yollar.
func notifyStepRole(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service, pid uuid.UUID, roleCode string, actor uuid.UUID, in notify.Input) {
	rows, err := pool.Query(ctx, `
		SELECT pm.user_id FROM project_members pm JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL AND r.code=$2`, pid, roleCode)
	if err != nil {
		return
	}
	defer rows.Close()
	var targets []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if rows.Scan(&uid) != nil {
			continue
		}
		if uid == actor {
			continue
		}
		targets = append(targets, uid)
	}
	if len(targets) == 0 {
		return
	}
	in.UserIDs = targets
	nt.Send(ctx, in)
}
