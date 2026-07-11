// Package projects — Faz 2 proje çekirdeği: proje CRUD (künye) + milestone
// yönetimi. Üyelik/rol atama Faz 1 admin ucunda kalır. Tüm yazımlar audit'lenir
// ve kritik güncellemeler optimistic locking (row_version) ile korunur (Plan §5.1).
package projects

import (
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
	"github.com/ipks/ipks/backend/internal/rbac"
)

type Handler struct {
	pool *pgxpool.Pool
	eval *rbac.Evaluator
	rec  *audit.Recorder
	log  *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder, log *slog.Logger) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, log: log}
}

// ---------------------------------------------------------------------------
// Projeler
// ---------------------------------------------------------------------------

type projectDTO struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Location    *string    `json:"location,omitempty"`
	ClientName  *string    `json:"client_name,omitempty"`
	BudgetTotal *float64   `json:"budget_total,omitempty"`
	Currency    string     `json:"currency"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Status      string     `json:"status"`
	RowVersion  int        `json:"row_version"`
	CreatedAt   time.Time  `json:"created_at"`
}

const projectCols = `id, code, name, location, client_name, budget_total::float8, currency,
	start_date, end_date, status, row_version, created_at`

func scanProject(row pgx.Row, p *projectDTO) error {
	return row.Scan(&p.ID, &p.Code, &p.Name, &p.Location, &p.ClientName, &p.BudgetTotal,
		&p.Currency, &p.StartDate, &p.EndDate, &p.Status, &p.RowVersion, &p.CreatedAt)
}

// ListProjects — proje seçicinin kaynağı. Kullanıcı yalnızca ÜYESİ olduğu
// projeleri görür (Plan §3). İstisna: projects.view iznine GLOBAL (proje
// bağımsız) GRANT sahibi kullanıcı (bootstrap admin) tüm projeleri görür.
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+projectCols+`
		FROM projects p
		WHERE p.deleted_at IS NULL AND (
			EXISTS (SELECT 1 FROM project_members pm
			        WHERE pm.project_id = p.id AND pm.user_id = $1 AND pm.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM user_permissions up
			           JOIN permissions pe ON pe.id = up.permission_id
			           WHERE up.user_id = $1 AND up.project_id IS NULL
			             AND up.effect = 'GRANT' AND pe.code = 'projects.view')
		)
		ORDER BY p.created_at DESC`, uid)
	if err != nil {
		h.log.Error("proje listesi", "err", err)
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []projectDTO{}
	for rows.Next() {
		var p projectDTO
		if err := scanProject(rows, &p); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"projects": out})
}

type createProjectReq struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Location    *string  `json:"location"`
	ClientName  *string  `json:"client_name"`
	BudgetTotal *float64 `json:"budget_total"`
	Currency    *string  `json:"currency"`
	StartDate   *string  `json:"start_date"` // YYYY-MM-DD
	EndDate     *string  `json:"end_date"`
	Status      *string  `json:"status"`
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	currency := "TRY"
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	status := "Planning"
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}
	if fields := ValidateProject(req.Code, req.Name, currency, status); len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}

	uid, _ := auth.UserIDFrom(r.Context())

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var p projectDTO
	err = scanProject(tx.QueryRow(r.Context(), `
		INSERT INTO projects (code, name, location, client_name, budget_total, currency,
			start_date, end_date, status)
		VALUES ($1,$2,$3,$4,$5,$6, NULLIF($7,'')::date, NULLIF($8,'')::date, $9)
		RETURNING `+projectCols,
		req.Code, req.Name, req.Location, req.ClientName, req.BudgetTotal, currency,
		strDeref(req.StartDate), strDeref(req.EndDate), status), &p)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu proje kodu zaten kullanımda.", nil)
			return
		}
		h.log.Error("proje oluşturma", "err", err)
		httpx.Internal(w, r)
		return
	}

	// Oluşturanı ProjectManager olarak projeye ekle → yeni projeyi yönetebilsin.
	if _, err := tx.Exec(r.Context(), `
		INSERT INTO project_members (project_id, user_id, role_id)
		SELECT $1, $2, r.id FROM roles r WHERE r.code = 'ProjectManager'
		ON CONFLICT (project_id, user_id) WHERE deleted_at IS NULL DO NOTHING`,
		p.ID, uid); err != nil {
		h.log.Error("oluşturan üyelik", "err", err)
		httpx.Internal(w, r)
		return
	}

	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "projects", EntityID: p.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"code": p.Code, "name": p.Name, "status": p.Status},
		IP:    m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"project": p})
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var p projectDTO
	err := scanProject(h.pool.QueryRow(r.Context(),
		`SELECT `+projectCols+` FROM projects WHERE id=$1 AND deleted_at IS NULL`, pid), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"project": p})
}

type updateProjectReq struct {
	Name        *string  `json:"name"`
	Location    *string  `json:"location"`
	ClientName  *string  `json:"client_name"`
	BudgetTotal *float64 `json:"budget_total"`
	Currency    *string  `json:"currency"`
	StartDate   *string  `json:"start_date"`
	EndDate     *string  `json:"end_date"`
	Status      *string  `json:"status"`
	RowVersion  int      `json:"row_version"`
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req updateProjectReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before projectDTO
	err = scanProject(tx.QueryRow(r.Context(),
		`SELECT `+projectCols+` FROM projects WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, pid), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}

	name := before.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	currency := before.Currency
	if req.Currency != nil && strings.TrimSpace(*req.Currency) != "" {
		currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	status := before.Status
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}
	if fields := ValidateProject(before.Code, name, currency, status); len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}
	location := before.Location
	if req.Location != nil {
		location = req.Location
	}
	clientName := before.ClientName
	if req.ClientName != nil {
		clientName = req.ClientName
	}
	budget := before.BudgetTotal
	if req.BudgetTotal != nil {
		budget = req.BudgetTotal
	}

	var after projectDTO
	err = scanProject(tx.QueryRow(r.Context(), `
		UPDATE projects SET
			name=$2, location=$3, client_name=$4, budget_total=$5, currency=$6,
			start_date=COALESCE(NULLIF($7,'')::date, start_date),
			end_date=COALESCE(NULLIF($8,'')::date, end_date),
			status=$9, row_version=row_version+1
		WHERE id=$1
		RETURNING `+projectCols,
		pid, name, location, clientName, budget, currency,
		strDeref(req.StartDate), strDeref(req.EndDate), status), &after)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "projects", EntityID: pid.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"name": before.Name, "status": before.Status},
		After:  map[string]interface{}{"name": after.Name, "status": after.Status},
		IP:     m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"project": after})
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())
	ct, err := tx.Exec(r.Context(),
		`UPDATE projects SET deleted_at=now(), status='Archived' WHERE id=$1 AND deleted_at IS NULL`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "projects", EntityID: pid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// ---------------------------------------------------------------------------
// Milestone'lar
// ---------------------------------------------------------------------------

type milestoneDTO struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Name        string     `json:"name"`
	PlannedDate *time.Time `json:"planned_date,omitempty"`
	ActualDate  *time.Time `json:"actual_date,omitempty"`
	WeightPct   *float64   `json:"weight_pct,omitempty"`
	Status      string     `json:"status"`
	SortOrder   int        `json:"sort_order"`
	RowVersion  int        `json:"row_version"`
	CreatedAt   time.Time  `json:"created_at"`
}

