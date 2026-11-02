package dashboard

import (
	"bytes"
	"testing"
	"time"
)

// PDF üretimi duman testi — geçerli PDF iskeleti ve snapshot rakamlarının
// içerikte yer aldığı doğrulanır (ohs/pdf_test deseni).
func TestBuildMonthlyPDF(t *testing.T) {
	in, _ := controlSet()
	sn := MonthlySnapshot{
		GeneratedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
		ProjectName: "Örnek Konut Projesi", ProjectCode: "PRJ-01", Currency: "TRY",
		Kind: KindMonthly, Label: "2026-02", Year: 2026, Month: 2, PeriodStart: "2026-02-01", PeriodEnd: "2026-02-28",
	}
	pts := BuildSCurve(in)
	sn.EVM = EVMResult{
		BAC: in.BAC, PV: pts[1].PV, EV: pts[1].EV, AC: pts[1].AC,
		SPI: SPI(pts[1].EV, pts[1].PV), CPI: CPI(pts[1].EV, pts[1].AC),
		SCurve: pts, PlanSource: "manual", Currency: "TRY",
	}
	sn.EVM.EAC = EAC(in.BAC, sn.EVM.EV, sn.EVM.AC, sn.EVM.EV/sn.EVM.AC)
	sn.EVM.ETC = ETC(sn.EVM.EAC, sn.EVM.AC)
	sn.BudgetVariance = round2(in.BAC - sn.EVM.EAC)
	sn.FinalizedPayments = []MonthlySubPayment{
		{Subcontractor: "Kaba İnşaat Ltd.", PeriodNo: 2, GrossThis: 120000, Deductions: 10000, NetPayable: 110000},
	}
	sn.MonthGross, sn.MonthDeductions, sn.MonthNet = 120000, 10000, 110000
	sn.DeductionsByType = map[string]float64{"Retention": 3600, "AdvanceOffset": 6400}
	sn.Milestones = []MonthlyMilestone{{Name: "Temel", PlannedDate: "2026-01-31", ActualDate: "2026-02-05", Status: "Completed"}}

	pdf := BuildMonthlyPDF(sn)
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatal("PDF başlığı yok")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatal("PDF sonlandırıcısı yok")
	}
	for _, want := range []string{"AYLIK YONETIM RAPORU", "PRJ-01", "940000.00", "0.833", "1.064"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Fatalf("PDF içeriğinde %q bekleniyordu", want)
		}
	}
}

func TestBuildPeriodPDFTitles(t *testing.T) {
	for kind, want := range map[string]string{
		KindQuarterly: "CEYREK YIL YONETIM RAPORU",
		KindCustom:    "OZEL DONEM YONETIM RAPORU",
	} {
		pdf := BuildMonthlyPDF(MonthlySnapshot{Kind: kind, Label: "2026 Q2", ProjectCode: "PRJ-01",
			PeriodStart: "2026-04-01", PeriodEnd: "2026-06-30"})
		if !bytes.Contains(pdf, []byte(want)) || !bytes.Contains(pdf, []byte("2026 Q2")) {
			t.Fatalf("%s: başlık/dönem etiketi PDF'te yok", kind)
		}
	}
}

func TestPeriodLabel(t *testing.T) {
	d := func(y, m, dd int) time.Time { return time.Date(y, time.Month(m), dd, 0, 0, 0, 0, time.UTC) }
	if got := periodLabel(KindQuarterly, d(2026, 4, 1), d(2026, 6, 30)); got != "2026 Q2" {
		t.Fatalf("çeyrek etiketi: %q", got)
	}
	if got := periodLabel(KindCustom, d(2026, 3, 1), d(2026, 5, 15)); got != "01.03.2026 - 15.05.2026" {
		t.Fatalf("özel dönem etiketi: %q", got)
	}
}
