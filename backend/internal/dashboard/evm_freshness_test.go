package dashboard

import "testing"

func TestDataFreshness(t *testing.T) {
	cases := []struct {
		name      string
		last, as  string
		wantLevel string
		wantMonth int
	}{
		{"hakediş yok", "", "2026-11", FreshnessYok, -1},
		{"aynı ay", "2026-11", "2026-11", FreshnessGuncel, 0},
		{"bir ay önce", "2026-10", "2026-11", FreshnessGuncel, 1},
		{"iki ay önce", "2026-09", "2026-11", FreshnessGecikmeli, 2},
		{"üç ay önce", "2026-08", "2026-11", FreshnessGecikmeli, 3},
		{"dört ay önce", "2026-07", "2026-11", FreshnessEski, 4},
		{"yıl sınırı", "2025-12", "2026-02", FreshnessGecikmeli, 2},
		{"gelecek ay bozuk veri", "2026-12", "2026-11", FreshnessGuncel, 0},
		{"bozuk format", "2026/11", "2026-11", FreshnessYok, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			level, months := DataFreshness(c.last, c.as)
			if level != c.wantLevel || months != c.wantMonth {
				t.Fatalf("DataFreshness(%q,%q) = (%q,%d), want (%q,%d)",
					c.last, c.as, level, months, c.wantLevel, c.wantMonth)
			}
		})
	}
}
