package projects

import "testing"

func TestValidateProject(t *testing.T) {
	if f := ValidateProject("KMP-01", "Kampüs", "TRY", "Planning"); len(f) != 0 {
		t.Fatalf("geçerli proje reddedildi: %v", f)
	}
	f := ValidateProject("", "", "TL", "Bogus")
	for _, k := range []string{"code", "name", "currency", "status"} {
		if _, ok := f[k]; !ok {
			t.Errorf("beklenen alan hatası yok: %s (%v)", k, f)
		}
	}
}

func TestValidateMilestone(t *testing.T) {
	w := 50.0
	if f := ValidateMilestone("Temel", "Planned", &w); len(f) != 0 {
		t.Fatalf("geçerli milestone reddedildi: %v", f)
	}
	bad := 140.0
	f := ValidateMilestone("", "Nope", &bad)
	for _, k := range []string{"name", "status", "weight_pct"} {
		if _, ok := f[k]; !ok {
			t.Errorf("beklenen alan hatası yok: %s (%v)", k, f)
		}
	}
	// weight_pct nil → hata olmamalı (opsiyonel).
	if f := ValidateMilestone("Kaba", "InProgress", nil); len(f) != 0 {
		t.Fatalf("nil ağırlık reddedildi: %v", f)
	}
}
