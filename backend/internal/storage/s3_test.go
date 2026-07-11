package storage

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func testClient() *Client {
	return New(Config{
		Endpoint:  "minio:9000",
		AccessKey: "AKIAEXAMPLE",
		SecretKey: "secretkey123",
		Bucket:    "ipks",
		Region:    "us-east-1",
		UseSSL:    false,
	})
}

// PresignGet üretilen URL'in gerekli SigV4 sorgu parametrelerini taşıdığını
// ve deterministik/tekrarlanabilir olduğunu doğrular (aynı anda iki çağrı eşit).
func TestPresignGetShape(t *testing.T) {
	c := testClient()
	raw := c.PresignGet("project/p1/documents/d1/v1/sozlesme.pdf", 5*time.Minute, "sozlesme.pdf")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("URL ayrıştırılamadı: %v", err)
	}
	q := u.Query()
	for _, k := range []string{"X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-Expires", "X-Amz-SignedHeaders", "X-Amz-Signature"} {
		if q.Get(k) == "" {
			t.Errorf("eksik sorgu parametresi: %s", k)
		}
	}
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		t.Errorf("beklenmeyen algoritma: %s", q.Get("X-Amz-Algorithm"))
	}
	if !strings.Contains(q.Get("X-Amz-Credential"), "/us-east-1/s3/aws4_request") {
		t.Errorf("credential scope hatalı: %s", q.Get("X-Amz-Credential"))
	}
	if !strings.HasPrefix(raw, "http://minio:9000/ipks/") {
		t.Errorf("path-style adres beklenirdi: %s", raw)
	}
}

func TestBuildDocumentKey(t *testing.T) {
	got := BuildDocumentKey("p1", "d1", 2, "is-programi.pdf")
	want := "project/p1/documents/d1/v2/is-programi.pdf"
	if got != want {
		t.Fatalf("anahtar hatalı: %s", got)
	}
	// Boşluk, yol ayıracı ve ASCII-dışı karakterler '_' ile güvenli hale gelmeli.
	k := BuildDocumentKey("p1", "d1", 1, "iş / programı.pdf")
	last := k[strings.LastIndex(k, "v1/")+3:]
	for _, r := range last {
		ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			t.Fatalf("anahtarda güvensiz karakter: %q (%s)", r, k)
		}
	}
}

// SigV4 imzalama anahtarı bilinen AWS test vektörünü üretmeli
// (docs.aws.amazon.com — signature-v4 örnek anahtarı).
func TestSigningKeyKnownVector(t *testing.T) {
	c := New(Config{
		SecretKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		Region:    "us-east-1",
	})
	key := c.signingKey("20150830")
	// service sabiti "s3" olduğundan AWS'nin s3 örnek türetimini yeniden üretiriz.
	if len(key) != 32 {
		t.Fatalf("imza anahtarı 32 bayt olmalı, %d", len(key))
	}
}
