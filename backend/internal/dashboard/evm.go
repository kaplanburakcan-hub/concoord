// Package dashboard — Faz 9: rol duyarlı proje/portföy dashboard'u, EVM
// (PV/EV/AC → SPI/CPI/EAC/ETC), aylık yönetim raporu ve eşik tabanlı uyarılar.
//
// Bu dosya EVM HESAP ÇEKİRDEĞİDİR: payments/calc.go ile aynı bilinçli karar —
// tamamen saf (DB'siz, I/O'suz) fonksiyonlar, birebir birim testlenir
// (Plan Faz 9 kabul kriteri: "EVM değerleri elle hesaplanan kontrol setiyle
// birebir tutuyor"). DB'den veri toplama evm_db.go'dadır.
//
// Tanımlar (Plan §7):
//   PV = planlanmış S-eğrisi (aylık dağılım girişi → milestone ağırlıkları →
//        doğrusal dağılım öncelik sırasıyla türetilir) × BAC
//   EV = kesinleşmiş hakediş brütlerinin sözleşme toplamına oranı × BAC
//   AC = kesinleşen hakediş net tutarları + teslim alınmış PO tutarları
package dashboard

import (
	"math"
	"sort"
	"time"
)

// round2 — kuruş yuvarlama, sıfırdan uzağa (payments.round2 ile aynı kural;
// toplamlar ile satır dökümü birbirini tutsun diye tüm tutarlara uygulanır).
func round2(x float64) float64 {
	if x < 0 {
		return -math.Round(-x*100) / 100
	}
	return math.Round(x*100) / 100
}

// round3 — endeksler (SPI/CPI) için 3 ondalık.
func round3(x float64) float64 {
	if x < 0 {
		return -math.Round(-x*1000) / 1000
	}
	return math.Round(x*1000) / 1000
}

// MonthKey — "YYYY-MM".
func MonthKey(t time.Time) string { return t.Format("2006-01") }

// MonthsBetween — start..end aralığındaki ay anahtarları (her ikisi dahil).
// end < start ise yalnızca start ayı döner (bozuk künyeye karşı emniyet).
func MonthsBetween(start, end time.Time) []string {
	s := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	e := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	if e.Before(s) {
		e = s
	}
	var out []string
	for cur := s; !cur.After(e); cur = cur.AddDate(0, 1, 0) {
		out = append(out, MonthKey(cur))
	}
	return out
}

// LinearPlanPct — ay başına eşit planlanan yüzde (Σ = 100). Son aya kalan
// yuvarlanır ki toplam tam 100 olsun.
func LinearPlanPct(months []string) map[string]float64 {
	out := map[string]float64{}
	n := len(months)
	if n == 0 {
		return out
	}
	per := math.Round(100.0/float64(n)*1000) / 1000
	sum := 0.0
	for i, m := range months {
		if i == n-1 {
			out[m] = math.Round((100.0-sum)*1000) / 1000
		} else {
			out[m] = per
			sum += per
		}
	}
	return out
}

// SCurvePoint — bir ayın KÜMÜLATİF PV/EV/AC değerleri.
type SCurvePoint struct {
	Month string  `json:"month"` // YYYY-MM
	PV    float64 `json:"pv"`
	EV    float64 `json:"ev"`
	AC    float64 `json:"ac"`
}

// SCurveInput — eğri girdisi. PlannedPct DÖNEMSEL yüzdedir (kümülatif değil);
// EVByMonth/ACByMonth dönemsel TUTARDIR. Months sıralı olmalıdır.
type SCurveInput struct {
	BAC        float64
	Months     []string
	PlannedPct map[string]float64
	EVByMonth  map[string]float64
	ACByMonth  map[string]float64
}

// BuildSCurve — kümülatif S-eğrisi noktaları. PV = kümülatif % × BAC / 100.
func BuildSCurve(in SCurveInput) []SCurvePoint {
	var out []SCurvePoint
	cumPct, cumEV, cumAC := 0.0, 0.0, 0.0
	for _, m := range in.Months {
		cumPct += in.PlannedPct[m]
		cumEV += in.EVByMonth[m]
		cumAC += in.ACByMonth[m]
		out = append(out, SCurvePoint{
			Month: m,
			PV:    round2(in.BAC * cumPct / 100.0),
			EV:    round2(cumEV),
			AC:    round2(cumAC),
		})
	}
	return out
}

// EVScale — EV ölçeği: sözleşme toplamı ve BAC pozitifse BAC/sözleşme oranı;
// aksi halde 1 (hakediş brütü doğrudan EV sayılır — küçük kurulumlarda bütçe
// girilmemiş olabilir; motor sıfıra bölmez, veri geldikçe kendini düzeltir).
func EVScale(bac, contractsTotal float64) float64 {
	if bac > 0 && contractsTotal > 0 {
		return bac / contractsTotal
	}
	return 1
}

// SPI — EV/PV. PV=0 iken tanımsızdır; 0 döner (arayüz "—" gösterir).
func SPI(ev, pv float64) float64 {
	if pv <= 0 {
		return 0
	}
	return round3(ev / pv)
}

// CPI — EV/AC. AC=0 iken tanımsızdır; 0 döner.
func CPI(ev, ac float64) float64 {
	if ac <= 0 {
		return 0
	}
	return round3(ev / ac)
}

// EAC — tamamlanma tahmini: AC + (BAC−EV)/CPI. CPI tanımsızsa (0) kalan iş
// bütçelendiği gibi harcanır varsayımı: AC + (BAC−EV).
func EAC(bac, ev, ac, cpi float64) float64 {
	rem := bac - ev
	if rem < 0 {
		rem = 0
	}
	if cpi > 0 {
		return round2(ac + rem/cpi)
	}
	return round2(ac + rem)
}

// ETC — kalan maliyet tahmini: EAC − AC (negatife düşmez).
func ETC(eac, ac float64) float64 {
	v := round2(eac - ac)
	if v < 0 {
		return 0
	}
	return v
}

// ProgressPct — fiziki ilerleme %: EV/BAC. BAC yoksa 0.
func ProgressPct(ev, bac float64) float64 {
	if bac <= 0 {
		return 0
	}
	p := round2(ev / bac * 100)
	if p > 100 {
		p = 100
	}
	return p
}

// NormalizeMonthTotals — anahtarları sıralı döndürür (deterministik çıktı).
func NormalizeMonthTotals(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
