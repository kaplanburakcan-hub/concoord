// Package payments — Faz 3 finansal çekirdek: taşeron, sözleşme, birim fiyat
// cetveli (work_items) ve kümülatif hakediş yönetimi (Plan §6.3, §6.4).
//
// Bu dosya HESAP ÇEKİRDEĞİDİR: tamamen saf (DB'siz, I/O'suz), bu yüzden birebir
// birim testlenir (Plan Faz 3 kabul kriteri: "kümülatif hesap ikinci dönemde
// doğru taşınıyor; avans mahsubu ve teminat kesintisi sentetik doğrulama setiyle
// birebir tutuyor"). Handler yalnızca girdi toplar, sonucu DB'ye yazar.
package payments

import "math"

// round2 — kuruş (2 ondalık) yuvarlama, sıfırdan uzağa (banker's değil) — Türk
// hakediş uygulamasındaki alışılmış yuvarlama. Tüm ara/nihai tutarlara uygulanır
// ki toplamlar ile satır dökümü birbirini tutsun.
func round2(x float64) float64 {
	if x < 0 {
		return -math.Round(-x*100) / 100
	}
	return math.Round(x*100) / 100
}

// CalcLineInput — bir poz için hesap girdisi. cum_qty = bu döneme dek KÜMÜLATİF
// yapılan iş miktarı (kullanıcı bunu girer). prev_cum_qty önceki Finalized
// hakedişten taşınır (yoksa 0).
type CalcLineInput struct {
	WorkItemID  string
	PozNo       string
	Description string
	Unit        string
	UnitPrice   float64
	ContractQty float64
	PrevCumQty  float64
	CumQty      float64
}

// CalcLine — bir poz için hesaplanmış sonuç.
type CalcLine struct {
	WorkItemID    string  `json:"work_item_id"`
	PozNo         string  `json:"poz_no"`
	Description   string  `json:"description"`
	Unit          string  `json:"unit"`
	UnitPrice     float64 `json:"unit_price"`
	ContractQty   float64 `json:"contract_qty"`
	PrevCumQty    float64 `json:"prev_cum_qty"`
	ThisPeriodQty float64 `json:"this_period_qty"`
	CumQty        float64 `json:"cum_qty"`
	CumAmount     float64 `json:"cum_amount"`
	ThisAmount    float64 `json:"this_amount"`
}

// ContractTerms — kesinti hesabını süren sözleşme parametreleri. Oranlar YÜZDE
// (ör. 3.0 = %3) olarak verilir; core bunları kesirlere çevirir.
type ContractTerms struct {
	AdvanceAmount        float64 // toplam avans
	AdvanceRecoveredPrev float64 // önceki dönemlere dek mahsup edilmiş avans
	AdvanceRatePct       float64 // bu dönem brütünden mahsup oranı (%)
	RetentionPct         float64 // teminat kesinti oranı (%)
	VatPct               float64 // KDV oranı (%)

	// Stopaj (GVK Md. 42-44 — yıllara sari inşaat işleri). Aynı takvim yılında
	// başlayıp biten işlerde kesilmez; sonraki yıla sarkan işlerde kesilir.
	// ApplyWithholding çağıran tarafından çözülür: hakedişte elle işaretlenmişse
	// o değer, değilse sözleşmenin is_multi_year varsayılanı.
	ApplyWithholding bool    // bu hakedişte stopaj uygulanacak mı
	WithholdingPct   float64 // stopaj oranı (%) — tipik 5.00

	// KDV tevkifatı (Faz 11). Diğer kesintilerden YAPISAL OLARAK FARKLIDIR:
	// brütten değil, hesaplanan KDV TUTARI üzerinden uygulanır. Yapım işlerinde
	// 4/10 → 0.4: KDV'nin %40'ı taşerona ödenmez, alıcı tarafından doğrudan
	// vergi dairesine yatırılır. İşin maliyetini (EVM AC) ETKİLEMEZ, yalnızca
	// ödenecek tutarı değiştirir.
	VatWithholdingRatio float64 // 0 = tevkifat yok, 0.4 = 4/10
}

// ExtraDeduction — otomatik olmayan (İSG ceza, vergi/stopaj, diğer) kesinti kalemi.
// Amount doğrudan verilir; rate_pct yalnızca kayıt/gösterim içindir.
type ExtraDeduction struct {
	// Nature/ReducesCost boş bırakılırsa DeductionNature(Type) ile türetilir.
	Nature      string   `json:"nature,omitempty"`
	ReducesCost *bool    `json:"reduces_cost,omitempty"`
	GroupCode   string   `json:"group_code,omitempty"`
	CatalogCode string   `json:"catalog_code,omitempty"`
	VatPct      float64  `json:"vat_pct,omitempty"` // kesintinin kendi KDV oranı (Amount KDV dahil)
	Type        string   `json:"type"` // Withholding | Tax | OHSPenalty | Other
	Description string   `json:"description"`
	RatePct     *float64 `json:"rate_pct,omitempty"`
	Amount      float64  `json:"amount"`
	SourceEntity *string `json:"source_entity,omitempty"`
	SourceID     *string `json:"source_id,omitempty"`
}

