// Package materials — Faz 5: Malzeme Onay Süreci (Submittals / MAR).
//
// Plan §6.5 + §8 (Faz 5). Yaşam döngüsü:
//   Submitted → UnderReview → Approved | ConditionallyApproved | Rejected
// Karar notu karar anında ZORUNLUDUR. Doküman ekleri Faz 2 polimorfik doküman
// motoruna 'material_approval' entity_type ile bağlanır. Her statü geçişi
// workflow_transitions'a yazılır (Plan §5.1); bildirimler merkezi notify
// servisiyle gönderilir (Plan Faz 4 altyapısı).
//
// Satır seviyesi güvenlik (Plan §4):
//   - SubcontractorRep yalnızca kendi firmasının MAR'larını görür (backend'de zorunlu).
//   - Client (müşavir/işveren) yalnızca "kendisine sunulan" MAR'ları görür:
//     UnderReview ve karara bağlanmış kayıtlar. Submitted (henüz iç incelemede)
//     kayıtlar Client'a görünmez — kısıtlı inceleme ekranı bu filtre üzerine kurulur.
package materials

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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
// DTO ve yardımcılar
// ---------------------------------------------------------------------------

type marDTO struct {
	ID               uuid.UUID  `json:"id"`
	ProjectID        uuid.UUID  `json:"project_id"`
	SubcontractorID  *uuid.UUID `json:"subcontractor_id,omitempty"`
	MARNo            string     `json:"mar_no"`
	MaterialName     string     `json:"material_name"`
	SpecRef          *string    `json:"spec_ref,omitempty"`
	Manufacturer     *string    `json:"manufacturer,omitempty"`
	Status           string     `json:"status"`
	DecisionNote     *string    `json:"decision_note,omitempty"`
	DecidedBy        *uuid.UUID `json:"decided_by,omitempty"`
	DecidedByName    *string    `json:"decided_by_name,omitempty"`
	DecidedAt        *time.Time `json:"decided_at,omitempty"`
	CreatedBy        uuid.UUID  `json:"created_by"`
	CreatedByName    string     `json:"created_by_name"`
	SubcontractorName *string   `json:"subcontractor_name,omitempty"`
	AttachmentCount  int        `json:"attachment_count"`
	RowVersion       int        `json:"row_version"`
	CreatedAt        time.Time  `json:"created_at"`
}

const marSelect = `
	SELECT m.id, m.project_id, m.subcontractor_id, m.mar_no, m.material_name,
	       m.spec_ref, m.manufacturer, m.status, m.decision_note,
	       m.decided_by, du.full_name, m.decided_at,
	       m.created_by, cu.full_name, s.company_name,
	       (SELECT count(*) FROM documents d
	         WHERE d.entity_type='material_approval' AND d.entity_id=m.id AND d.deleted_at IS NULL),
	       m.row_version, m.created_at
	FROM material_approvals m
	JOIN users cu ON cu.id = m.created_by
	LEFT JOIN users du ON du.id = m.decided_by
	LEFT JOIN subcontractors s ON s.id = m.subcontractor_id`

func scanMAR(row pgx.Row, m *marDTO) error {
	var cnt int64
	err := row.Scan(&m.ID, &m.ProjectID, &m.SubcontractorID, &m.MARNo, &m.MaterialName,
		&m.SpecRef, &m.Manufacturer, &m.Status, &m.DecisionNote,
		&m.DecidedBy, &m.DecidedByName, &m.DecidedAt,
		&m.CreatedBy, &m.CreatedByName, &m.SubcontractorName,
		&cnt, &m.RowVersion, &m.CreatedAt)
	m.AttachmentCount = int(cnt)
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

// scope — istek sahibinin satır seviyesi görünürlük kapsamı.
type scope struct {
	uid            uuid.UUID
	sub            *uuid.UUID // SubcontractorRep ise firması; nil = kısıtsız
	clientLimited  bool       // Client rolü: yalnızca kendisine sunulanlar
}

// resolveScope — project_members'tan taşeron bağını ve rol kodunu çözer.
// Filtreler DAİMA backend'de uygulanır (Plan §4).
func (h *Handler) resolveScope(ctx context.Context, uid, pid uuid.UUID) (scope, error) {
	sc := scope{uid: uid}
	var roleCode *string
	err := h.pool.QueryRow(ctx, `
		SELECT pm.subcontractor_id, r.code
		FROM project_members pm
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.user_id=$1 AND pm.project_id=$2 AND pm.deleted_at IS NULL
		LIMIT 1`, uid, pid).Scan(&sc.sub, &roleCode)
	if errors.Is(err, pgx.ErrNoRows) {
		// Üyelik yok (ör. global Admin) → kısıtsız; izin katmanı zaten denetledi.
		return sc, nil
	}
	if err != nil {
		return sc, err
	}
	if roleCode != nil && *roleCode == "Client" {
		sc.clientLimited = true
	}
	return sc, nil
}

func (h *Handler) requireScope(w http.ResponseWriter, r *http.Request, pid uuid.UUID) (scope, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return scope{}, false
	}
	sc, err := h.resolveScope(r.Context(), uid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return scope{}, false
	}
	return sc, true
}

