package attendance

import (
	"math"
	"testing"
)

func TestHaversineMeters(t *testing.T) {
	cases := []struct {
		name             string
		lat1, lng1       float64
		lat2, lng2       float64
		wantApproxMeters float64
		tolerance        float64
	}{
		{"aynı nokta", 40.21, 29.06, 40.21, 29.06, 0, 0.5},
		// 1 derece enlem farkı ~111.19 km'ye karşılık gelir.
		{"1 derece enlem farkı", 40.0, 29.0, 41.0, 29.0, 111195, 500},
		// Smoke test'te (Adım 2) doğrulanan gerçek koordinat çifti.
		{"kısa mesafe — smoke test verisi", 40.2104, 29.0600, 40.2100, 29.0600, 44.48, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := haversineMeters(c.lat1, c.lng1, c.lat2, c.lng2)
			if diff := math.Abs(got - c.wantApproxMeters); diff > c.tolerance {
				t.Fatalf("haversineMeters=%v, beklenen ~%v (±%v)", got, c.wantApproxMeters, c.tolerance)
			}
		})
	}
}