// DeductionLine — hesaplanmış tek kesinti satırı (payment_deductions'a yazılır).
type DeductionLine struct {
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	RatePct      *float64 `json:"rate_pct,omitempty"`
	Amount       float64  `json:"amount"`
	SourceEntity *string  `json:"source_entity,omitempty"`
	SourceID     *string  `json:"source_id,omitempty"`

	// Nitelik (Faz 11): Offset=avans mahsubu, Temporary=iade edilecek (teminat),
	// Permanent=kâti. ReducesCost, kesintinin EVM Gerçekleşen Maliyet (AC)
	// hesabından düşülüp düşülmeyeceğini belirtir.
	Nature      string `json:"nature"`
	ReducesCost bool   `json:"reduces_cost"`
	GroupCode   string `json:"group_code,omitempty"`   // Tax|Advance|Retention|Penalty|GoodsService|Adjustment
	CatalogCode string `json:"catalog_code,omitempty"` // deduction_catalog.code

	// Kesintinin KENDİ KDV oranı. Mal/hizmet kesintileri (yemek, elektrik,
	// malzeme) ana yüklenicinin taşerona yaptığı satıştır ve KDV taşır; ceza ile
	// finansman kalemleri (avans, teminat, stopaj) KDV'sizdir. Amount KDV DAHİL
	// tutardır; NetAmount maliyet hesabında (AC) kullanılan KDV hariç kısımdır.
	VatPct    float64 `json:"vat_pct"`
	NetAmount float64 `json:"net_amount"`
}

// CalcResult — bir hakediş döneminin tam hesap sonucu (Plan §6.4 A–I).
type CalcResult struct {
	Lines           []CalcLine      `json:"lines"`
	GrossCum        float64         `json:"gross_cum"`        // A
	GrossPrev       float64         `json:"gross_prev"`       // B
	GrossThis       float64         `json:"gross_this"`       // C
	// --- KDV (dönem brütü üzerinden) ---
	VatPct       float64 `json:"vat_pct"`
	VatAmount    float64 `json:"vat_amount"`    // hesaplanan KDV = C × oran
	VatWithheld  float64 `json:"vat_withheld"`  // tevkif edilen (vergi dairesine yatırılır)
	VatCollected float64 `json:"vat_collected"` // tahsil edilen (ödemeye eklenir)

	// --- Ödeme akışı ---
	PayableGross    float64         `json:"payable_gross"` // C + tahsil edilen KDV
	Deductions      []DeductionLine `json:"deductions"`
	TotalDeductions float64         `json:"total_deductions"` // KDV DAHİL kesinti toplamı
	NetPayable      float64         `json:"net_payable"`      // yükleniciye ödenecek nihai tutar

	// --- Maliyet (EVM) ---
	CostReducing float64 `json:"cost_reducing"` // maliyeti azaltan kesintilerin KDV hariç toplamı
	ActualCost   float64 `json:"actual_cost"`   // AC = C − CostReducing (KDV hariç)
}

