package reports

import (
	"strings"
	"testing"
)

func TestValidateDelayCauses(t *testing.T) {
	cases := []struct {
		name    string
		causes  []delayCauseDTO
		wantKey string // boş ise hata beklenmez
	}{
		{"geçerli kayıt", []delayCauseDTO{{ReasonCode: "Hava", Hours: 3.5}}, ""},
		{"bilinmeyen neden", []delayCauseDTO{{ReasonCode: "Kaza", Hours: 1}}, "delay_causes[0].reason_code"},
		{"sıfır saat", []delayCauseDTO{{ReasonCode: "Sahip", Hours: 0}}, "delay_causes[0].hours"},
		{"24 saati aşan tek kayıt", []delayCauseDTO{{ReasonCode: "Sahip", Hours: 25}}, "delay_causes[0].hours"},
		{"toplam 24 saati aşar", []delayCauseDTO{{ReasonCode: "Hava", Hours: 15}, {ReasonCode: "Malzeme", Hours: 10}}, "delay_causes"},
		{"uzun not", []delayCauseDTO{{ReasonCode: "Diger", Hours: 1, Note: strings.Repeat("a", 501)}}, "delay_causes[0].note"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fields := map[string]string{}
			validateDelayCauses(c.causes, fields)
			if c.wantKey == "" {
				if len(fields) != 0 {
					t.Fatalf("hata beklenmiyordu: %v", fields)
				}
				return
			}
			if _, ok := fields[c.wantKey]; !ok {
				t.Fatalf("%q alanı için hata bekleniyordu, alınan: %v", c.wantKey, fields)
			}
		})
	}
}

func TestDelayCausesJSON(t *testing.T) {
	if got := delayCausesJSON(nil); got != "[]" {
		t.Fatalf("nil liste [] olmalı, alınan %s", got)
	}
	got := delayCausesJSON([]delayCauseDTO{{ReasonCode: "Hava", Hours: 2}})
	if !strings.Contains(got, `"reason_code":"Hava"`) || !strings.Contains(got, `"hours":2`) {
		t.Fatalf("beklenmeyen JSON: %s", got)
	}
}
