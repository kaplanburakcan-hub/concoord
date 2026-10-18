package payments

import (
	"strings"
	"testing"
)

func TestTrNum(t *testing.T) {
	cases := []struct {
		v    float64
		dec  int
		want string
	}{
		{0, 2, "0,00"}, {1234567.891, 2, "1.234.567,89"}, {-3151250, 2, "-3.151.250,00"},
		{999, 0, "999"}, {1000, 0, "1.000"}, {56.3456, 1, "56,3"},
	}
	for _, c := range cases {
		if got := trNum(c.v, c.dec); got != c.want {
			t.Errorf("trNum(%v,%d)=%q, beklenen %q", c.v, c.dec, got, c.want)
		}
	}
	if got := trQty(1200); got != "1.200" {
		t.Errorf("trQty(1200)=%q", got)
	}
	if got := trQty(2.5); got != "2,5" {
		t.Errorf("trQty(2.5)=%q", got)
	}
}

func TestBuildSummaryPDF_Basics(t *testing.T) {
	d := PDFData{ProjectName: "Şişli Ğ İ", ProjectCode: "X-1", Subcontractor: "Çağrı Ltd.", PeriodNo: 2, Status: "Draft"}
	for i := 0; i < 80; i++ { // çok sayfaya taşmalı
		d.Lines = append(d.Lines, CalcLine{PozNo: "P", Description: "kalem", Unit: "m", UnitPrice: 1, CumQty: 1, ThisAmount: 1})
	}
	pdf := string(BuildSummaryPDF(d))
	if !strings.HasPrefix(pdf, "%PDF-1.4") || !strings.HasSuffix(pdf, "%%EOF") {
		t.Fatal("geçerli PDF çerçevesi yok")
	}
	if !strings.Contains(pdf, "/Type /Pages /Count 2") && !strings.Contains(pdf, "/Type /Pages /Count 3") {
		t.Error("80 satır birden çok sayfaya taşmalıydı")
	}
	if !strings.Contains(pdf, "/FontFile2") {
		t.Error("yazı tipi gömülmemiş")
	}
}
