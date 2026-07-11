package dashboard

import (
	"math"
	"testing"
	"time"
)

// Sentetik kontrol seti — tüm beklenen değerler ELLE hesaplanmıştır
// (Plan Faz 9 kabul kriteri: "EVM değerleri elle hesaplanan kontrol setiyle
// birebir tutuyor").
//
// Senaryo: BAC = 1.000.000 TL, 5 aylık proje (2026-01..2026-05).
// Planlanan dağılım: %10, %20, %30, %25, %15 (Σ100).
// Sözleşme toplamı 800.000 TL → EV ölçeği = 1.000.000/800.000 = 1,25.
// Kesinleşen hakediş brütleri: Oca 80.000, Şub 120.000 → EV(dönem) 100.000 / 150.000.
// AC: Oca net 90.000 + PO 15.000 = 105.000; Şub net 130.000.
//
// Şubat sonu kümülatifler:
//   PV = (10+20)% × 1.000.000 = 300.000
//   EV = 100.000 + 150.000   = 250.000
//   AC = 105.000 + 130.000   = 235.000
//   SPI = 250.000/300.000 = 0,833   CPI = 250.000/235.000 = 1,064 (3 ondalık)
//   EAC = 235.000 + (1.000.000−250.000)/ (250.000/235.000)
//       = 235.000 + 750.000×0,94 = 235.000 + 705.000 = 940.000
//   ETC = 940.000 − 235.000 = 705.000
func controlSet() (SCurveInput, float64) {
	months := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05"}
	bac := 1_000_000.0
	scale := EVScale(bac, 800_000) // 1,25
	in := SCurveInput{
		BAC:    bac,
		Months: months,
		PlannedPct: map[string]float64{
			"2026-01": 10, "2026-02": 20, "2026-03": 30, "2026-04": 25, "2026-05": 15,
		},
		EVByMonth: map[string]float64{
			"2026-01": round2(80_000 * scale),  // 100.000
			"2026-02": round2(120_000 * scale), // 150.000
		},
		ACByMonth: map[string]float64{
			"2026-01": 105_000,
			"2026-02": 130_000,
		},
	}
	return in, bac
}

func approx(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestEVScale(t *testing.T) {
	if got := EVScale(1_000_000, 800_000); !approx(got, 1.25) {
		t.Fatalf("EVScale = %v, beklenen 1.25", got)
	}
	if got := EVScale(0, 800_000); got != 1 {
		t.Fatalf("BAC=0 iken ölçek 1 olmalı, got %v", got)
	}
	if got := EVScale(1_000_000, 0); got != 1 {
		t.Fatalf("sözleşme toplamı 0 iken ölçek 1 olmalı, got %v", got)
	}
}

func TestSCurveCumulative(t *testing.T) {
	in, _ := controlSet()
	pts := BuildSCurve(in)
	if len(pts) != 5 {
		t.Fatalf("5 nokta beklenirdi, %d geldi", len(pts))
	}
	// Ocak.
	if !approx(pts[0].PV, 100_000) || !approx(pts[0].EV, 100_000) || !approx(pts[0].AC, 105_000) {
		t.Fatalf("Ocak kümülatifleri yanlış: %+v", pts[0])
	}
	// Şubat (elle hesaplanan kontrol noktası).
	if !approx(pts[1].PV, 300_000) || !approx(pts[1].EV, 250_000) || !approx(pts[1].AC, 235_000) {
		t.Fatalf("Şubat kümülatifleri yanlış: %+v", pts[1])
	}
	// Mayıs: PV tam BAC'a oturur; EV/AC değişmez.
	last := pts[4]
	if !approx(last.PV, 1_000_000) || !approx(last.EV, 250_000) || !approx(last.AC, 235_000) {
		t.Fatalf("Mayıs kümülatifleri yanlış: %+v", last)
	}
}

func TestIndicesAndForecast(t *testing.T) {
	in, bac := controlSet()
	pts := BuildSCurve(in)
	feb := pts[1]

	spi := SPI(feb.EV, feb.PV)
	cpi := CPI(feb.EV, feb.AC)
	if !approx(spi, 0.833) {
		t.Fatalf("SPI = %v, beklenen 0.833", spi)
	}
	if !approx(cpi, 1.064) {
		t.Fatalf("CPI = %v, beklenen 1.064", cpi)
	}

	// EAC ham CPI ile hesaplanır (yuvarlanmışla değil) — elle: 940.000.
	rawCPI := feb.EV / feb.AC
	eac := EAC(bac, feb.EV, feb.AC, rawCPI)
	if !approx(eac, 940_000) {
		t.Fatalf("EAC = %v, beklenen 940000", eac)
	}
	if etc := ETC(eac, feb.AC); !approx(etc, 705_000) {
		t.Fatalf("ETC = %v, beklenen 705000", etc)
	}
}

func TestIndicesUndefined(t *testing.T) {
	if SPI(100, 0) != 0 || CPI(100, 0) != 0 {
		t.Fatal("PV/AC=0 iken endeksler 0 (tanımsız) dönmeli")
	}
	// CPI tanımsızken EAC = AC + (BAC−EV).
	if got := EAC(1000, 200, 0, 0); !approx(got, 800) {
		t.Fatalf("CPI tanımsızken EAC = %v, beklenen 800", got)
	}
}

func TestProgressPct(t *testing.T) {
	if got := ProgressPct(250_000, 1_000_000); !approx(got, 25) {
		t.Fatalf("ilerleme %% = %v, beklenen 25", got)
	}
	if ProgressPct(10, 0) != 0 {
		t.Fatal("BAC=0 iken ilerleme 0 olmalı")
	}
	if ProgressPct(1_200_000, 1_000_000) != 100 {
		t.Fatal("ilerleme %100'de tavanlanmalı")
	}
}

func TestLinearPlanSumsTo100(t *testing.T) {
	months := MonthsBetween(
		time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)) // 7 ay
	if len(months) != 7 {
		t.Fatalf("7 ay beklenirdi, %d geldi: %v", len(months), months)
	}
	plan := LinearPlanPct(months)
	sum := 0.0
	for _, m := range months {
		sum += plan[m]
	}
	if !approx(sum, 100) {
		t.Fatalf("doğrusal plan toplamı %v, beklenen 100", sum)
	}
}

func TestMonthsBetweenDegenerate(t *testing.T) {
	// end < start → tek ay (emniyet).
	m := MonthsBetween(
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	if len(m) != 1 || m[0] != "2026-05" {
		t.Fatalf("bozuk aralıkta tek ay beklenirdi, got %v", m)
	}
}
