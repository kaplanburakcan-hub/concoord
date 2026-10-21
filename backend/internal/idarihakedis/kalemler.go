package idarihakedis

// Standart kesinti kalemleri, sözleşmeden gelen oranlar ve oran değiştirme
// yetkisi. Kural: vergi / teminat / avans / SGK gibi temel kalemler silinemez
// (sunucu her kayıtta eksik olanı yeniden ekler); ceza ve ek kesintiler
// serbestçe eklenip silinebilir. Oranlı kalemlerin tutarı sunucuda
// hesaplanır (baz × oran); oranı hakedişte değiştirmek yetki ister.

import (
	"context"
	"math"
	"strings"

	"github.com/google/uuid"
)

// KesintiKalem — rapordaki a-i kesinti/mahsup kalemleri + serbest ek satırlar.
type KesintiKalem struct {
	Kod     string   `json:"kod,omitempty"` // standart kalem kodu; serbest satırda boş
	Ad      string   `json:"ad"`
	Tutar   float64  `json:"tutar"`
	OranPct *float64 `json:"oran_pct,omitempty"` // oranlı kalem: tutar = baz × oran / 100
	Kilitli bool     `json:"kilitli"`            // sunucu belirler; silinemez
}

type stdKalem struct {
	Kod     string `json:"kod"`
	Ad      string `json:"ad"`
	Kilitli bool   `json:"kilitli"`
	Baz     string `json:"baz"` // oranlı kalemlerde "E" (bu hakediş tutarı) ya da "F" (KDV); yoksa ""
}

var stdKalemler = []stdKalem{
	{"STOPAJ", "Gelir / Kurumlar Vergisi", true, "E"},
	{"DAMGA", "Damga Vergisi", true, "E"},
	{"KDV_TEVKIFAT", "KDV Tevkifatı", true, "F"},
	{"GECICI_KABUL", "Geçici Kabul Kesintisi", true, "E"},
	{"FF_KESIN_TEMINAT", "Fiyat Farkı Kesin Teminat Kesintisi", true, ""},
	{"SGK", "Sosyal Sigortalar Kurumu Kesintisi", true, ""},
	{"IDARE_MAKINESI", "İdare Makinesi Kiraları", false, ""},
	{"GECIKME_CEZASI", "Gecikme Cezası", false, ""},
	{"AVANS", "Avans Mahsubu", true, ""},
}

func stdByKod(kod string) (stdKalem, bool) {
	for _, s := range stdKalemler {
		if s.Kod == kod {
			return s, true
		}
	}
	return stdKalem{}, false
}

// Eski kayıtlarda yalnızca ad vardı: ad'dan standart kalem kodu çıkarılır.
func kodFromAd(ad string) string {
	a := foldTR(ad)
	switch {
	case strings.Contains(a, "gelir") || strings.Contains(a, "kurumlar"):
		return "STOPAJ"
	case strings.Contains(a, "damga"):
		return "DAMGA"
	case strings.Contains(a, "tevkifat"):
		return "KDV_TEVKIFAT"
	case strings.Contains(a, "gecici kabul"):
		return "GECICI_KABUL"
	case strings.Contains(a, "kesin teminat"):
		return "FF_KESIN_TEMINAT"
	case strings.Contains(a, "sosyal sigorta") || strings.Contains(a, "sgk"):
		return "SGK"
	case strings.Contains(a, "makine"):
		return "IDARE_MAKINESI"
	case strings.Contains(a, "gecikme"):
		return "GECIKME_CEZASI"
	case strings.Contains(a, "avans"):
		return "AVANS"
	}
	return ""
}

func foldTR(s string) string {
	return strings.ToLower(strings.NewReplacer(
		"ı", "i", "İ", "i", "I", "i", "ş", "s", "Ş", "s", "ğ", "g", "Ğ", "g",
		"ü", "u", "Ü", "u", "ö", "o", "Ö", "o", "ç", "c", "Ç", "c",
	).Replace(s))
}

