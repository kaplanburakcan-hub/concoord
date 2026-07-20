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
	// Yeni akış: ödenebilir = brüt + KDV, kesintiler ondan düşülür.
	// 10.000 + 2.000 (KDV %20) − 800 = 11.200
	if !approx(res.TotalDeductions, 800) || !approx(res.NetPayable, 11200) {
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

// --- Faz 11: kesinti niteliği ve stopaj ---

// TestWithholdingApplied — yıllara sari işte stopaj brütten kesilir, AC'yi
// AZALTMAZ (taşeronun vergisidir, ana yüklenicinin maliyeti brüt kalır).
func TestWithholdingApplied(t *testing.T) {
	terms := ContractTerms{
		RetentionPct: 0, AdvanceRatePct: 0, VatPct: 20,
		ApplyWithholding: true, WithholdingPct: 5,
	}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 100, PrevCumQty: 0, CumQty: 1000}, // brüt 100.000
	}, 0, terms, nil)

	if !approx(res.TotalDeductions, 5000) {
		t.Fatalf("stopaj 5.000 olmalıydı, %.2f", res.TotalDeductions)
	}
	// 100.000 + 20.000 KDV − 5.000 stopaj = 115.000
	if !approx(res.NetPayable, 115000) {
		t.Fatalf("ödenecek 115.000 olmalıydı, %.2f", res.NetPayable)
	}
	// AC brüt ile aynı: stopaj maliyeti azaltmaz.
	if !approx(res.ActualCost, 100000) {
		t.Fatalf("AC 100.000 olmalıydı (stopaj düşülmemeli), %.2f", res.ActualCost)
	}
}

// TestWithholdingNotApplied — aynı yıl içinde biten işte stopaj kesilmez.
func TestWithholdingNotApplied(t *testing.T) {
	terms := ContractTerms{VatPct: 20, ApplyWithholding: false, WithholdingPct: 5}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 100, PrevCumQty: 0, CumQty: 1000},
	}, 0, terms, nil)
	for _, d := range res.Deductions {
		if d.Type == "Withholding" {
			t.Fatalf("stopaj uygulanmamalıydı")
		}
	}
}

// TestActualCostExcludesFinancing — EVM AC hesabı: avans mahsubu ve teminat
// maliyeti azaltmaz; İSG cezası ve diğer mal/hizmet bedeli azaltır.
func TestActualCostExcludesFinancing(t *testing.T) {
	terms := ContractTerms{
		AdvanceAmount: 50000, AdvanceRatePct: 20, RetentionPct: 5, VatPct: 20,
	}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 100, PrevCumQty: 0, CumQty: 1000}, // brüt 100.000
	}, 0, terms, []ExtraDeduction{
		{Type: "OHSPenalty", Description: "İSG ceza", Amount: 3000},
		{Type: "Other", Description: "Yemek bedeli", Amount: 2000},
	})

	// Kesintiler: avans 20.000 + teminat 5.000 + ceza 3.000 + yemek 2.000 = 30.000
	if !approx(res.TotalDeductions, 30000) {
		t.Fatalf("toplam kesinti 30.000 olmalıydı, %.2f", res.TotalDeductions)
	}
	// 100.000 + 20.000 KDV − 30.000 = 90.000
	if !approx(res.NetPayable, 90000) {
		t.Fatalf("ödenecek 90.000 olmalıydı, %.2f", res.NetPayable)
	}
	// AC yalnızca mal/hizmet bedelini düşer: 100.000 − (3.000 + 2.000) = 95.000
	if !approx(res.CostReducing, 5000) {
		t.Fatalf("maliyet azaltan kesinti 5.000 olmalıydı, %.2f", res.CostReducing)
	}
	if !approx(res.ActualCost, 95000) {
		t.Fatalf("AC 95.000 olmalıydı, %.2f", res.ActualCost)
	}
}

