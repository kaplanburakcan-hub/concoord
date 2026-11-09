package documents

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDriveListFolder(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Token uç noktası: JWT imzasını doğrular ve jeton döner.
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("form: %v", err)
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type yanlış: %q", r.Form.Get("grant_type"))
		}
		parts := strings.Split(r.Form.Get("assertion"), ".")
		if len(parts) != 3 {
			t.Fatalf("JWT üç parça olmalı, %d geldi", len(parts))
		}
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
			t.Errorf("JWT imzası doğrulanamadı: %v", err)
		}
		claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
		if !strings.Contains(string(claims), driveScope) {
			t.Errorf("scope eksik: %s", claims)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"tok-123","expires_in":3600}`))
	})

	// Dosya uç noktası: sayfalama ve yetki başlığını kontrol eder.
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-123" {
			http.Error(w, "yetkisiz", http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.URL.Query().Get("q"), "'folder-1' in parents") {
			t.Errorf("sorgu klasörü içermiyor: %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			w.Write([]byte(`{"nextPageToken":"p2","files":[{"id":"f1","name":"A-101 Plan.pdf","modifiedTime":"2026-11-01T10:00:00.000Z","webViewLink":"https://drive.google.com/file/d/f1/view"}]}`))
			return
		}
		w.Write([]byte(`{"files":[{"id":"f2","name":"B-201 Kesit.dwg","modifiedTime":"2026-11-02T10:00:00.000Z","webViewLink":"https://drive.google.com/file/d/f2/view"}]}`))
	})

	raw, _ := json.Marshal(map[string]string{
		"client_email": "ipks-reader@test.iam.gserviceaccount.com",
		"private_key":  string(keyPEM),
		"token_uri":    srv.URL + "/token",
	})

	h := &Handler{}
	if err := h.ConfigureDrive(string(raw), "folder-1"); err != nil {
		t.Fatal(err)
	}
	h.drive.filesURL = srv.URL + "/files"

	recs, err := h.driveDrawingRecords(context.Background(), "Drawing")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("iki kayıt bekleniyordu, %d geldi", len(recs))
	}
	if recs[0].Title != "A-101 Plan.pdf" || recs[0].DocCategory != "Drawing" || recs[0].Source != "drive" {
		t.Errorf("ilk kayıt yanlış: %+v", recs[0])
	}
	if recs[0].ID == recs[1].ID {
		t.Error("farklı dosyalar aynı kimliği almamalı")
	}

	// Başka bir kategoride Drive'a hiç gidilmemeli.
	none, err := h.driveDrawingRecords(context.Background(), "Contract")
	if err != nil || none != nil {
		t.Errorf("Contract kategorisinde Drive sonucu dönmemeli: %v %v", none, err)
	}
}

func TestConfigureDriveDisabledWithoutFolder(t *testing.T) {
	h := &Handler{}
	if err := h.ConfigureDrive("", ""); err != nil || h.drive != nil {
		t.Fatalf("klasör yoksa özellik kapalı kalmalı: %v", err)
	}
	if err := h.ConfigureDrive("{}", "bad id!"); err == nil {
		t.Error("geçersiz klasör kimliği reddedilmeli")
	}
}
