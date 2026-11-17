package reportpdf

import (
	"fmt"
	"time"
)

// Malzeme - Envanter raporları: Depo-Stok ve Makine-Ekipman.
// KURAL (reportpdf ile aynı): PDF'e giren her değer çağıranın verdiği veriden
// gelir; burada sayı uydurulmaz. Stoklar miktar bazlıdır, birim fiyat yoktur.

// StokKalem — bir stok kaleminin dönem sonu durumu.
type StokKalem struct {
	Ad, Kategori, Birim string
	Mevcut, Min, Eksik  float64
	Kapsama             *float64 // gün; son 30 gün çıkışına göre, çıkış yoksa nil
	SonHareket          string   // YYYY-MM-DD, yoksa ""
	Kritik              bool
}

// HareketSatir — dönemdeki tek bir stok hareketi.
type HareketSatir struct {
	Tarih, Belge, Malzeme, Birim, Tur, Aciklama string
	Miktar                                      float64
}

// ZimmetSatir — iade edilmemiş (açık) zimmet kaydı.
type ZimmetSatir struct {
	Tarih, Personel, Firma, Kalem, Birim string
	Miktar                               float64
}

// DepoStokData — Depo-Stok raporunun girdisi.
type DepoStokData struct {
	ProjeAd, ProjeKod string
	Baslangic, Bitis  time.Time
	RaporTarihi       time.Time
	Kalemler          []StokKalem
	TurSayisi         map[string]int // hareket türü → dönemdeki kayıt adedi
	Hareketler        []HareketSatir
	AcikZimmet        []ZimmetSatir
	ToplamAcikZimmet  int
	GeneratedAt       time.Time
}

// EkipmanSatir — projeye atanmış bir ekipmanın dönem özeti.
type EkipmanSatir struct {
	Ad, Tip, Kimlik, Sahiplik, Durum string
	Calisma                          float64
	CalismaBirim                     string
	AtanmisGun, CalismaGun, BostaGun int
	Kullanim                         float64  // 0..1: çalışılan gün ÷ atanmış gün
	BostaMaliyet                     *float64 // yalnızca kiralık: boşta gün × günlük ücret
	SonBakim, SonrakiBakim           string
	KalanGun                         *int // sonraki bakıma kalan gün; negatif = geçmiş
}

// TransferSatir — proje ile ilgili bir transfer talebi.
type TransferSatir struct {
	Tarih, Ekipman, Kaynak, Hedef, Durum string
}

// MakineEkipmanData — Makine-Ekipman raporunun girdisi.
type MakineEkipmanData struct {
	ProjeAd, ProjeKod string
	Baslangic, Bitis  time.Time
	RaporTarihi       time.Time
	Ekipmanlar        []EkipmanSatir
	Transferler       []TransferSatir
	GeneratedAt       time.Time
}

func dateOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return ISO2DMY(s)
}

func periodText(a, b time.Time) string {
	return fmt.Sprintf("%s – %s", DMY(a), DMY(b))
}