// TestDeductionNatures — her kesinti satırı doğru nitelikle etiketlenir.
func TestDeductionNatures(t *testing.T) {
	terms := ContractTerms{
		AdvanceAmount: 50000, AdvanceRatePct: 10, RetentionPct: 3, VatPct: 20,
		ApplyWithholding: true, WithholdingPct: 5,
	}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 100, PrevCumQty: 0, CumQty: 1000},
	}, 0, terms, []ExtraDeduction{
		{Type: "OHSPenalty", Description: "İSG ceza", Amount: 1000},
	})

	want := map[string]struct {
		nature  string
		reduces bool
	}{
		"AdvanceOffset": {NatureOffset, false},
		"Retention":     {NatureTemporary, false},
		"Withholding":   {NaturePermanent, false},
		"OHSPenalty":    {NaturePermanent, true},
	}
	if len(res.Deductions) != len(want) {
		t.Fatalf("%d kesinti bekleniyordu, %d geldi", len(want), len(res.Deductions))
	}
	for _, d := range res.Deductions {
		w, ok := want[d.Type]
		if !ok {
			t.Fatalf("beklenmeyen kesinti tipi: %s", d.Type)
		}
		if d.Nature != w.nature {
			t.Errorf("%s niteliği %s olmalıydı, %s", d.Type, w.nature, d.Nature)
		}
		if d.ReducesCost != w.reduces {
			t.Errorf("%s reduces_cost %v olmalıydı, %v", d.Type, w.reduces, d.ReducesCost)
		}
	}
}

// TestResolveWithholding — hakediş tiki sözleşme varsayılanını geçersiz kılar.
func TestResolveWithholding(t *testing.T) {
	yes, no := true, false
	if ResolveWithholding(nil, true) != true {
		t.Error("tik yokken yıllara sari sözleşme stopaj uygulamalı")
	}
	if ResolveWithholding(nil, false) != false {
		t.Error("tik yokken tek yıllık sözleşme stopaj uygulamamalı")
	}
	if ResolveWithholding(&no, true) != false {
		t.Error("elle kapatma sözleşme varsayılanını geçersiz kılmalı")
	}
	if ResolveWithholding(&yes, false) != true {
		t.Error("elle açma sözleşme varsayılanını geçersiz kılmalı")
	}
}

// TestVatWithholding — KDV tevkifatı: KDV tutarı üzerinden uygulanır, brütten
// DEĞİL. Ödenecek tutarı düşürür, işin maliyetini (AC) etkilemez.
func TestVatWithholding(t *testing.T) {
	terms := ContractTerms{VatPct: 20, VatWithholdingRatio: 0.4} // 4/10 yapım işleri
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 100, PrevCumQty: 0, CumQty: 1000}, // brüt 100.000
	}, 0, terms, nil)

	if !approx(res.VatAmount, 20000) {
		t.Fatalf("hesaplanan KDV 20.000 olmalıydı, %.2f", res.VatAmount)
	}
	if !approx(res.VatWithheld, 8000) {
		t.Fatalf("tevkif edilen KDV 8.000 olmalıydı (20.000×0,4), %.2f", res.VatWithheld)
	}
	if !approx(res.VatCollected, 12000) {
		t.Fatalf("tahsil edilen KDV 12.000 olmalıydı, %.2f", res.VatCollected)
	}
	// Kesinti yok: ödenecek = brüt + tahsil edilen KDV
	if !approx(res.PayableGross, 112000) || !approx(res.NetPayable, 112000) {
		t.Fatalf("ödenecek 112.000 olmalıydı, %.2f", res.NetPayable)
	}
	// Tevkifat maliyeti etkilemez.
	if !approx(res.ActualCost, 100000) {
		t.Fatalf("AC 100.000 olmalıydı, %.2f", res.ActualCost)
	}
}

// TestVatExempt — %0 KDV (istisna) durumunda KDV ve tevkifat sıfırdır.
func TestVatExempt(t *testing.T) {
	terms := ContractTerms{VatPct: 0, VatWithholdingRatio: 0.4}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 100, PrevCumQty: 0, CumQty: 1000},
	}, 0, terms, nil)
	if !approx(res.VatAmount, 0) || !approx(res.VatWithheld, 0) || !approx(res.VatCollected, 0) {
		t.Fatalf("istisnada KDV sıfır olmalıydı: kdv=%.2f tevkifat=%.2f tahsil=%.2f",
			res.VatAmount, res.VatWithheld, res.VatCollected)
	}
	if !approx(res.NetPayable, 100000) {
		t.Fatalf("ödenecek brüt ile aynı olmalıydı, %.2f", res.NetPayable)
	}
}