// Compute — bir hakediş döneminin kümülatif hesabını üretir (Plan §6.4).
//
// grossPrev çağıran tarafından önceki Finalized hakedişin gross_cum'undan verilir
// (yoksa 0). Avans mahsubu kalan avansı aşamaz; teminat bu dönem brütü üzerinden
// hesaplanır. Negatif dönem brütünde (düzeltme senaryosu) otomatik kesintiler 0
// alınır — ters kayıt manuel kalemle yapılır.
func Compute(inputs []CalcLineInput, grossPrev float64, terms ContractTerms, extras []ExtraDeduction) CalcResult {
	res := CalcResult{VatPct: terms.VatPct}
	var grossCum float64
	for _, in := range inputs {
		thisQty := in.CumQty - in.PrevCumQty
		cumAmount := round2(in.CumQty * in.UnitPrice)
		thisAmount := round2(thisQty * in.UnitPrice)
		grossCum += cumAmount
		res.Lines = append(res.Lines, CalcLine{
			WorkItemID:    in.WorkItemID,
			PozNo:         in.PozNo,
			Description:   in.Description,
			Unit:          in.Unit,
			UnitPrice:     in.UnitPrice,
			ContractQty:   in.ContractQty,
			PrevCumQty:    in.PrevCumQty,
			ThisPeriodQty: thisQty,
			CumQty:        in.CumQty,
			CumAmount:     cumAmount,
			ThisAmount:    thisAmount,
		})
	}
	res.GrossCum = round2(grossCum)
	res.GrossPrev = round2(grossPrev)
	res.GrossThis = round2(res.GrossCum - res.GrossPrev)

	// D — Avans mahsubu: kalan avansı aşmaz.
	if res.GrossThis > 0 && terms.AdvanceRatePct > 0 {
		remaining := terms.AdvanceAmount - terms.AdvanceRecoveredPrev
		if remaining < 0 {
			remaining = 0
		}
		offset := round2(res.GrossThis * terms.AdvanceRatePct / 100)
		if offset > remaining {
			offset = round2(remaining)
		}
		if offset > 0 {
			rate := terms.AdvanceRatePct
			res.Deductions = append(res.Deductions, DeductionLine{
				Type: "AdvanceOffset", Description: "Avans mahsubu", RatePct: &rate, Amount: offset,
				Nature: NatureOffset, ReducesCost: false, GroupCode: "Advance", CatalogCode: "ADV_CONTRACT",
				VatPct: 0, NetAmount: offset,
			})
		}
	}

	// E — Teminat (retention) kesintisi: bu dönem brütü üzerinden.
	if res.GrossThis > 0 && terms.RetentionPct > 0 {
		ret := round2(res.GrossThis * terms.RetentionPct / 100)
		if ret > 0 {
			rate := terms.RetentionPct
			res.Deductions = append(res.Deductions, DeductionLine{
				Type: "Retention", Description: "Teminat kesintisi", RatePct: &rate, Amount: ret,
				Nature: NatureTemporary, ReducesCost: false, GroupCode: "Retention", CatalogCode: "RET_PERFORMANCE",
				VatPct: 0, NetAmount: ret,
			})
		}
	}

	// F — Stopaj (yıllara sari işler): bu dönem brütü üzerinden, KDV hariç.
	// Taşeronun gelir vergisi mahsubudur; kaynakta kesilip onun adına yatırılır,
	// bu yüzden ana yüklenicinin maliyetini AZALTMAZ (ReducesCost=false).
	if res.GrossThis > 0 && terms.ApplyWithholding && terms.WithholdingPct > 0 {
		wh := round2(res.GrossThis * terms.WithholdingPct / 100)
		if wh > 0 {
			rate := terms.WithholdingPct
			res.Deductions = append(res.Deductions, DeductionLine{
				Type: "Withholding", Description: "Stopaj (yıllara sari)", RatePct: &rate, Amount: wh,
				Nature: NaturePermanent, ReducesCost: false, GroupCode: "Tax", CatalogCode: "WHT_INCOME",
				VatPct: 0, NetAmount: wh,
			})
		}
	}

	// G/H — İSG ceza, vergi, diğer (manuel/otomasyon kalemleri).
	for _, e := range extras {
		amt := round2(e.Amount)
		if amt == 0 {
			continue
		}
		nature := e.Nature
		if nature == "" {
			nature = DeductionNature(e.Type)
		}
		reduces := DeductionReducesCost(e.Type)
		if e.ReducesCost != nil {
			reduces = *e.ReducesCost
		}
		// Kesintinin kendi KDV oranı; Amount KDV DAHİL tutardır.
		net := amt
		if e.VatPct > 0 {
			net = round2(amt / (1 + e.VatPct/100))
		}
		res.Deductions = append(res.Deductions, DeductionLine{
			Type: e.Type, Description: e.Description, RatePct: e.RatePct, Amount: amt,
			SourceEntity: e.SourceEntity, SourceID: e.SourceID,
			Nature: nature, ReducesCost: reduces,
			GroupCode: e.GroupCode, CatalogCode: e.CatalogCode,
			VatPct: e.VatPct, NetAmount: net,
		})
	}

	// ---- Ödeme akışı (gerçek hakediş sırası) ----
	//
	//   KDV, DÖNEM BRÜTÜ üzerinden hesaplanır (kesintilerden ÖNCE): taşeron işin
	//   tamamını faturalar. Tevkifat kadarı taşerona ödenmez, doğrudan vergi
	//   dairesine yatırılır.
	//
	//   Kesintiler, KDV DAHİL ödenebilir toplamdan düşülür: ana yüklenicinin
	//   taşerona sattığı mal/hizmet (yemek, elektrik, malzeme) kendi KDV'siyle
	//   birlikte mahsup edilir.
	res.VatAmount = round2(res.GrossThis * terms.VatPct / 100)
	res.VatWithheld = round2(res.VatAmount * terms.VatWithholdingRatio)
	res.VatCollected = round2(res.VatAmount - res.VatWithheld)
	res.PayableGross = round2(res.GrossThis + res.VatCollected)

	var totalDed, costRed float64
	for _, d := range res.Deductions {
		totalDed += d.Amount
		if d.ReducesCost {
			// Maliyet KDV HARİÇ hesaplanır: KDV devreden bir kalemdir, maliyet değil.
			costRed += d.NetAmount
		}
	}
	res.TotalDeductions = round2(totalDed)
	res.NetPayable = round2(res.PayableGross - res.TotalDeductions)

	// ---- EVM Gerçekleşen Maliyet (AC) ----
	// İşin sözleşme bedeli (KDV hariç) eksi yalnızca maliyeti gerçekten azaltan
	// kesintilerin KDV hariç tutarı. Avans, teminat ve stopaj finansman/vergi
	// kalemidir — maliyeti değiştirmez.
	res.CostReducing = round2(costRed)
	res.ActualCost = round2(res.GrossThis - res.CostReducing)
	return res
}