// ---------------------------------------------------------------------------
// Liste + tekil
// ---------------------------------------------------------------------------

func (h *Handler) ListMARs(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sc, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	rows, err := h.pool.Query(r.Context(), marSelect+`
		WHERE m.project_id=$1 AND m.deleted_at IS NULL
		  AND ($2='' OR m.status=$2)
		  AND ($3::uuid IS NULL OR m.subcontractor_id=$3)
		  AND ($4 = false OR m.status <> 'Submitted')
		ORDER BY m.mar_no DESC`, pid, status, sc.sub, sc.clientLimited)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []marDTO{}
	for rows.Next() {
		var m marDTO
		if err := scanMAR(rows, &m); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, m)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"material_approvals": out})
}

// getScoped — kapsam filtresiyle tekil MAR; görünmüyorsa NotFound (bilgi sızıntısı yok).
func (h *Handler) getScoped(ctx context.Context, pid, id uuid.UUID, sc scope) (*marDTO, error) {
	var m marDTO
	err := scanMAR(h.pool.QueryRow(ctx, marSelect+`
		WHERE m.project_id=$1 AND m.id=$2 AND m.deleted_at IS NULL
		  AND ($3::uuid IS NULL OR m.subcontractor_id=$3)
		  AND ($4 = false OR m.status <> 'Submitted')`,
		pid, id, sc.sub, sc.clientLimited), &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (h *Handler) GetMAR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	sc, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	m, err := h.getScoped(r.Context(), pid, id, sc)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "MAR bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

// ---------------------------------------------------------------------------
// Oluşturma (Submitted) + taslak düzenleme
// ---------------------------------------------------------------------------

type marReq struct {
	SubcontractorID *string `json:"subcontractor_id"`
	MaterialName    string  `json:"material_name"`
	SpecRef         *string `json:"spec_ref"`
	Manufacturer    *string `json:"manufacturer"`
}

func (h *Handler) CreateMAR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sc, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	var req marReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if errs := validateMAR(req.MaterialName); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	subID, verrs := resolveSubID(req.SubcontractorID, sc.sub)
	if len(verrs) > 0 {
		httpx.ValidationFailed(w, r, verrs)
		return
	}
	if subID != nil {
		// Taşeron bu projeye ait mi?
		var one int
		err := h.pool.QueryRow(r.Context(), `
			SELECT 1 FROM subcontractors WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
			subID, pid).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "bu projede kayıtlı taşeron değil"})
			return
		}
		if err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// MAR numarası: proje içinde sıralı (MAR-001...). Numaralamadaki yarış
	// koşulunu tx kapsamlı advisory lock ile engelleriz (tekil indeks son sigorta).
	if _, err := tx.Exec(r.Context(),
		`SELECT pg_advisory_xact_lock(hashtext('mar_no:' || $1::text))`, pid); err != nil {
		httpx.Internal(w, r)
		return
	}
	var marNo string
	if err := tx.QueryRow(r.Context(), `
		SELECT 'MAR-' || lpad((count(*)+1)::text, 3, '0')
		FROM material_approvals WHERE project_id=$1`, pid).Scan(&marNo); err != nil {
		httpx.Internal(w, r)
		return
	}

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO material_approvals
			(project_id, subcontractor_id, mar_no, material_name, spec_ref, manufacturer, created_by)
		VALUES ($1,$2,$3,$4,NULLIF(btrim(coalesce($5,'')),''),NULLIF(btrim(coalesce($6,'')),''),$7)
		RETURNING id`,
		pid, subID, marNo, req.MaterialName, req.SpecRef, req.Manufacturer, sc.uid).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	_, _ = tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
		VALUES ('material_approval', $1, '', 'Submitted', $2)`, id, sc.uid)

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "material_approvals", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"mar_no": marNo, "material_name": req.MaterialName, "status": "Submitted"},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Bildirim: inceleme yetkisi olan (Client hariç — henüz sunulmadı) üyeler.
	h.notifyByCapability(r.Context(), pid, "material_approvals.review", false, sc.uid, notify.Input{
		Type:  notify.TypeMARSubmitted,
		Title: marNo + " — yeni malzeme onay talebi",
		Body:  req.MaterialName,
		EntityType: "material_approvals", EntityID: &id, ProjectID: &pid,
	})

	out, err := h.getScoped(r.Context(), pid, id, sc)
	if err != nil {
		httpx.JSON(w, http.StatusCreated, map[string]string{"id": id.String(), "mar_no": marNo})
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// UpdateMAR — yalnızca Submitted statüsünde künye düzenlenir; taşeron temsilcisi
// yalnızca kendi kaydını düzenleyebilir (kapsam filtresi zaten uygular).
func (h *Handler) UpdateMAR(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	sc, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	cur, err := h.getScoped(r.Context(), pid, id, sc)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "MAR bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if cur.Status != "Submitted" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca 'Sunuldu' durumundaki MAR düzenlenebilir.", nil)
		return
	}
	var req marReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if errs := validateMAR(req.MaterialName); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE material_approvals
		SET material_name=$3,
		    spec_ref=NULLIF(btrim(coalesce($4,'')),''),
		    manufacturer=NULLIF(btrim(coalesce($5,'')),''),
		    row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='Submitted'`,
		id, pid, req.MaterialName, req.SpecRef, req.Manufacturer)
	if err != nil || ct.RowsAffected() == 0 {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "material_approvals", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"material_name": cur.MaterialName},
		After:  map[string]interface{}{"material_name": req.MaterialName},
		IP:     m.IP, ReqID: m.ReqID,
	})
	out, _ := h.getScoped(r.Context(), pid, id, sc)
	httpx.JSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------------
