package rbac

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Decide — SAF karar fonksiyonu (DB'siz, tam test edilebilir).
// Girdi: uygulanabilir override etkileri + rol varsayılanı.
// Kural (Plan §4): önce DENY, sonra GRANT, sonra rol varsayılanı.
func Decide(effects []string, roleHasDefault bool) bool {
	grant := false
	for _, e := range effects {
		switch e {
		case EffectDeny:
			return false // DENY her şeyi ezer
		case EffectGrant:
			grant = true
		}
	}
	if grant {
		return true
	}
	return roleHasDefault
}

// Evaluator — DB destekli yetki motoru. Repository/middleware bunu kullanır.
type Evaluator struct {
	pool *pgxpool.Pool
}

func NewEvaluator(pool *pgxpool.Pool) *Evaluator {
	return &Evaluator{pool: pool}
}

// Can — (kullanıcı, proje, izin) üçlüsü için nihai kararı verir.
// projectID nil → proje bağımsız (global) kontrol: yalnızca global override'lar
// (project_id IS NULL) ve kullanıcının HERHANGİ bir projedeki rolünün varsayılanı
// dikkate alınır. Bu, admin.* gibi proje üstü ekranların çalışmasını sağlar.
func (e *Evaluator) Can(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, permCode string) (bool, error) {
	effects, err := e.overrides(ctx, userID, projectID, permCode)
	if err != nil {
		return false, err
	}
	roleHas, err := e.roleDefault(ctx, userID, projectID, permCode)
	if err != nil {
		return false, err
	}
	return Decide(effects, roleHas), nil
}

// overrides — uygulanabilir user_permissions etkilerini döner.
func (e *Evaluator) overrides(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, permCode string) ([]string, error) {
	rows, err := e.pool.Query(ctx, `
		SELECT up.effect
		FROM user_permissions up
		JOIN permissions p ON p.id = up.permission_id
		WHERE up.user_id = $1
		  AND p.code = $2
		  AND (up.project_id IS NULL OR up.project_id = $3)`,
		userID, permCode, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var eff string
		if err := rows.Scan(&eff); err != nil {
			return nil, err
		}
		out = append(out, eff)
	}
	return out, rows.Err()
}

// roleDefault — kullanıcının ilgili roldeki varsayılan iznini kontrol eder.
func (e *Evaluator) roleDefault(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, permCode string) (bool, error) {
	var exists bool
	if projectID != nil {
		err := e.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM project_members pm
				JOIN role_permissions rp ON rp.role_id = pm.role_id
				JOIN permissions p       ON p.id = rp.permission_id
				WHERE pm.user_id = $1 AND pm.project_id = $2
				  AND pm.deleted_at IS NULL AND p.code = $3
			)`, userID, *projectID, permCode).Scan(&exists)
		return exists, err
	}
	// Global kontrol: kullanıcının herhangi bir üyeliğindeki rol yeterli.
	err := e.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM project_members pm
			JOIN role_permissions rp ON rp.role_id = pm.role_id
			JOIN permissions p       ON p.id = rp.permission_id
			WHERE pm.user_id = $1 AND pm.deleted_at IS NULL AND p.code = $2
		)`, userID, permCode).Scan(&exists)
	return exists, err
}

// EffectivePermissions — kullanıcının (opsiyonel proje kapsamında) sahip olduğu
// tüm izin kodlarını döner. /auth/me ve frontend <Can> bunu tüketir.
func (e *Evaluator) EffectivePermissions(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) ([]string, error) {
	rows, err := e.pool.Query(ctx, `
		WITH perms AS (SELECT id, code FROM permissions),
		ov AS (
			SELECT p.code,
			       bool_or(up.effect = 'DENY')  AS denied,
			       bool_or(up.effect = 'GRANT') AS granted
			FROM user_permissions up
			JOIN perms p ON p.id = up.permission_id
			WHERE up.user_id = $1 AND (up.project_id IS NULL OR up.project_id = $2)
			GROUP BY p.code
		),
		roledef AS (
			SELECT DISTINCT p.code
			FROM project_members pm
			JOIN role_permissions rp ON rp.role_id = pm.role_id
			JOIN perms p ON p.id = rp.permission_id
			WHERE pm.user_id = $1 AND pm.deleted_at IS NULL
			  AND ($2::uuid IS NULL OR pm.project_id = $2)
		)
		SELECT p.code
		FROM perms p
		LEFT JOIN ov ON ov.code = p.code
		WHERE COALESCE(ov.denied, false) = false
		  AND (COALESCE(ov.granted, false) = true OR p.code IN (SELECT code FROM roledef))
		ORDER BY p.code`,
		userID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

// MatrixRow — admin izin matrisi ekranının bir satırı.
type MatrixRow struct {
	Code        string  `json:"code"`
	Module      string  `json:"module"`
	Action      string  `json:"action"`
	Description string  `json:"description"`
	RoleDefault bool    `json:"role_default"` // rolden gelen varsayılan
	Override    *string `json:"override"`     // "GRANT" | "DENY" | null
	Effective   bool    `json:"effective"`    // nihai karar
}

// Matrix — bir kullanıcı (+opsiyonel proje) için tüm izinlerin matris görünümü.
func (e *Evaluator) Matrix(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) ([]MatrixRow, error) {
	rows, err := e.pool.Query(ctx, `
		WITH perms AS (SELECT id, code, module, action, description FROM permissions),
		ov AS (
			SELECT permission_id,
			       bool_or(effect='DENY')  AS denied,
			       bool_or(effect='GRANT') AS granted
			FROM user_permissions
			WHERE user_id = $1 AND (project_id IS NULL OR project_id = $2)
			GROUP BY permission_id
		),
		roledef AS (
			SELECT DISTINCT rp.permission_id
			FROM project_members pm
			JOIN role_permissions rp ON rp.role_id = pm.role_id
			WHERE pm.user_id = $1 AND pm.deleted_at IS NULL
			  AND ($2::uuid IS NULL OR pm.project_id = $2)
		)
		SELECT p.code, p.module, p.action, COALESCE(p.description,''),
		       (rd.permission_id IS NOT NULL) AS role_default,
		       CASE WHEN COALESCE(ov.denied,false) THEN 'DENY'
		            WHEN COALESCE(ov.granted,false) THEN 'GRANT'
		            ELSE NULL END AS override
		FROM perms p
		LEFT JOIN ov ON ov.permission_id = p.id
		LEFT JOIN roledef rd ON rd.permission_id = p.id
		ORDER BY p.module, p.action`,
		userID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatrixRow
	for rows.Next() {
		var r MatrixRow
		if err := rows.Scan(&r.Code, &r.Module, &r.Action, &r.Description, &r.RoleDefault, &r.Override); err != nil {
			return nil, err
		}
		var effects []string
		if r.Override != nil {
			effects = []string{*r.Override}
		}
		r.Effective = Decide(effects, r.RoleDefault)
		out = append(out, r)
	}
	return out, rows.Err()
}
