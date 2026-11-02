package dashboard

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func sampleQuarter() MonthlySnapshot {
	in, _ := controlSet()
	pts := BuildSCurve(in)
	sn := MonthlySnapshot{
		GeneratedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		ProjectName: "Örnek Konut Projesi", ProjectCode: "PRJ-01", Currency: "TRY",
		Kind: KindQuarterly, Label: "2026 Q1", Year: 2026, Month: 1, PeriodStart: "2026-01-01", PeriodEnd: "2026-03-31",
	}
	sn.EVM = EVMResult{BAC: in.BAC, PV: pts[1].PV, EV: pts[1].EV, AC: pts[1].AC, SPI: SPI(pts[1].EV, pts[1].PV),
		CPI: CPI(pts[1].EV, pts[1].AC), SCurve: pts, PlanSource: "manual", Currency: "TRY", AsOfMonth: pts[1].Month, ProgressPct: 30}
	sn.EVM.EAC = EAC(in.BAC, sn.EVM.EV, sn.EVM.AC, sn.EVM.CPI)
	sn.BudgetVariance = round2(in.BAC - sn.EVM.EAC)
	sn.Months = []MonthBrief{{Month: "2026-01", ProgressD: 10}, {Month: "2026-02", ProgressD: 8}}
	sn.DelayCauses = []MonthlyDelayCause{{ReasonCode: "Hava", Label: "Hava koşulları", Hours: 12, Reports: 4}}
	return sn
}

// Çeyrek anlatısı snapshot rakamlarından türer; PDF girdisi snapshot'ı birebir taşır.
func TestPeriodPDFFromSnapshot(t *testing.T) {
	sn := sampleQuarter()
	d := periodPDFData(sn)
	if d.Kind != KindQuarterly || d.Title != "2026 Q1" || d.BAC != sn.EVM.BAC || d.SPI != sn.EVM.SPI || len(d.Months) != 2 {
		t.Fatalf("snapshot değerleri PDF girdisine taşınmadı: %+v", d)
	}
	if len(d.Narrative) == 0 || !strings.Contains(d.Narrative[0], "%18,0") {
		t.Fatalf("anlatı ilerleme toplamını (10+8) içermeli: %v", d.Narrative)
	}
	if !strings.Contains(strings.Join(d.Narrative, " "), "hava koşulları") {
		t.Fatalf("anlatı en büyük gecikme nedenini içermeli: %v", d.Narrative)
	}
	pdf := buildPeriodPDF(context.Background(), nil, sn)
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF")) {
		t.Fatal("geçerli PDF iskeleti üretilmedi")
	}
	sn.Kind, sn.Label, sn.Month = KindMonthly, "2026-02", 2
	if d := periodPDFData(sn); d.Title != "Şubat 2026" {
		t.Fatalf("aylık başlık: %q", d.Title)
	}
}

func TestAttention(t *testing.T) {
	sn := MonthlySnapshot{Thresholds: &Thresholds{CPIMin: 0.9, SPIMin: 0.9}}
	sn.EVM.SPI, sn.EVM.CPI = 0.85, 1.1
	sn.OHS.OpenCritical = 2
	sn.Milestones = []MonthlyMilestone{{Name: "Çatı", PlannedDate: "2026-10-15", Late: true}}
	a := buildAttention(&sn)
	if len(a) < 3 || a[0].Level != "bad" || !strings.Contains(a[0].Title, "SPI") {
		t.Fatalf("en ciddi madde ilk sırada olmalı: %+v", a)
	}
	sn = MonthlySnapshot{}
	sn.EVM.SPI, sn.EVM.CPI = 1.02, 1.01
	if a := buildAttention(&sn); len(a) != 0 {
		t.Fatalf("eşik aşımı yokken madde olmamalı: %+v", a)
	}
}