// BuildDepoStok — Depo-Stok raporu PDF'i.
func BuildDepoStok(in DepoStokData) []byte {
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now()
	}
	footer := fmt.Sprintf("ConCoord · Depo-Stok Raporu · %s", in.ProjeKod)
	p := newPage("DEPO-STOK RAPORU", footer, "Depo-Stok · "+in.ProjeKod)
	p.bigHeader("DEPO-STOK RAPORU", "Stok Durumu", 22, periodText(in.Baslangic, in.Bitis),
		fmt.Sprintf("%s · %s", in.ProjeAd, in.ProjeKod))

	kritik := 0
	for _, kk := range in.Kalemler {
		if kk.Kritik {
			kritik++
		}
	}
	p.kpiStrip([]kpi{
		{Label: "Stok kalemi", Value: fmt.Sprint(len(in.Kalemler))},
		{Label: "Kritik / eksik", Value: fmt.Sprint(kritik), Warn: kritik > 0},
		{Label: "Dönem hareketi", Value: fmt.Sprint(sumCounts(in.TurSayisi))},
		{Label: "Açık zimmet", Value: fmt.Sprint(in.ToplamAcikZimmet), Accent: true},
	}, 44)

	p.section("A. Stok bakiyesi ve minimum stok uyumu", "Kapsama = mevcut ÷ son 30 gün ortalama çıkış")
	cols := fillCols([]col{
		{Title: "Malzeme", W: 130}, {Title: "Kategori", W: 62},
		{Title: "Mevcut", W: 60, Right: true}, {Title: "Min.", W: 56, Right: true},
		{Title: "Eksik", W: 56, Right: true}, {Title: "Kapsama (gün)", W: 66, Right: true},
		{Title: "Son hareket", W: 62}, {Title: "Durum", W: -1},
	}, cW)
	var rows [][]string
	for _, kk := range in.Kalemler {
		kap := "—"
		if kk.Kapsama != nil {
			kap = Num(*kk.Kapsama, 0)
		}
		durum := "Normal"
		if kk.Kritik {
			durum = "Kritik"
		}
		rows = append(rows, []string{
			kk.Ad, kk.Kategori,
			Qty(kk.Mevcut) + " " + kk.Birim, Qty(kk.Min) + " " + kk.Birim,
			Qty(kk.Eksik) + " " + kk.Birim, kap, dateOrDash(kk.SonHareket), durum,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"Kayıt yok", "", "", "", "", "", "", ""})
	}
	p.table(cols, rows, true)
	p.y += 8

	p.section("B. Dönem hareketleri (tür bazında)", periodText(in.Baslangic, in.Bitis))
	turCols := fillCols([]col{{Title: "Hareket türü", W: 160}, {Title: "Kayıt adedi", W: 90, Right: true}}, 250)
	turRows := [][]string{
		{"Giriş", fmt.Sprint(in.TurSayisi["giris"])},
		{"Çıkış", fmt.Sprint(in.TurSayisi["cikis"])},
		{"İade", fmt.Sprint(in.TurSayisi["iade"])},
		{"Sayım", fmt.Sprint(in.TurSayisi["sayim"])},
	}
	p.table(turCols, turRows, true)
	p.y += 8

	p.section("C. Açık zimmetler", fmt.Sprintf("%d kayıt", in.ToplamAcikZimmet))
	zCols := fillCols([]col{
		{Title: "Zimmet tarihi", W: 70}, {Title: "Personel", W: 120}, {Title: "Firma", W: 140},
		{Title: "Kalem", W: 150}, {Title: "Miktar", W: -1, Right: true},
	}, cW)
	var zRows [][]string
	for _, z := range in.AcikZimmet {
		zRows = append(zRows, []string{dateOrDash(z.Tarih), z.Personel, z.Firma, z.Kalem, Qty(z.Miktar) + " " + z.Birim})
	}
	if len(zRows) == 0 {
		zRows = append(zRows, []string{"Kayıt yok", "", "", "", ""})
	}
	p.table(zCols, zRows, false)
	p.y += 8

	p.section("D. Hareket dökümü (son kayıtlar)", "En fazla 60 kayıt")
	hCols := fillCols([]col{
		{Title: "Tarih", W: 60}, {Title: "Belge no", W: 80}, {Title: "Malzeme", W: 150},
		{Title: "Tür", W: 60}, {Title: "Miktar", W: 70, Right: true}, {Title: "Açıklama", W: -1},
	}, cW)
	var hRows [][]string
	for _, h := range in.Hareketler {
		hRows = append(hRows, []string{dateOrDash(h.Tarih), h.Belge, h.Malzeme, h.Tur,
			Qty(h.Miktar) + " " + h.Birim, h.Aciklama})
	}
	if len(hRows) == 0 {
		hRows = append(hRows, []string{"Kayıt yok", "", "", "", "", ""})
	}
	p.table(hCols, hRows, false)

	return p.finish(in.GeneratedAt)
}

