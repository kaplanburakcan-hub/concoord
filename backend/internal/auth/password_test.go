package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("Gizli!Parola.123")
	if err != nil {
		t.Fatalf("hash hatası: %v", err)
	}
	ok, err := VerifyPassword("Gizli!Parola.123", hash)
	if err != nil {
		t.Fatalf("verify hatası: %v", err)
	}
	if !ok {
		t.Fatal("doğru parola eşleşmeliydi")
	}
	bad, err := VerifyPassword("yanlis", hash)
	if err != nil {
		t.Fatalf("verify hatası: %v", err)
	}
	if bad {
		t.Fatal("yanlış parola eşleşmemeliydi")
	}
}

func TestVerifyRejectsBadHashFormat(t *testing.T) {
	if _, err := VerifyPassword("x", "duz-metin-hash-degil"); err != ErrInvalidHash {
		t.Fatalf("format hatası beklendi, gelen %v", err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	h1, _ := HashPassword("ayni")
	h2, _ := HashPassword("ayni")
	if h1 == h2 {
		t.Fatal("aynı parola farklı tuzla farklı özet vermeli")
	}
}
