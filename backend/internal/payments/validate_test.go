package payments

import "testing"

func TestValidateContract(t *testing.T) {
	if f := ValidateContract("SZ-01", "Sub", 3, 20); len(f) != 0 {
		t.Fatalf("geçerli sözleşme reddedildi: %v", f)
	}
	f := ValidateContract("", "Nope", 140, -5)
	for _, k := range []string{"contract_no", "type", "retention_pct", "advance_rate_pct"} {
		if _, ok := f[k]; !ok {
			t.Errorf("beklenen alan hatası yok: %s (%v)", k, f)
		}
	}
}

func TestValidateWorkItem(t *testing.T) {
	if f := ValidateWorkItem("15.001", "Kalıp", 100, 250); len(f) != 0 {
		t.Fatalf("geçerli poz reddedildi: %v", f)
	}
	f := ValidateWorkItem("", "", -1, -1)
	for _, k := range []string{"poz_no", "description", "contract_qty", "unit_price"} {
		if _, ok := f[k]; !ok {
			t.Errorf("beklenen alan hatası yok: %s", k)
		}
	}
}

func TestCanTransition(t *testing.T) {
	ok := [][2]string{{"Draft", "Submitted"}, {"Submitted", "SiteApproved"}, {"SiteApproved", "Finalized"}, {"Submitted", "Rejected"}}
	for _, c := range ok {
		if !CanTransition(c[0], c[1]) {
			t.Errorf("%s→%s izinli olmalı", c[0], c[1])
		}
	}
	bad := [][2]string{{"Draft", "Finalized"}, {"Finalized", "Draft"}, {"Draft", "SiteApproved"}, {"Finalized", "Submitted"}}
	for _, c := range bad {
		if CanTransition(c[0], c[1]) {
			t.Errorf("%s→%s yasak olmalı", c[0], c[1])
		}
	}
}
