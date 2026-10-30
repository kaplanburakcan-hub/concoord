package materials

import "testing"

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }

func TestOrderDeadline(t *testing.T) {
	cases := []struct {
		name   string
		needed *string
		lead   *int
		want   *string
	}{
		{"30 gün geri", strp("2026-12-01"), intp(30), strp("2026-11-01")},
		{"ay sınırını aşar", strp("2026-03-05"), intp(10), strp("2026-02-23")},
		{"sıfır süre aynı gün", strp("2026-06-10"), intp(0), strp("2026-06-10")},
		{"ihtiyaç tarihi yok", nil, intp(7), nil},
		{"süre yok", strp("2026-06-10"), nil, nil},
		{"bozuk tarih", strp("10.06.2026"), intp(7), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := orderDeadline(c.needed, c.lead)
			if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
				t.Fatalf("got %v want %v", deref(got), deref(c.want))
			}
		})
	}
}

func TestValidateMARInputLeadAndDate(t *testing.T) {
	ok := marReq{MaterialName: "Çimento", LeadTimeDays: intp(14), NeededByDate: strp("2026-12-01")}
	if errs := validateMARInput(ok); len(errs) != 0 {
		t.Fatalf("geçerli girdi reddedildi: %v", errs)
	}
	if errs := validateMARInput(marReq{MaterialName: "x", LeadTimeDays: intp(-1)}); errs["lead_time_days"] == "" {
		t.Fatal("negatif tedarik süresi reddedilmeli")
	}
	if errs := validateMARInput(marReq{MaterialName: "x", LeadTimeDays: intp(366)}); errs["lead_time_days"] == "" {
		t.Fatal("365 günü aşan tedarik süresi reddedilmeli")
	}
	if errs := validateMARInput(marReq{MaterialName: "x", NeededByDate: strp("01.12.2026")}); errs["needed_by_date"] == "" {
		t.Fatal("TR biçimli tarih reddedilmeli (YYYY-MM-DD bekleniyor)")
	}
	if errs := validateMARInput(marReq{MaterialName: "x", NeededByDate: strp("  ")}); len(errs) != 0 {
		t.Fatalf("boş tarih isteğe bağlı olmalı: %v", errs)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
