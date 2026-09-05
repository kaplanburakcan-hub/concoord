package attendance

import (
	"testing"
	"time"
)

func f(v float64) *float64 { return &v }

// ---------------------------------------------------------------------------
// Kabul kriteri 6 — "Geofence dışından atılan kayıt reddedilmiyor,
// işaretleniyor." Bu test evaluateGeofence'in DOĞRU işaretlediğini
// doğrular; reddetmeme davranışı CreateEvents'te satırın HER KOŞULDA
// eklenmesiyle sağlanır (bkz. events.go — evaluateGeofence çağrısı bir
// hata/erken-dönüş yolunda değil, INSERT'ten önce sıradan bir alan
// hesabıdır).
// ---------------------------------------------------------------------------

func TestEvaluateGeofence(t *testing.T) {
	cases := []struct {
		name      string
		distanceM *float64
		radiusM   int
		accuracyM *float64
		want      bool
	}{
		{"sınır içinde, doğruluk iyi", f(50), 200, f(12.5), true},
		{"tam sınırda (eşitlik dahil)", f(200), 200, nil, true},
		{"sınır dışında — işaretlenir, reddedilmez", f(1399), 200, f(15), false},
		{"sınır içinde ama doğruluk kötü (>100m)", f(50), 200, f(150), false},
		{"doğruluk tam 100m — sınırda kabul", f(50), 200, f(100), true},
		{"konum hiç yok (izin verilmedi)", nil, 200, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evaluateGeofence(c.distanceM, c.radiusM, c.accuracyM); got != c.want {
				t.Fatalf("evaluateGeofence(%v,%v,%v)=%v, beklenen %v", c.distanceM, c.radiusM, c.accuracyM, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Kabul kriteri 8'i mümkün kılan tasarım kararı — token'ın penceresi
// SUNUCUNUN o anki saatiyle değil, olayın kendi captured_at'iyle
// karşılaştırılır. Bu, uçak modunda geç senkronize olan kayıtların (token
// çoktan süresi dolmuş olsa bile) captured_at token'ın ORİJİNAL penceresine
// düşüyorsa kabul edilmesini sağlar.
// ---------------------------------------------------------------------------

func TestWithinTokenWindow(t *testing.T) {
	issued := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	expires := issued.Add(60 * time.Second)

	cases := []struct {
		name       string
		capturedAt time.Time
		want       bool
	}{
		{"pencerenin tam ortası", issued.Add(30 * time.Second), true},
		{"tam issued_at anında", issued, true},
		{"tam expires_at anında", expires, true},
		{"issued_at'ten hemen önce ama tolerans içinde (kiosk/telefon saat kayması)", issued.Add(-3 * time.Second), true},
		{"expires_at'ten hemen sonra ama tolerans içinde", expires.Add(3 * time.Second), true},
		{"tolerans dışında, çok erken", issued.Add(-10 * time.Second), false},
		{"tolerans dışında, çok geç (uçak modunda saatler sonra senkron — token gerçekten eski)", expires.Add(2 * time.Hour), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := withinTokenWindow(c.capturedAt, issued, expires); got != c.want {
				t.Fatalf("withinTokenWindow(%v)=%v, beklenen %v", c.capturedAt, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// deriveDayHours — attendance_days türetme mantığı. "Sıralama bozuksa ...
// kaydı al, status='derived' olarak bırak" kuralının saf karşılığı:
// malformed=true olduğunda hours nil kalır (ham veri hiçbir zaman silinmez,
// bu fonksiyon zaten ham veriye dokunmaz — yalnızca ne hesaplanacağına karar
// verir).
// ---------------------------------------------------------------------------

func TestDeriveDayHours(t *testing.T) {
	d := func(hh, mm int) time.Time { return time.Date(2026, 1, 1, hh, mm, 0, 0, time.UTC) }

	cases := []struct {
		name          string
		events        []rawEvent
		wantHours     *float64
		wantMalformed bool
	}{
		{
			name:      "olay yok",
			events:    nil,
			wantHours: nil,
		},
		{
			name: "temiz tek çift: 08:00 in, 17:00 out = 9 saat",
			events: []rawEvent{
				{Type: "in", At: d(8, 0)},
				{Type: "out", At: d(17, 0)},
			},
			wantHours: f(9),
		},
		{
			name: "iki temiz çift (öğle molası ayrı giriş/çıkışla)",
			events: []rawEvent{
				{Type: "in", At: d(8, 0)},
				{Type: "out", At: d(12, 0)},
				{Type: "in", At: d(13, 0)},
				{Type: "out", At: d(17, 0)},
			},
			wantHours: f(8),
		},
		{
			name: "iki ardışık 'in' — bozuk, saat NULL",
			events: []rawEvent{
				{Type: "in", At: d(8, 0)},
				{Type: "in", At: d(9, 0)},
				{Type: "out", At: d(17, 0)},
			},
			wantMalformed: true,
		},
		{
			name: "sahipsiz 'out' — bozuk, saat NULL",
			events: []rawEvent{
				{Type: "out", At: d(17, 0)},
			},
			wantMalformed: true,
		},
		{
			name: "kapanmamış son 'in' (henüz çıkış yapmadı) — saat NULL",
			events: []rawEvent{
				{Type: "in", At: d(8, 0)},
			},
			wantMalformed: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hours, malformed := deriveDayHours(c.events)
			if malformed != c.wantMalformed {
				t.Fatalf("malformed=%v, beklenen %v", malformed, c.wantMalformed)
			}
			if c.wantMalformed {
				if hours != nil {
					t.Fatalf("malformed=true iken hours nil olmalı, geldi: %v", *hours)
				}
				return
			}
			if c.wantHours == nil {
				if hours != nil {
					t.Fatalf("hours nil olmalı, geldi: %v", *hours)
				}
				return
			}
			if hours == nil || *hours != *c.wantHours {
				t.Fatalf("hours=%v, beklenen %v", hours, *c.wantHours)
			}
		})
	}
}