// normalizeKesintiler — kodları atar, kilit bayrağını sunucu belirler, silinmiş
// temel kalemleri 0 tutarla geri ekler. Çıktı sırası: standart kalemler (standart
// sırada), ardından serbest satırlar. İstemcinin gönderdiği "kilitli" dikkate alınmaz.
func normalizeKesintiler(in []KesintiKalem) []KesintiKalem {
	byKod := map[string]KesintiKalem{}
	var free []KesintiKalem
	for _, k := range in {
		kod := k.Kod
		if _, ok := stdByKod(kod); !ok {
			kod = kodFromAd(k.Ad)
		}
		if kod != "" {
			if _, taken := byKod[kod]; !taken {
				k.Kod = kod
				byKod[kod] = k
				continue
			}
		}
		k.Kod, k.Kilitli = "", false
		if strings.TrimSpace(k.Ad) != "" || k.Tutar != 0 {
			free = append(free, k)
		}
	}
	out := make([]KesintiKalem, 0, len(stdKalemler)+len(free))
	for _, s := range stdKalemler {
		k, ok := byKod[s.Kod]
		if !ok {
			if !s.Kilitli {
				continue // silinebilir standart kalem: kullanıcı kaldırdıysa geri eklenmez
			}
			k = KesintiKalem{Kod: s.Kod}
		}
		k.Ad, k.Kilitli = s.Ad, s.Kilitli
		if s.Baz == "" {
			k.OranPct = nil
		}
		out = append(out, k)
	}
	return append(out, free...)
}

// ── Oranlar ──────────────────────────────────────────────────────────────

var oranKodlari = []string{"STOPAJ", "DAMGA", "KDV_TEVKIFAT", "GECICI_KABUL"}

type oranlar struct {
	Var  bool // sözleşmede oran tanımlı mı
	Kdv  float64
	Oran map[string]float64 // kod → oran
}

// contractOranlar — Ana Sözleşme'deki hakedişe esas oranlar. Sözleşme yoksa
// Var=false ve KDV %20 varsayılanı döner.
func (h *Handler) contractOranlar(ctx context.Context, pid uuid.UUID) oranlar {
	o := oranlar{Kdv: 20, Oran: map[string]float64{}}
	var kdv, stopaj, damga, tevk, temin float64
	err := h.pool.QueryRow(ctx, `
		SELECT kdv_pct::float8, stopaj_pct::float8, damga_pct::float8,
		       kdv_tevkifat_pct::float8, teminat_pct::float8
		FROM project_main_contracts WHERE project_id=$1`, pid).Scan(&kdv, &stopaj, &damga, &tevk, &temin)
	if err != nil {
		return o
	}
	o.Var, o.Kdv = true, kdv
	o.Oran["STOPAJ"], o.Oran["DAMGA"], o.Oran["KDV_TEVKIFAT"], o.Oran["GECICI_KABUL"] = stopaj, damga, tevk, temin
	return o
}

func oranlarFromKalemler(kdv float64, ks []KesintiKalem) oranlar {
	o := oranlar{Var: true, Kdv: kdv, Oran: map[string]float64{}}
	for _, k := range ks {
		if k.OranPct != nil {
			o.Oran[k.Kod] = *k.OranPct
		}
	}
	return o
}

func f64eq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// oranDegisti — gönderilen oranlar referanstan farklı mı? (referansta olmayan
// oran, gönderilende de yoksa fark sayılmaz)
func oranDegisti(kdv float64, ks []KesintiKalem, ref oranlar) bool {
	if !f64eq(kdv, ref.Kdv) {
		return true
	}
	for _, k := range ks {
		isRate := false
		for _, rk := range oranKodlari {
			if rk == k.Kod {
				isRate = true
			}
		}
		if !isRate {
			continue
		}
		rv, hasRef := ref.Oran[k.Kod]
		switch {
		case k.OranPct == nil && !hasRef:
		case k.OranPct == nil || !hasRef:
			return true
		case !f64eq(*k.OranPct, rv):
			return true
		}
	}
	return false
}

// oranDoldur — oranı gönderilmeyen oranlı kalemlere sözleşmedeki oranı yazar.
func oranDoldur(ks []KesintiKalem, ref oranlar) {
	if !ref.Var {
		return
	}
	for i := range ks {
		s, ok := stdByKod(ks[i].Kod)
		if !ok || s.Baz == "" || ks[i].OranPct != nil {
			continue
		}
		if v, has := ref.Oran[ks[i].Kod]; has {
			vv := v
			ks[i].OranPct = &vv
		}
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// oranliTutarlar — oranlı kalemlerin tutarını baz × oran / 100 olarak yazar.
func oranliTutarlar(ks []KesintiKalem, e, f float64) {
	for i := range ks {
		s, ok := stdByKod(ks[i].Kod)
		if !ok || s.Baz == "" || ks[i].OranPct == nil {
			continue
		}
		base := e
		if s.Baz == "F" {
			base = f
		}
		ks[i].Tutar = round2(base * *ks[i].OranPct / 100)
	}
}
