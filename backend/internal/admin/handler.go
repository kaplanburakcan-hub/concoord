// Package admin — Admin Paneli v1 uçları: kullanıcı CRUD, rol atama (proje
// üyeliği), kullanıcı × izin matrisi override'ı ve denetim izi görüntüleyici.
// Tüm yazma işlemleri audit.Recorder ile kaydedilir (Plan §5.1).
package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
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
// Kullanıcılar
// ---------------------------------------------------------------------------

type userDTO struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	FullName    string     `json:"full_name"`
	Phone       *string    `json:"phone,omitempty"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	RowVersion  int        `json:"row_version"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, email, username, full_name, phone, is_active, last_login_at, row_version, created_at
		FROM users
		WHERE deleted_at IS NULL
		  AND ($1 = '' OR email ILIKE '%'||$1||'%' OR username ILIKE '%'||$1||'%' OR full_name ILIKE '%'||$1||'%')
		ORDER BY created_at DESC
		LIMIT 500`, q)
	if err != nil {
		h.log.Error("kullanıcı listesi", "err", err)
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []userDTO{}
	for rows.Next() {
		var u userDTO
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.FullName, &u.Phone,
			&u.IsActive, &u.LastLoginAt, &u.RowVersion, &u.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, u)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"users": out})
}

type createUserReq struct {
	Email    string  `json:"email"`
	Username string  `json:"username"`
	FullName string  `json:"full_name"`
	Phone    *string `json:"phone"`
	Password string  `json:"password"`
	IsActive *bool   `json:"is_active"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Username = strings.TrimSpace(req.Username)
	fields := map[string]string{}
	if !strings.Contains(req.Email, "@") {
		fields["email"] = "geçerli e-posta gerekli"
	}
	if req.Username == "" {
		fields["username"] = "zorunlu"
	}
	if strings.TrimSpace(req.FullName) == "" {
		fields["full_name"] = "zorunlu"
	}
	if len(req.Password) < 8 {
		fields["password"] = "en az 8 karakter"
	}
	if len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var u userDTO
	err = tx.QueryRow(r.Context(), `
		INSERT INTO users (email, username, password_hash, full_name, phone, is_active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, email, username, full_name, phone, is_active, last_login_at, row_version, created_at`,
		req.Email, req.Username, hash, strings.TrimSpace(req.FullName), req.Phone, active).
		Scan(&u.ID, &u.Email, &u.Username, &u.FullName, &u.Phone, &u.IsActive, &u.LastLoginAt, &u.RowVersion, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "E-posta veya kullanıcı adı zaten kayıtlı.", nil)
			return
		}
		h.log.Error("kullanıcı oluşturma", "err", err)
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "users", EntityID: u.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"email": u.Email, "username": u.Username, "is_active": u.IsActive},
		IP:    m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"user": u})
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var u userDTO
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, email, username, full_name, phone, is_active, last_login_at, row_version, created_at
		FROM users WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&u.ID, &u.Email, &u.Username, &u.FullName, &u.Phone, &u.IsActive, &u.LastLoginAt, &u.RowVersion, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kullanıcı bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"user": u})
}

