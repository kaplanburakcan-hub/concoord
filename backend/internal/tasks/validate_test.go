package tasks

import (
	"strings"
	"testing"
)

func TestValidStatus(t *testing.T) {
	for _, s := range StatusOrder {
		if !ValidStatus(s) {
			t.Fatalf("%s geçerli olmalı", s)
		}
	}
	for _, s := range []string{"", "done", "Bitti", "Archived"} {
		if ValidStatus(s) {
			t.Fatalf("%q geçersiz olmalı", s)
		}
	}
}

func TestValidPriority(t *testing.T) {
	for _, p := range []string{"Low", "Normal", "High", "Urgent"} {
		if !ValidPriority(p) {
			t.Fatalf("%s geçerli olmalı", p)
		}
	}
	if ValidPriority("Kritik") {
		t.Fatal("Kritik geçersiz olmalı")
	}
}

func TestValidateTitle(t *testing.T) {
	if _, ok := ValidateTitle("   "); ok {
		t.Fatal("boş başlık reddedilmeli")
	}
	if got, ok := ValidateTitle("  Kalıp söküm kontrolü  "); !ok || got != "Kalıp söküm kontrolü" {
		t.Fatalf("trim beklendi, got=%q ok=%v", got, ok)
	}
	if _, ok := ValidateTitle(strings.Repeat("a", 301)); ok {
		t.Fatal("300 karakter üstü reddedilmeli")
	}
	// Türkçe karakterler rune bazında sayılır (byte değil).
	if _, ok := ValidateTitle(strings.Repeat("ş", 300)); !ok {
		t.Fatal("300 rune'luk Türkçe başlık kabul edilmeli")
	}
}

func TestValidateCommentBody(t *testing.T) {
	if _, ok := ValidateCommentBody(""); ok {
		t.Fatal("boş yorum reddedilmeli")
	}
	if _, ok := ValidateCommentBody(strings.Repeat("y", 4001)); ok {
		t.Fatal("4000 karakter üstü reddedilmeli")
	}
	if got, ok := ValidateCommentBody(" @ahmet bakar mısın "); !ok || got != "@ahmet bakar mısın" {
		t.Fatalf("trim beklendi, got=%q", got)
	}
}
