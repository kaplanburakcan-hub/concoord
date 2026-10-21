package payments

import (
	"os"
	"path/filepath"
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

func TestFoldTR(t *testing.T) {
	if foldTR("Stopaj (yıllara sarı)") != foldTR("Stopaj (yillara sari)") {
		t.Fatal("aksansız karşılaştırma eşleşmedi")
	}
}

func sampleFlowData(layout string) PDFData {
	rate := 5.0
	d := PDFData{
		ProjectName: "Nilüfer Şehir Hastanesi Kompleksi", ProjectCode: "DEMO-04", Subcontractor: "Bursa Güç Elektrik Taahhüt Ltd.",
		ContractNo: "SZL-2025-H03", ContractAmount: 62000000, PeriodNo: 4, PeriodStart: "2026-09-01", PeriodEnd: "2026-09-30",
		Status: "SiteApproved", VatPct: 20, GrossCum: 20500000, GrossPrev: 17300000, GrossThis: 3200000,
		VatAmount: 640000, TotalDeductions: 1000000, NetPayable: 2840000, Layout: layout,
		Deductions: []DeductionLine{
			{Type: "AdvanceOffset", Description: "Avans mahsubu", Amount: 480000},
			{Type: "Withholding", Description: "Stopaj (yıllara sari)", RatePct: &rate, Amount: 160000},
			{Type: "OHSPenalty", Description: "ISG-002", Amount: 50000},
		},
		Lines: []CalcLine{
			{PozNo: "Y.E01", Description: "Ana dağıtım panosu", Unit: "adet", UnitPrice: 85000, ContractQty: 400, PrevCumQty: 120, ThisPeriodQty: 65, CumQty: 185, ThisAmount: 5525000},
			{PozNo: "Y.E02", Description: "Kablo tavası", Unit: "m", UnitPrice: 1250, ContractQty: 0, PrevCumQty: 10, ThisPeriodQty: 5, CumQty: 15, ThisAmount: 6250},
		},
		Chain: []ChainStepView{
			{Label: "Teknik Ofis Uzmanı", Actor: "Ece Aydın", Date: "05.10.2026", State: "done"},
			{Label: "Şantiye Müdürü", State: "now"},
			{Label: "Finansal Kontrol"},
		},
		History: []PeriodHist{{No: 1, Gross: 9800000, Net: 9310000, Status: "Finalized", Date: "12.06.2026"}},
		Plan:    []PlanRow{{Method: "Havale", Amount: 1000000, Date: "30.10.2026"}},
	}
	return d
}

func TestBuildSummaryPDF_Layouts(t *testing.T) {
	for _, lay := range []string{"", "panel", "akis"} {
		pdf := BuildSummaryPDF(sampleFlowData(lay))
		s := string(pdf)
		if !strings.HasPrefix(s, "%PDF-1.4") || !strings.HasSuffix(s, "%%EOF") {
			t.Errorf("%q: geçerli PDF çerçevesi yok", lay)
		}
		if p := os.Getenv("PDF_OUT_DIR"); p != "" {
			name := lay
			if name == "" {
				name = "resmi"
			}
			_ = os.WriteFile(filepath.Join(p, "ornek-"+name+".pdf"), pdf, 0o644)
		}
	}
	// Veri boşken de (zincir/plan/geçmiş yok) çökmemeli.
	for _, lay := range []string{"panel", "akis"} {
		BuildSummaryPDF(PDFData{ProjectCode: "X", Subcontractor: "Y", PeriodNo: 1, Layout: lay})
	}
}