type updateUserReq struct {
	FullName   *string `json:"full_name"`
	Phone      *string `json:"phone"`
	IsActive   *bool   `json:"is_active"`
	RowVersion int     `json:"row_version"` // optimistic locking
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateUserReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// row_version ile çakışma kontrolü + audit için before.
	var before userDTO
	err = tx.QueryRow(r.Context(), `
		SELECT id, email, username, full_name, phone, is_active, last_login_at, row_version, created_at
		FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).
		Scan(&before.ID, &before.Email, &before.Username, &before.FullName, &before.Phone,
			&before.IsActive, &before.LastLoginAt, &before.RowVersion, &before.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kullanıcı bulunamadı.", nil)
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
	full := before.FullName
	if req.FullName != nil {
		full = strings.TrimSpace(*req.FullName)
	}
	phone := before.Phone
	if req.Phone != nil {
		phone = req.Phone
	}
	active := before.IsActive
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var after userDTO
	err = tx.QueryRow(r.Context(), `
		UPDATE users SET full_name=$2, phone=$3, is_active=$4, row_version=row_version+1
		WHERE id=$1
		RETURNING id, email, username, full_name, phone, is_active, last_login_at, row_version, created_at`,
		id, full, phone, active).
		Scan(&after.ID, &after.Email, &after.Username, &after.FullName, &after.Phone,
			&after.IsActive, &after.LastLoginAt, &after.RowVersion, &after.CreatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "users", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"full_name": before.FullName, "is_active": before.IsActive},
		After:  map[string]interface{}{"full_name": after.FullName, "is_active": after.IsActive},
		IP:     m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"user": after})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if actor, ok := auth.UserIDFrom(r.Context()); ok && actor == id {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Kendinizi silemezsiniz.", nil)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())
	ct, err := tx.Exec(r.Context(),
		`UPDATE users SET deleted_at=now(), is_active=false WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kullanıcı bulunamadı.", nil)
		return
	}
	// Aktif oturumları da kapat.
	_, _ = tx.Exec(r.Context(), `UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, id)
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "users", EntityID: id.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Roller & izin sözlüğü (referans)
// ---------------------------------------------------------------------------

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `SELECT id, code, name FROM roles ORDER BY name`)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	type role struct {
		ID   uuid.UUID `json:"id"`
		Code string    `json:"code"`
		Name string    `json:"name"`
	}
	out := []role{}
	for rows.Next() {
		var x role
		if err := rows.Scan(&x.ID, &x.Code, &x.Name); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, x)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"roles": out})
}

func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	// Sözlük statik (rbac paketinden) — DB yerine tek kaynaktan servis edilir.
	type perm struct {
		Code, Module, Action, Description string
	}
	out := make([]map[string]string, 0, len(rbac.AllPermissions))
	for _, p := range rbac.AllPermissions {
		out = append(out, map[string]string{
			"code": p.Code(), "module": p.Module, "action": p.Action, "description": p.Description,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"permissions": out})
}

// ---------------------------------------------------------------------------
// Kullanıcı × izin matrisi (override yönetimi)
// ---------------------------------------------------------------------------

func (h *Handler) GetUserMatrix(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	projectID := auth.ProjectFromRequest(r)
	rows, err := h.eval.Matrix(r.Context(), id, projectID)
	if err != nil {
		h.log.Error("matris", "err", err)
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"user_id": id, "project_id": projectID, "matrix": rows})
}

type setOverrideReq struct {
	ProjectID *string `json:"project_id"` // null = global override
	Effect    *string `json:"effect"`     // "GRANT" | "DENY" | null(=temizle)
}

// SetOverride — kullanıcı × izin override'ını ayarlar/temizler. Etkisi bir
// sonraki API isteğinde ANINDA geçerlidir (Faz 1 kabul kriteri).
func (h *Handler) SetOverride(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	permCode := chi.URLParam(r, "code")
	var req setOverrideReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// İzin kodu doğrula.
	var permID uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `SELECT id FROM permissions WHERE code=$1`, permCode).Scan(&permID); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Bilinmeyen izin kodu.", map[string]string{"code": permCode})
		return
	}
	var projID *uuid.UUID
	if req.ProjectID != nil && *req.ProjectID != "" {
		p, err := uuid.Parse(*req.ProjectID)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"project_id": "geçersiz UUID"})
			return
		}
		projID = &p
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	m := audit.MetaFrom(r.Context())
	if req.Effect == nil || *req.Effect == "" {
		// Override temizle.
		_, err = tx.Exec(r.Context(), `
			DELETE FROM user_permissions
			WHERE user_id=$1 AND permission_id=$2 AND project_id IS NOT DISTINCT FROM $3`, id, permID, projID)
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
			ActorID: m.ActorID, Entity: "user_permissions", EntityID: id.String(), Action: audit.ActionDelete,
			Before: map[string]interface{}{"permission": permCode, "project_id": projID},
			IP:     m.IP, ReqID: m.ReqID,
		})
	} else {
		eff := strings.ToUpper(*req.Effect)
		if eff != rbac.EffectGrant && eff != rbac.EffectDeny {
			httpx.ValidationFailed(w, r, map[string]string{"effect": "GRANT | DENY | null olmalı"})
			return
		}
		_, err = tx.Exec(r.Context(), `
			INSERT INTO user_permissions (user_id, project_id, permission_id, effect)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (user_id, project_id, permission_id) DO UPDATE
			  SET effect=EXCLUDED.effect, row_version=user_permissions.row_version+1`,
			id, projID, permID, eff)
		if err != nil {
			httpx.Internal(w, r)
			return
		}
		_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
			ActorID: m.ActorID, Entity: "user_permissions", EntityID: id.String(), Action: audit.ActionUpdate,
			After: map[string]interface{}{"permission": permCode, "project_id": projID, "effect": eff},
			IP:    m.IP, ReqID: m.ReqID,
		})
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Proje üyeliği (rol atama). project_members üzerinden.
// ---------------------------------------------------------------------------

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT pm.id, pm.user_id, u.full_name, u.email, r.code, r.name, pm.subcontractor_id
		FROM project_members pm
		JOIN users u ON u.id = pm.user_id
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL
		ORDER BY u.full_name`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	type member struct {
		ID             uuid.UUID  `json:"id"`
		UserID         uuid.UUID  `json:"user_id"`
		FullName       string     `json:"full_name"`
		Email          string     `json:"email"`
		RoleCode       string     `json:"role_code"`
		RoleName       string     `json:"role_name"`
		SubcontractorID *uuid.UUID `json:"subcontractor_id,omitempty"`
	}
	out := []member{}
	for rows.Next() {
		var x member
		if err := rows.Scan(&x.ID, &x.UserID, &x.FullName, &x.Email, &x.RoleCode, &x.RoleName, &x.SubcontractorID); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, x)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"members": out})
}

