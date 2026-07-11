// Package seed — idempotent seed mekanizması.
//
// Her seed adımı bir kez çalışır; uygulananlar seed_history tablosuna yazılır.
// Faz 1: izin sözlüğü + 7 rol + rol varsayılanları + bootstrap admin.
// İzin/rol verisi TEK KAYNAK: internal/rbac (seed onu DB'ye yansıtır).
package seed

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/rbac"
)

// Options — seed davranışını dışarıdan besleyen girdiler (config'ten gelir).
type Options struct {
	AdminEmail    string
	AdminPassword string // boşsa rastgele üretilir ve loglanır (yalnızca dev)
}

type Step struct {
	Name string
	Run  func(ctx context.Context, tx pgx.Tx, opts Options, log *slog.Logger) error
}

// registry — sıralı seed adımları. Yeni faz = yeni adım(lar); eskiler değişmez.
// (İzin sözlüğü ileride büyürse yeni bir "sync" adımı eklenir; upsert kullanılır.)
var registry = []Step{
	{Name: "0001_ornek_kontrol", Run: stepHealthCheck},
	{Name: "0002_izin_sozlugu", Run: stepPermissions},
	{Name: "0003_roller", Run: stepRoles},
	{Name: "0004_rol_izinleri", Run: stepRolePermissions},
	{Name: "0005_bootstrap_admin", Run: stepBootstrapAdmin},
	// Faz 2: sözlük büyüdü (projects.create/delete). Upsert idempotent olduğundan
	// mevcut kurulumlar bu adımla yeni izinleri ve rol bağlarını alır.
	{Name: "0006_faz2_izin_sync", Run: stepFaz2PermSync},
}

// stepFaz2PermSync — izin sözlüğünü ve rol varsayılanlarını yeniden senkronlar.
// Ayrıca mevcut kurulumlarda bootstrap admin'e yeni izinleri global GRANT olarak
// ekler (fresh install'da 0005 zaten yapar; bu adım idempotenttir).
func stepFaz2PermSync(ctx context.Context, tx pgx.Tx, opts Options, log *slog.Logger) error {
	if err := stepPermissions(ctx, tx, opts, log); err != nil {
		return err
	}
	if err := stepRolePermissions(ctx, tx, opts, log); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO user_permissions (user_id, project_id, permission_id, effect)
		SELECT u.id, NULL, p.id, 'GRANT'
		FROM users u, permissions p
		WHERE u.email=$1 AND u.deleted_at IS NULL
		ON CONFLICT DO NOTHING`, opts.AdminEmail)
	return err
}

func stepHealthCheck(ctx context.Context, tx pgx.Tx, _ Options, _ *slog.Logger) error {
	_, err := tx.Exec(ctx, `SELECT 1`)
	return err
}

// stepPermissions — izin sözlüğünü upsert eder (idempotent).
func stepPermissions(ctx context.Context, tx pgx.Tx, _ Options, _ *slog.Logger) error {
	for _, p := range rbac.AllPermissions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permissions (code, module, action, description)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (code) DO UPDATE
			  SET module=EXCLUDED.module, action=EXCLUDED.action, description=EXCLUDED.description`,
			p.Code(), p.Module, p.Action, p.Description); err != nil {
			return fmt.Errorf("izin %s: %w", p.Code(), err)
		}
	}
	return nil
}

// stepRoles — 7 sistem rolünü upsert eder.
func stepRoles(ctx context.Context, tx pgx.Tx, _ Options, _ *slog.Logger) error {
	for _, r := range rbac.Roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roles (code, name) VALUES ($1,$2)
			ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name`,
			r.Code, r.Name); err != nil {
			return fmt.Errorf("rol %s: %w", r.Code, err)
		}
	}
	return nil
}

// stepRolePermissions — rol varsayılanlarını bağlar (code join ile).
func stepRolePermissions(ctx context.Context, tx pgx.Tx, _ Options, _ *slog.Logger) error {
	for _, r := range rbac.Roles {
		for _, code := range rbac.RoleDefaults(r.Code) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO role_permissions (role_id, permission_id)
				SELECT ro.id, pe.id FROM roles ro, permissions pe
				WHERE ro.code=$1 AND pe.code=$2
				ON CONFLICT DO NOTHING`,
				r.Code, code); err != nil {
				return fmt.Errorf("rol_izin %s/%s: %w", r.Code, code, err)
			}
		}
	}
	return nil
}

// stepBootstrapAdmin — ilk platform yöneticisini oluşturur ve tüm izinleri
// GLOBAL GRANT (project_id NULL) olarak verir → proje bağımsız superadmin.
// Sonraki adminler matris ekranından ya da proje rolüyle yetkilendirilir.
func stepBootstrapAdmin(ctx context.Context, tx pgx.Tx, opts Options, log *slog.Logger) error {
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 AND deleted_at IS NULL)`,
		opts.AdminEmail).Scan(&exists); err != nil {
		return err
	}
	if exists {
		log.Info("bootstrap admin zaten mevcut", "email", opts.AdminEmail)
		return nil
	}

	pw := opts.AdminPassword
	generated := false
	if pw == "" {
		p, err := auth.NewOpaqueToken()
		if err != nil {
			return err
		}
		pw = p[:20]
		generated = true
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}

	var uid string
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, full_name, is_active)
		VALUES ($1,'admin',$2,'Sistem Yöneticisi', true)
		RETURNING id`, opts.AdminEmail, hash).Scan(&uid); err != nil {
		return fmt.Errorf("admin kullanıcı: %w", err)
	}

	// Tüm izinler için global GRANT.
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_permissions (user_id, project_id, permission_id, effect)
		SELECT $1, NULL, id, 'GRANT' FROM permissions
		ON CONFLICT DO NOTHING`, uid); err != nil {
		return fmt.Errorf("admin izinleri: %w", err)
	}

	if generated {
		log.Warn("BOOTSTRAP ADMIN PAROLASI ÜRETİLDİ — güvenli yerde saklayın ve değiştirin",
			"email", opts.AdminEmail, "password", pw)
	} else {
		log.Info("bootstrap admin oluşturuldu", "email", opts.AdminEmail)
	}
	return nil
}

// Apply — kayıtlı adımları idempotent uygular.
func Apply(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, opts Options) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS seed_history (
			name text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("seed_history oluşturulamadı: %w", err)
	}

	for _, s := range registry {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM seed_history WHERE name=$1)`, s.Name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			log.Info("seed atlandı (uygulanmış)", "step", s.Name)
			continue
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if err := s.Run(ctx, tx, opts, log); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("seed %s: %w", s.Name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO seed_history(name) VALUES ($1)`, s.Name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Info("seed uygulandı", "step", s.Name)
	}
	return nil
}