const milestoneCols = `id, project_id, name, planned_date, actual_date, weight_pct::float8,
	status, sort_order, row_version, created_at`

func scanMilestone(row pgx.Row, mst *milestoneDTO) error {
	return row.Scan(&mst.ID, &mst.ProjectID, &mst.Name, &mst.PlannedDate, &mst.ActualDate,
		&mst.WeightPct, &mst.Status, &mst.SortOrder, &mst.RowVersion, &mst.CreatedAt)
}

func (h *Handler) ListMilestones(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+milestoneCols+` FROM milestones
		 WHERE project_id=$1 AND deleted_at IS NULL
		 ORDER BY sort_order, planned_date NULLS LAST, created_at`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []milestoneDTO{}
	for rows.Next() {
		var mst milestoneDTO
		if err := scanMilestone(rows, &mst); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, mst)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"milestones": out})
}

type milestoneReq struct {
	Name        string   `json:"name"`
	PlannedDate *string  `json:"planned_date"`
	ActualDate  *string  `json:"actual_date"`
	WeightPct   *float64 `json:"weight_pct"`
	Status      *string  `json:"status"`
	SortOrder   *int     `json:"sort_order"`
	RowVersion  int      `json:"row_version"`
}

func (h *Handler) CreateMilestone(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req milestoneReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	status := "Planned"
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}
	if fields := ValidateMilestone(req.Name, status, req.WeightPct); len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}
	order := 0
	if req.SortOrder != nil {
		order = *req.SortOrder
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var mst milestoneDTO
	err = scanMilestone(tx.QueryRow(r.Context(), `
		INSERT INTO milestones (project_id, name, planned_date, actual_date, weight_pct, status, sort_order)
		VALUES ($1,$2, NULLIF($3,'')::date, NULLIF($4,'')::date, $5, $6, $7)
		RETURNING `+milestoneCols,
		pid, req.Name, strDeref(req.PlannedDate), strDeref(req.ActualDate),
		req.WeightPct, status, order), &mst)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "milestones", EntityID: mst.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid, "name": mst.Name, "status": mst.Status},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"milestone": mst})
}

func (h *Handler) UpdateMilestone(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	mid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req milestoneReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before milestoneDTO
	err = scanMilestone(tx.QueryRow(r.Context(),
		`SELECT `+milestoneCols+` FROM milestones
		 WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, mid, pid), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Milestone bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}
	name := before.Name
	if req.Name != "" {
		name = strings.TrimSpace(req.Name)
	}
	status := before.Status
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	}
	if fields := ValidateMilestone(name, status, req.WeightPct); len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}
	weight := before.WeightPct
	if req.WeightPct != nil {
		weight = req.WeightPct
	}
	order := before.SortOrder
	if req.SortOrder != nil {
		order = *req.SortOrder
	}

	var after milestoneDTO
	err = scanMilestone(tx.QueryRow(r.Context(), `
		UPDATE milestones SET
			name=$3, status=$4, weight_pct=$5, sort_order=$6,
			planned_date=COALESCE(NULLIF($7,'')::date, planned_date),
			actual_date=COALESCE(NULLIF($8,'')::date, actual_date),
			row_version=row_version+1
		WHERE id=$1 AND project_id=$2
		RETURNING `+milestoneCols,
		mid, pid, name, status, weight, order,
		strDeref(req.PlannedDate), strDeref(req.ActualDate)), &after)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "milestones", EntityID: mid.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"name": before.Name, "status": before.Status},
		After:  map[string]interface{}{"name": after.Name, "status": after.Status},
		IP:     m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"milestone": after})
}

func (h *Handler) DeleteMilestone(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	mid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE milestones SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, mid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Milestone bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "milestones", EntityID: mid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// yardımcılar
// ---------------------------------------------------------------------------

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", map[string]string{param: "geçersiz UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func isUniqueViolation(err error) bool { return sqlState(err) == "23505" }
func isFKViolation(err error) bool     { return sqlState(err) == "23503" }

func sqlState(err error) string {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState()
	}
	return ""
}