type addMemberReq struct {
	UserID   string `json:"user_id"`
	RoleCode string `json:"role_code"`
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req addMemberReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	userID, err := uuid.Parse(strings.TrimSpace(req.UserID))
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"user_id": "geçersiz UUID"})
		return
	}
	var roleID uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `SELECT id FROM roles WHERE code=$1`, req.RoleCode).Scan(&roleID); err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"role_code": "bilinmeyen rol"})
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())
	var memberID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO project_members (project_id, user_id, role_id)
		VALUES ($1,$2,$3)
		ON CONFLICT (project_id, user_id) WHERE deleted_at IS NULL
		DO UPDATE SET role_id=EXCLUDED.role_id, row_version=project_members.row_version+1
		RETURNING id`, pid, userID, roleID).Scan(&memberID)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Kullanıcı veya proje bulunamadı.", nil)
			return
		}
		h.log.Error("üye ekleme", "err", err)
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "project_members", EntityID: memberID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid, "user_id": userID, "role": req.RoleCode},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"member_id": memberID})
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE project_members SET deleted_at=now() WHERE project_id=$1 AND user_id=$2 AND deleted_at IS NULL`, pid, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Üyelik bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "project_members", EntityID: uid.String(), Action: audit.ActionDelete,
		Before: map[string]interface{}{"project_id": pid, "user_id": uid}, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// ---------------------------------------------------------------------------
// Denetim izi görüntüleyici
// ---------------------------------------------------------------------------

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity")
	action := r.URL.Query().Get("action")
	actor := r.URL.Query().Get("actor_id")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT a.id, a.actor_id, u.full_name, a.entity, a.entity_id, a.action,
		       a.before, a.after, a.ip, a.request_id, a.at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_id
		WHERE ($1='' OR a.entity=$1)
		  AND ($2='' OR a.action=$2)
		  AND ($3='' OR a.actor_id=$3::uuid)
		ORDER BY a.at DESC
		LIMIT $4`, entity, action, actor, limit)
	if err != nil {
		h.log.Error("audit listesi", "err", err)
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	type entry struct {
		ID        uuid.UUID       `json:"id"`
		ActorID   *uuid.UUID      `json:"actor_id,omitempty"`
		ActorName *string         `json:"actor_name,omitempty"`
		Entity    string          `json:"entity"`
		EntityID  *uuid.UUID      `json:"entity_id,omitempty"`
		Action    string          `json:"action"`
		Before    map[string]any  `json:"before,omitempty"`
		After     map[string]any  `json:"after,omitempty"`
		IP        *string         `json:"ip,omitempty"`
		RequestID *string         `json:"request_id,omitempty"`
		At        time.Time       `json:"at"`
	}
	out := []entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Entity, &e.EntityID, &e.Action,
			&e.Before, &e.After, &e.IP, &e.RequestID, &e.At); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, e)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"logs": out})
}

// ---------------------------------------------------------------------------
// yardımcılar
// ---------------------------------------------------------------------------

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

