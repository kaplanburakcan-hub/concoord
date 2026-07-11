package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// JWT (HS256) stdlib ile üretilir — access token için ek bağımlılık taşımadan.
// Refresh ve reset jetonları opak rastgele dizelerdir; DB'de yalnızca SHA-256
// özetleri saklanır (token.go içindeki HashToken).

type Claims struct {
	Sub string `json:"sub"` // user id
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Typ string `json:"typ"` // "access"
}

var (
	ErrTokenMalformed = errors.New("jeton biçimi hatalı")
	ErrTokenSignature = errors.New("jeton imzası geçersiz")
	ErrTokenExpired   = errors.New("jeton süresi dolmuş")
)

type TokenSigner struct {
	secret []byte
}

func NewTokenSigner(secret string) *TokenSigner {
	return &TokenSigner{secret: []byte(secret)}
}

var jwtHeader = base64URL([]byte(`{"alg":"HS256","typ":"JWT"}`))

// SignAccess — verilen kullanıcı için imzalı access token üretir.
func (s *TokenSigner) SignAccess(userID string, ttl time.Duration) (string, Claims, error) {
	now := time.Now()
	c := Claims{
		Sub: userID,
		Iat: now.Unix(),
		Exp: now.Add(ttl).Unix(),
		Typ: "access",
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", Claims{}, err
	}
	signingInput := jwtHeader + "." + base64URL(payload)
	sig := s.sign(signingInput)
	return signingInput + "." + sig, c, nil
}

// Parse — imza ve süre doğrulaması yapıp claim'leri döner.
func (s *TokenSigner) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenMalformed
	}
	signingInput := parts[0] + "." + parts[1]
	expected := s.sign(signingInput)
	// Sabit zamanlı imza karşılaştırması.
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, ErrTokenSignature
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrTokenMalformed
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return Claims{}, ErrTokenMalformed
	}
	if time.Now().Unix() >= c.Exp {
		return Claims{}, ErrTokenExpired
	}
	return c, nil
}

func (s *TokenSigner) sign(input string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// NewOpaqueToken — refresh/reset için kriptografik rastgele opak jeton.
func NewOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken — opak jetonun DB'de saklanacak SHA-256 özeti (ham jeton saklanmaz).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
