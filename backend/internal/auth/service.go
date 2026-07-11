package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("e-posta veya parola hatalı")
	ErrUserInactive       = errors.New("kullanıcı pasif")
	ErrInvalidToken       = errors.New("jeton geçersiz veya süresi dolmuş")
)

// User — servis dışına dönen güvenli kullanıcı görünümü (parola özeti yok).
type User struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	FullName    string     `json:"full_name"`
	Phone       *string    `json:"phone,omitempty"`
	IsActive    bool       `json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"` // access token son kullanma
}

type Service struct {
	pool       *pgxpool.Pool
	signer     *TokenSigner
	accessTTL  time.Duration
	refreshTTL time.Duration
	resetTTL   time.Duration
}

func NewService(pool *pgxpool.Pool, signer *TokenSigner, access, refresh, reset time.Duration) *Service {
	return &Service{pool: pool, signer: signer, accessTTL: access, refreshTTL: refresh, resetTTL: reset}
}

// Login — e-posta VEYA kullanıcı adı + parola ile giriş.
func (s *Service) Login(ctx context.Context, identifier, password, ua, ip string) (*User, *TokenPair, error) {
	var (
		u    User
		hash string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, username, full_name, phone, is_active, last_login_at, password_hash
		FROM users
		WHERE (email = $1 OR username = $1) AND deleted_at IS NULL`, identifier).
		Scan(&u.ID, &u.Email, &u.Username, &u.FullName, &u.Phone, &u.IsActive, &u.LastLoginAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		// Zamanlama sızıntısını azaltmak için yine de bir doğrulama yap.
		_, _ = VerifyPassword(password, "$argon2id$v=19$m=65536,t=2,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil || !ok {
		return nil, nil, ErrInvalidCredentials
	}
	if !u.IsActive {
		return nil, nil, ErrUserInactive
	}

	pair, err := s.issuePair(ctx, u.ID, ua, ip)
	if err != nil {
		return nil, nil, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, u.ID)
	return &u, pair, nil
}

// Refresh — geçerli refresh token'ı döndürür (rotasyon: eski iptal, yeni verilir).
func (s *Service) Refresh(ctx context.Context, refreshToken, ua, ip string) (*TokenPair, error) {
	h := HashToken(refreshToken)
	var (
		id     uuid.UUID
		userID uuid.UUID
		exp    time.Time
		active bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT rt.id, rt.user_id, rt.expires_at, u.is_active
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id AND u.deleted_at IS NULL
		WHERE rt.token_hash = $1 AND rt.revoked_at IS NULL`, h).
		Scan(&id, &userID, &exp, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(exp) {
		return nil, ErrInvalidToken
	}
	if !active {
		return nil, ErrUserInactive
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, id); err != nil {
		return nil, err
	}
	pair, err := s.issuePairTx(ctx, tx, userID, ua, ip)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return pair, nil
}

// Logout — verilen refresh token'ı iptal eder (stateless access token süresini bekler).
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		HashToken(refreshToken))
	return err
}

// ChangePassword — kimliği doğrulanmış kullanıcı kendi parolasını değiştirir.
// Tüm refresh oturumları iptal edilir.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	var hash string
	if err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&hash); err != nil {
		return err
	}
	ok, err := VerifyPassword(current, hash)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}
	newHash, err := HashPassword(next)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE users SET password_hash=$2, row_version=row_version+1 WHERE id=$1`, userID, newHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ForgotPassword — e-postaya karşılık gelen kullanıcı varsa sıfırlama jetonu
// üretir ve DÖNER (ham jeton yalnızca çağırana verilir; DB'de özeti saklanır).
// Kullanıcı yoksa boş jeton döner — sızıntı yapılmaz. E-posta gönderimi Faz 4.
func (s *Service) ForgotPassword(ctx context.Context, email string) (rawToken string, err error) {
	var userID uuid.UUID
	e := s.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE email=$1 AND deleted_at IS NULL AND is_active`, email).Scan(&userID)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", nil // sessiz: e-posta varlığı ifşa edilmez
	}
	if e != nil {
		return "", e
	}
	raw, err := NewOpaqueToken()
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, HashToken(raw), time.Now().Add(s.resetTTL))
	if err != nil {
		return "", err
	}
	return raw, nil
}

// ResetPassword — geçerli sıfırlama jetonuyla yeni parola belirler.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	h := HashToken(rawToken)
	var (
		id     uuid.UUID
		userID uuid.UUID
		exp    time.Time
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at FROM password_reset_tokens
		WHERE token_hash=$1 AND used_at IS NULL`, h).Scan(&id, &userID, &exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if time.Now().After(exp) {
		return ErrInvalidToken
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE password_reset_tokens SET used_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET password_hash=$2, row_version=row_version+1 WHERE id=$1`, userID, newHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetUser — id ile güvenli kullanıcı görünümü.
func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, username, full_name, phone, is_active, last_login_at
		FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).
		Scan(&u.ID, &u.Email, &u.Username, &u.FullName, &u.Phone, &u.IsActive, &u.LastLoginAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// --- iç yardımcılar ---

func (s *Service) issuePair(ctx context.Context, userID uuid.UUID, ua, ip string) (*TokenPair, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	pair, err := s.issuePairTx(ctx, tx, userID, ua, ip)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return pair, nil
}

func (s *Service) issuePairTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, ua, ip string) (*TokenPair, error) {
	access, claims, err := s.signer.SignAccess(userID.String(), s.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := NewOpaqueToken()
	if err != nil {
		return nil, err
	}
	exp := time.Now().Add(s.refreshTTL)
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1,$2,$3,$4,$5)`,
		userID, HashToken(refresh), exp, nullStr(ua), nullStr(ip)); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    time.Unix(claims.Exp, 0),
	}, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
