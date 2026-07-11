package payments

import "testing"

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 0.005
}

func dedByType(res CalcResult, t string) float64 {
	for _, d := range res.Deductions {
		if d.Type == t {
			return d.Amount
		}
	}
	return 0
}

// TestSyntheticTwoPeriods — Plan Faz 3 kabul kriteri: kümülatif ikinci dönemde
// doğru taşınıyor; avans mahsubu ve teminat kesintisi sentetik doğrulama setiyle
// BİREBİR tutuyor. Senaryo elle hesaplanıp beklenen değerler sabitlenmiştir.
//
// Sözleşme: avans 10.000, teminat %3, avans mahsup oranı %20, KDV %20.
// Pozlar: A (birim fiyat 100), B (birim fiyat 50).
func TestSyntheticTwoPeriods(t *testing.T) {
	terms := ContractTerms{AdvanceAmount: 10000, AdvanceRatePct: 20, RetentionPct: 3, VatPct: 20}

	// --- Dönem 1: A cum=100 (10.000), B cum=200 (10.000) ---
	p1 := Compute([]CalcLineInput{
		{PozNo: "A", UnitPrice: 100, PrevCumQty: 0, CumQty: 100},
		{PozNo: "B", UnitPrice: 50, PrevCumQty: 0, CumQty: 200},
	}, 0, terms, nil)

	if !approx(p1.GrossCum, 20000) || !approx(p1.GrossThis, 20000) {
		t.Fatalf("P1 brüt hatalı: cum=%.2f this=%.2f", p1.GrossCum, p1.GrossThis)
	}
	if !approx(dedByType(p1, "AdvanceOffset"), 4000) { // min(10.000, 20.000*%20=4.000)=4.000
		t.Errorf("P1 avans mahsubu 4000 bekleniyordu, %.2f", dedByType(p1, "AdvanceOffset"))
	}
	if !approx(dedByType(p1, "Retention"), 600) { // 20.000*%3
		t.Errorf("P1 teminat 600 bekleniyordu, %.2f", dedByType(p1, "Retention"))
	}
	if !approx(p1.TotalDeductions, 4600) || !approx(p1.NetPayable, 15400) {
		t.Errorf("P1 kesinti/net hatalı: ded=%.2f net=%.2f", p1.TotalDeductions, p1.NetPayable)
	}
	if !approx(p1.VatAmount, 3080) || !approx(p1.GrandTotal, 18480) {
		t.Errorf("P1 KDV/genel toplam hatalı: kdv=%.2f gt=%.2f", p1.VatAmount, p1.GrandTotal)
	}

	// Dönem 1 satır taşıması: A this=100→10.000, B this=200→10.000
	if !approx(p1.Lines[0].ThisAmount, 10000) || !approx(p1.Lines[1].ThisAmount, 10000) {
		t.Errorf("P1 satır tutarları hatalı: %+v", p1.Lines)
	}

	// --- Dönem 2: A cum=300 (30.000), B cum=500 (25.000); grossPrev = P1.GrossCum ---
	advRecovered := dedByType(p1, "AdvanceOffset") // 4.000
	terms2 := terms
	terms2.AdvanceRecoveredPrev = advRecovered
	p2 := Compute([]CalcLineInput{
		{PozNo: "A", UnitPrice: 100, PrevCumQty: 100, CumQty: 300},
		{PozNo: "B", UnitPrice: 50, PrevCumQty: 200, CumQty: 500},
	}, p1.GrossCum, terms2, nil)

	if !approx(p2.GrossCum, 55000) || !approx(p2.GrossPrev, 20000) || !approx(p2.GrossThis, 35000) {
		t.Fatalf("P2 brüt taşıma hatalı: cum=%.2f prev=%.2f this=%.2f", p2.GrossCum, p2.GrossPrev, p2.GrossThis)
	}
	// Kümülatif taşıma: A this=200→20.000, B this=300→15.000
	if !approx(p2.Lines[0].ThisAmount, 20000) || !approx(p2.Lines[1].ThisAmount, 15000) {
		t.Errorf("P2 satır taşıması hatalı: %+v", p2.Lines)
	}
	// Avans: kalan 6.000 (10.000−4.000); %20*35.000=7.000 → 6.000 ile SINIRLANIR.
	if !approx(dedByType(p2, "AdvanceOffset"), 6000) {
		t.Errorf("P2 avans mahsubu 6000 (kalan avansla sınırlı) bekleniyordu, %.2f", dedByType(p2, "AdvanceOffset"))
	}
	if !approx(dedByType(p2, "Retention"), 1050) { // 35.000*%3
		t.Errorf("P2 teminat 1050 bekleniyordu, %.2f", dedByType(p2, "Retention"))
	}
	if !approx(p2.TotalDeductions, 7050) || !approx(p2.NetPayable, 27950) {
		t.Errorf("P2 kesinti/net hatalı: ded=%.2f net=%.2f", p2.TotalDeductions, p2.NetPayable)
	}
	if !approx(p2.VatAmount, 5590) || !approx(p2.GrandTotal, 33540) {
		t.Errorf("P2 KDV/genel toplam hatalı: kdv=%.2f gt=%.2f", p2.VatAmount, p2.GrandTotal)
	}
}

// TestExtraDeductions — İSG ceza + vergi manuel kalemleri toplama girer.
func TestExtraDeductions(t *testing.T) {
	terms := ContractTerms{RetentionPct: 0, AdvanceRatePct: 0, VatPct: 20}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 10, PrevCumQty: 0, CumQty: 1000}, // 10.000
	}, 0, terms, []ExtraDeduction{
		{Type: "OHSPenalty", Description: "Baret ihlali", Amount: 500},
		{Type: "Tax", Description: "Stopaj", Amount: 300},
	})
	if !approx(res.TotalDeductions, 800) || !approx(res.NetPayable, 9200) {
		t.Fatalf("ekstra kesinti hatalı: ded=%.2f net=%.2f", res.TotalDeductions, res.NetPayable)
	}
	if len(res.Deductions) != 2 {
		t.Errorf("2 kesinti satırı bekleniyordu, %d", len(res.Deductions))
	}
}

// TestAdvanceFullyRecovered — avans tamamen kapandıysa mahsup 0 olur.
func TestAdvanceFullyRecovered(t *testing.T) {
	terms := ContractTerms{AdvanceAmount: 5000, AdvanceRecoveredPrev: 5000, AdvanceRatePct: 20, RetentionPct: 3, VatPct: 0}
	res := Compute([]CalcLineInput{
		{PozNo: "A", UnitPrice: 100, PrevCumQty: 0, CumQty: 100}, // 10.000
	}, 0, terms, nil)
	if dedByType(res, "AdvanceOffset") != 0 {
		t.Errorf("avans kapalıyken mahsup 0 olmalı, %.2f", dedByType(res, "AdvanceOffset"))
	}
	if !approx(dedByType(res, "Retention"), 300) {
		t.Errorf("teminat 300 bekleniyordu, %.2f", dedByType(res, "Retention"))
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{1.005: 1.01, 2.344: 2.34, 2.345: 2.35, -1.005: -1.01}
	for in, want := range cases {
		if got := round2(in); !approx(got, want) {
			t.Errorf("round2(%.4f)=%.2f, beklenen %.2f", in, got, want)
		}
	}
}