// BuildMakineEkipman — Makine-Ekipman raporu PDF'i.
func BuildMakineEkipman(in MakineEkipmanData) []byte {
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now()
	}
	footer := fmt.Sprintf("ConCoord · Makine-Ekipman Raporu · %s", in.ProjeKod)
	p := newPage("MAKİNE-EKİPMAN RAPORU", footer, "Makine-Ekipman · "+in.ProjeKod)
	p.bigHeader("MAKİNE-EKİPMAN RAPORU", "Envanter", 22, periodText(in.Baslangic, in.Bitis),
		fmt.Sprintf("%s · %s", in.ProjeAd, in.ProjeKod))

	var aktif, bakimda, devre, kiralik, gecmis, yakin int
	var bosta float64
	for _, e := range in.Ekipmanlar {
		switch e.Durum {
		case "Aktif":
			aktif++
		case "Bakımda":
			bakimda++
		case "Devre dışı":
			devre++
		}
		if e.Sahiplik == "Kiralık" {
			kiralik++
			if e.BostaMaliyet != nil {
				bosta += *e.BostaMaliyet
			}
		}
		if e.KalanGun != nil {
			if *e.KalanGun < 0 {
				gecmis++
			} else if *e.KalanGun <= 30 {
				yakin++
			}
		}
	}
	p.kpiStrip([]kpi{
		{Label: "Toplam ekipman", Value: fmt.Sprint(len(in.Ekipmanlar))},
		{Label: "Aktif / bakımda", Value: fmt.Sprintf("%d / %d", aktif, bakimda)},
		{Label: "Bakımı geçmiş", Value: fmt.Sprint(gecmis), Warn: gecmis > 0},
		{Label: "Bakımı yaklaşan (30 gün)", Value: fmt.Sprint(yakin)},
		{Label: "Kiralık boşta maliyeti", Value: Num(bosta, 0) + " TL", Accent: true,
			Sub: fmt.Sprintf("%d kiralık ekipman · devre dışı %d", kiralik, devre)},
	}, 44)

	p.section("A. Envanter durumu ve bakım", "Kalan gün = sonraki bakım − rapor tarihi")
	cols := fillCols([]col{
		{Title: "Ekipman", W: 130}, {Title: "Tip", W: 62}, {Title: "Kimlik", W: 74},
		{Title: "Sahiplik", W: 56}, {Title: "Durum", W: 56},
		{Title: "Son bakım", W: 56}, {Title: "Sonraki bakım", W: 62}, {Title: "Kalan gün", W: -1, Right: true},
	}, cW)
	var rows [][]string
	for _, e := range in.Ekipmanlar {
		kalan := "—"
		if e.KalanGun != nil {
			kalan = fmt.Sprint(*e.KalanGun)
		}
		rows = append(rows, []string{e.Ad, e.Tip, e.Kimlik, e.Sahiplik, e.Durum,
			dateOrDash(e.SonBakim), dateOrDash(e.SonrakiBakim), kalan})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"Kayıt yok", "", "", "", "", "", "", ""})
	}
	p.table(cols, rows, true)
	p.y += 8

	p.section("B. Kullanım oranı ve boşta maliyet", "Kullanım = çalışılan gün ÷ atanmış gün")
	uCols := fillCols([]col{
		{Title: "Ekipman", W: 120}, {Title: "Atanmış gün", W: 60, Right: true},
		{Title: "Çalışılan gün", W: 65, Right: true}, {Title: "Boşta gün", W: 55, Right: true},
		{Title: "Kullanım", W: 55, Right: true}, {Title: "Çalışma", W: 70, Right: true},
		{Title: "Boşta maliyet", W: -1, Right: true},
	}, cW)
	var uRows [][]string
	for _, e := range in.Ekipmanlar {
		bm := "—"
		if e.BostaMaliyet != nil {
			bm = Num(*e.BostaMaliyet, 0) + " TL"
		}
		uRows = append(uRows, []string{e.Ad, fmt.Sprint(e.AtanmisGun), fmt.Sprint(e.CalismaGun),
			fmt.Sprint(e.BostaGun), Num(e.Kullanim*100, 0) + "%",
			Qty(e.Calisma) + " " + birimOr(e.CalismaBirim), bm})
	}
	if len(uRows) == 0 {
		uRows = append(uRows, []string{"Kayıt yok", "", "", "", "", "", ""})
	}
	p.table(uCols, uRows, true)
	p.y += 8

	p.section("C. Transfer talepleri", fmt.Sprintf("%d kayıt", len(in.Transferler)))
	tCols := fillCols([]col{
		{Title: "Talep tarihi", W: 70}, {Title: "Ekipman", W: 150}, {Title: "Kaynak proje", W: 130},
		{Title: "Hedef proje", W: 130}, {Title: "Durum", W: -1},
	}, cW)
	var tRows [][]string
	for _, t := range in.Transferler {
		tRows = append(tRows, []string{dateOrDash(t.Tarih), t.Ekipman, t.Kaynak, t.Hedef, t.Durum})
	}
	if len(tRows) == 0 {
		tRows = append(tRows, []string{"Kayıt yok", "", "", "", ""})
	}
	p.table(tCols, tRows, false)

	return p.finish(in.GeneratedAt)
}

func sumCounts(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	return t
}

// birimOr — çalışma birimi boşsa varsayılan "saat" (hiç kayıt yoksa).
func birimOr(b string) string {
	if b == "" {
		return "saat"
	}
	return b
}