// TestRealHakedisFlow — saha örneğinin akışı: kesintiler KDV DAHİL ödenebilir
// toplamdan düşülür; mal/hizmet kesintileri kendi KDV oranını taşır; maliyet
// (AC) KDV hariç hesaplanır.
func TestRealHakedisFlow(t *testing.T) {
	terms := ContractTerms{VatPct: 20, VatWithholdingRatio: 0.4}
	res := Compute([]CalcLineInput{
		{PozNo: "X", UnitPrice: 1000, PrevCumQty: 0, CumQty: 100}, // brüt 100.000
	}, 0, terms, []ExtraDeduction{
		// Yemek bedeli KDV %10 dahil 11.000 → net 10.000
		{Type: "Other", Description: "Öğle yemeği", Amount: 11000, VatPct: 10,
			GroupCode: "GoodsService", CatalogCode: "GS_LUNCH"},
		// İSG cezası KDV'siz 5.000
		{Type: "OHSPenalty", Description: "İSG ceza", Amount: 5000, VatPct: 0,
			GroupCode: "Penalty", CatalogCode: "PEN_OHS"},
	})

	if !approx(res.VatAmount, 20000) {
		t.Fatalf("KDV 20.000 olmalıydı, %.2f", res.VatAmount)
	}
	if !approx(res.VatCollected, 12000) {
		t.Fatalf("tahsil edilen KDV 12.000 olmalıydı, %.2f", res.VatCollected)
	}
	if !approx(res.PayableGross, 112000) {
		t.Fatalf("ödenebilir toplam 112.000 olmalıydı, %.2f", res.PayableGross)
	}
	if !approx(res.TotalDeductions, 16000) {
		t.Fatalf("kesinti toplamı 16.000 olmalıydı, %.2f", res.TotalDeductions)
	}
	if !approx(res.NetPayable, 96000) {
		t.Fatalf("ödenecek 96.000 olmalıydı, %.2f", res.NetPayable)
	}
	// AC = 100.000 − (yemek KDV hariç 10.000 + ceza 5.000) = 85.000
	if !approx(res.CostReducing, 15000) {
		t.Fatalf("maliyet azaltan 15.000 olmalıydı, %.2f", res.CostReducing)
	}
	if !approx(res.ActualCost, 85000) {
		t.Fatalf("AC 85.000 olmalıydı, %.2f", res.ActualCost)
	}
}

// TestRetentionRefundAddition — teminat iadesi ödenecek tutarı ARTIRIR ve
// maliyeti (AC) DEĞİŞTİRMEZ: daha önce kesilmiş bir tutarın geri verilmesidir,
// yeni bir iş bedeli değildir.
func TestRetentionRefundAddition(t *testing.T) {
	terms := ContractTerms{VatPct: 20, RetentionPct: 5}
	res := ComputeWith(
		[]CalcLineInput{{PozNo: "X", UnitPrice: 1000, PrevCumQty: 0, CumQty: 100}}, // brüt 100.000
		0, terms, nil,
		[]AdditionLine{
			{Type: "RetentionRefund", Description: "Geçici kabul iadesi", Amount: 25000,
				Stage: "ProvisionalAcceptance"},
		},
	)

	// Teminat kesintisi: 100.000 × %5 = 5.000
	if !approx(res.TotalDeductions, 5000) {
		t.Fatalf("kesinti 5.000 olmalıydı, %.2f", res.TotalDeductions)
	}
	if !approx(res.TotalAdditions, 25000) {
		t.Fatalf("ilave 25.000 olmalıydı, %.2f", res.TotalAdditions)
	}
	// Ödenecek = brüt 100.000 + KDV 20.000 + iade 25.000 − teminat 5.000 = 140.000
	if !approx(res.NetPayable, 140000) {
		t.Fatalf("ödenecek 140.000 olmalıydı, %.2f", res.NetPayable)
	}
	// İade maliyeti etkilemez.
	if !approx(res.ActualCost, 100000) {
		t.Fatalf("AC 100.000 olmalıydı, %.2f", res.ActualCost)
	}
}
