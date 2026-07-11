package auth

import (
	"strings"
	"testing"
	"time"
)

func TestSignAndParseRoundtrip(t *testing.T) {
	s := NewTokenSigner("test-secret")
	tok, claims, err := s.SignAccess("user-123", time.Minute)
	if err != nil {
		t.Fatalf("imzalama hatası: %v", err)
	}
	got, err := s.Parse(tok)
	if err != nil {
		t.Fatalf("parse hatası: %v", err)
	}
	if got.Sub != "user-123" {
		t.Fatalf("sub beklenen user-123, gelen %q", got.Sub)
	}
	if got.Exp != claims.Exp {
		t.Fatalf("exp tutarsız")
	}
}

func TestParseRejectsTamperedSignature(t *testing.T) {
	s := NewTokenSigner("test-secret")
	tok, _, _ := s.SignAccess("user-123", time.Minute)
	// Farklı sırla imzalanmış jeton kabul edilmemeli.
	other := NewTokenSigner("baska-sir")
	if _, err := other.Parse(tok); err != ErrTokenSignature {
		t.Fatalf("imza reddi beklendi, gelen %v", err)
	}
}

func TestParseRejectsExpired(t *testing.T) {
	s := NewTokenSigner("test-secret")
	tok, _, _ := s.SignAccess("user-123", -time.Second) // geçmişte
	if _, err := s.Parse(tok); err != ErrTokenExpired {
		t.Fatalf("süre dolumu beklendi, gelen %v", err)
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	s := NewTokenSigner("test-secret")
	if _, err := s.Parse("sadece.iki"); err != ErrTokenMalformed {
		t.Fatalf("biçim hatası beklendi, gelen %v", err)
	}
}

func TestHashTokenStable(t *testing.T) {
	if HashToken("abc") != HashToken("abc") {
		t.Fatal("aynı jeton için özet kararlı olmalı")
	}
	if HashToken("abc") == HashToken("abd") {
		t.Fatal("farklı jetonlar farklı özet vermeli")
	}
	if strings.Contains(HashToken("secret"), "secret") {
		t.Fatal("özet ham jetonu içermemeli")
	}
}
