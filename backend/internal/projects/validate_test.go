package projects

import "testing"

func TestValidateProject(t *testing.T) {
	if f := ValidateProject("KMP-01", "Kampüs", "TRY", "Planning", ActiveExtra{}); len(f) != 0 {
		t.Fatalf("geçerli proje reddedildi: %v", f)
	}
	f := ValidateProject("", "", "TL", "Bogus", ActiveExtra{})
	for _, k := range []string{"code", "name", "currency", "status"} {
		if _, ok := f[k]; !ok {
			t.Errorf("beklenen alan hatası yok: %s (%v)", k, f)
		}
	}
}

func TestValidateProjectActiveExtraRequired(t *testing.T) {
	f := ValidateProject("KMP-01", "Kampüs", "TRY", "Active", ActiveExtra{})
	for _, k := range []string{"site_handover_date", "client_rep_name", "site_manager_name"} {
		if _, ok := f[k]; !ok {
			t.Errorf("Active statüde beklenen zorunlu alan hatası yok: %s (%v)", k, f)
		}
	}
	extra := ActiveExtra{SiteHandoverDate: "2026-01-15", ClientRepName: "Ali Veli", SiteManagerName: "Ahmet Yılmaz"}
	if f := ValidateProject("KMP-01", "Kampüs", "TRY", "Active", extra); len(f) != 0 {
		t.Fatalf("dolu Active ek alanlarıyla proje reddedildi: %v", f)
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
