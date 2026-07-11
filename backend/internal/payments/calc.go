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
}

// ExtraDeduction — otomatik olmayan (İSG ceza, vergi/stopaj, diğer) kesinti kalemi.
// Amount doğrudan verilir; rate_pct yalnızca kayıt/gösterim içindir.
type ExtraDeduction struct {
	Type        string   `json:"type"` // Tax | OHSPenalty | Other
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
}

// CalcResult — bir hakediş döneminin tam hesap sonucu (Plan §6.4 A–I).
type CalcResult struct {
	Lines           []CalcLine      `json:"lines"`
	GrossCum        float64         `json:"gross_cum"`        // A
	GrossPrev       float64         `json:"gross_prev"`       // B
	GrossThis       float64         `json:"gross_this"`       // C
	Deductions      []DeductionLine `json:"deductions"`       // D..H
	TotalDeductions float64         `json:"total_deductions"`
	NetPayable      float64         `json:"net_payable"`      // I = C − Σkesinti (KDV hariç)
	VatPct          float64         `json:"vat_pct"`
	VatAmount       float64         `json:"vat_amount"`       // KDV ayrı satır
	GrandTotal      float64         `json:"grand_total"`      // net + KDV
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
			})
		}
	}

	// F/G/H — İSG ceza, vergi/stopaj, diğer (manuel/otomasyon kalemleri).
	for _, e := range extras {
		amt := round2(e.Amount)
		if amt == 0 {
			continue
		}
		res.Deductions = append(res.Deductions, DeductionLine{
			Type: e.Type, Description: e.Description, RatePct: e.RatePct, Amount: amt,
			SourceEntity: e.SourceEntity, SourceID: e.SourceID,
		})
	}

	var totalDed float64
	for _, d := range res.Deductions {
		totalDed += d.Amount
	}
	res.TotalDeductions = round2(totalDed)
	res.NetPayable = round2(res.GrossThis - res.TotalDeductions)
	res.VatAmount = round2(res.NetPayable * terms.VatPct / 100)
	res.GrandTotal = round2(res.NetPayable + res.VatAmount)
	return res
}
