package attendance

import (
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Kabul kriteri 7 — "attendance.view_location izni olmayan kullanıcının
// aldığı yanıtta lat/lng alanları yok." İzin değerlendirmesinin kendisi
// (kim neye sahip) RBAC katmanının işi (bkz. internal/rbac/engine_test.go);
// bu test, maskeleme MEKANİZMASININ (maskLocation) gerçekten tüm konum
// alanlarını temizlediğini ve konumla İLGİSİZ alanlara (ör. geofence_ok,
// bir uyarı bayrağı — kendisi koordinat içermez) DOKUNMADIĞINI doğrular.
// writeLocationAwareJSON bu metodu izin yoksa her DTO için çağırır (bkz.
// serialize.go) — uçtan uca izin kontrolü + maskeleme akışı Adım 3'te
// canlı olarak (DENY override ile) doğrulandı.
// ---------------------------------------------------------------------------

func TestEventDTOMaskLocation(t *testing.T) {
	geofenceOK := true
	e := eventDTO{
		ID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		PersonID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		EventType:  "in",
		Source:     "qr",
		Lat:        f(40.21),
		Lng:        f(29.06),
		AccuracyM:  f(12.5),
		DistanceM:  f(44.5),
		GeofenceOK: &geofenceOK,
	}

	e.maskLocation()

	if e.Lat != nil || e.Lng != nil || e.AccuracyM != nil || e.DistanceM != nil {
		t.Fatalf("maskLocation() sonrası konum alanları nil olmalı, geldi: %+v", e)
	}
	if e.GeofenceOK == nil || !*e.GeofenceOK {
		t.Fatalf("maskLocation() konumla ilgisiz geofence_ok alanına DOKUNMAMALI, geldi: %v", e.GeofenceOK)
	}
	if e.EventType != "in" || e.Source != "qr" {
		t.Fatalf("maskLocation() konum dışı alanları bozmamalı, geldi: %+v", e)
	}
}

// locationMasker arayüzünü gerçekten uyguladığını (derleme zamanı garantisi
// gibi çalışan basit bir çalışma zamanı kontrolü) doğrular — writeLocationAwareJSON
// bu arayüz üzerinden çağırır.
func TestEventDTOImplementsLocationMasker(t *testing.T) {
	var _ locationMasker = &eventDTO{}
}