// İş akışı: incelemeye alma + karar
// ---------------------------------------------------------------------------

// ReviewMAR — Submitted → UnderReview. Bu geçiş MAR'ı müşavire/işverene "sunar";
// Client bu andan itibaren kaydı görür ve karar verebilir.
func (h *Handler) ReviewMAR(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "Submitted", "UnderReview", "", false)
}

type decideReq struct {
	Decision     string `json:"decision"` // Approved | ConditionallyApproved | Rejected
	DecisionNote string `json:"decision_note"`
}

// DecideMAR — UnderReview → karar. Karar notu ZORUNLU (Plan Faz 5).
func (h *Handler) DecideMAR(w http.ResponseWriter, r *http.Request) {
	var req decideReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if errs := validateDecision(req.Decision, req.DecisionNote); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	h.transition(w, r, "UnderReview", req.Decision, req.DecisionNote, true)
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request, from, to, note string, isDecision bool) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	sc, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	cur, err := h.getScoped(r.Context(), pid, id, sc)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "MAR bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if cur.Status != from {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Bu geçiş yalnızca '"+statusLabel(from)+"' durumundan yapılabilir.", nil)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	if isDecision {
		_, err = tx.Exec(r.Context(), `
			UPDATE material_approvals
			SET status=$3, decision_note=$4, decided_by=$5, decided_at=now(), row_version=row_version+1
			WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid, to, note, sc.uid)
	} else {
		_, err = tx.Exec(r.Context(), `
			UPDATE material_approvals
			SET status=$3, row_version=row_version+1
			WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid, to)
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	_, _ = tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('material_approval', $1, $2, $3, $4, NULLIF($5,''))`, id, from, to, sc.uid, note)

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "material_approvals", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": from}, After: map[string]interface{}{"status": to, "decision_note": note},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}

	if isDecision {
		// Karar bildirimi: talep sahibi + ilgili taşeronun temsilcileri (Plan Faz 5 kabul).
		targets := []uuid.UUID{cur.CreatedBy}
		if cur.SubcontractorID != nil {
			targets = append(targets, h.subcontractorUsers(r.Context(), pid, *cur.SubcontractorID)...)
		}
		h.nt.Send(r.Context(), notify.Input{
			UserIDs: targets,
			Type:    notify.TypeMARDecided,
			Title:   cur.MARNo + " kararı: " + statusLabel(to),
			Body:    note,
			EntityType: "material_approvals", EntityID: &id, ProjectID: &pid,
		})
	} else {
		// Karara sunuldu: karar verme yetkisi olanlara (Client dahil) bildir.
		h.notifyByCapability(r.Context(), pid, "material_approvals.decide", true, sc.uid, notify.Input{
			Type:  notify.TypeMARUnderReview,
			Title: cur.MARNo + " kararınıza sunuldu",
			Body:  cur.MaterialName,
			EntityType: "material_approvals", EntityID: &id, ProjectID: &pid,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": to})
}

// ---------------------------------------------------------------------------
// Bildirim hedefleme yardımcıları
// ---------------------------------------------------------------------------

// notifyByCapability — projede rolü verilen izni varsayılan olarak taşıyan
// üyelere gönderir (actor hariç). Basit hedefleme: kullanıcı bazlı override'lar
// bildirim hedeflemesinde dikkate alınmaz; erişim kontrolü zaten API katmanında.
func (h *Handler) notifyByCapability(ctx context.Context, pid uuid.UUID, permCode string, includeClient bool, actor uuid.UUID, in notify.Input) {
	rows, err := h.pool.Query(ctx, `
		SELECT pm.user_id, r.code FROM project_members pm
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL`, pid)
	if err != nil {
		h.log.Error("MAR bildirim hedefleri okunamadı", "err", err)
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
		if role == "Client" && !includeClient {
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

func (h *Handler) subcontractorUsers(ctx context.Context, pid, subID uuid.UUID) []uuid.UUID {
	rows, err := h.pool.Query(ctx, `
		SELECT user_id FROM project_members
		WHERE project_id=$1 AND subcontractor_id=$2 AND deleted_at IS NULL`, pid, subID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if rows.Scan(&uid) == nil {
			out = append(out, uid)
		}
	}
	return out
}
