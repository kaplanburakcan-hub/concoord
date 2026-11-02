package reportpdf

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testJPEG — deneme için düz renkli bir fotoğraf üretir.
func testJPEG(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			shade := uint8((x + y) * 40 / (w + h))
			img.SetRGBA(x, y, color.RGBA{c.R - shade, c.G - shade, c.B - shade, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 80})
	return b.Bytes()
}

// dumpPDF — REPORTPDF_OUT ayarlıysa PDF'i görsel inceleme için o klasöre yazar.
func dumpPDF(t *testing.T, name string, pdf []byte) {
	t.Helper()
	if dir := os.Getenv("REPORTPDF_OUT"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, name), pdf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func fp(v float64) *float64 { return &v }

func sampleDaily(photos int) DailyData {
	subm := time.Date(2026, 10, 14, 18, 42, 0, 0, time.UTC)
	d := DailyData{
		ProjectName: "Vadi Konutları Etap-2", ProjectCode: "DEMO-02",
		Date: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), RevisionNo: 1,
		Author: "Emre Kılıç", AuthorTitle: "Şantiye Şefi", Status: "Submitted", SubmittedAt: &subm,
		IssueFlagged: true, WeatherCond: "Parçalı bulutlu", TempMin: fp(9), TempMax: fp(17),
		Manpower: []DailyManRow{{"Kalıpçı", "Yılmaz Kalıp Ltd.", 22}, {"Demirci", "Yılmaz Kalıp Ltd.", 14},
			{"Betoncu", "Ana yüklenici", 9}, {"Duvarcı", "Öz-Yapı İnş.", 11}, {"Elektrik", "Akım Elektrik", 8},
			{"Tesisat", "Su-Isı Müh.", 6}, {"Genel işçilik", "Ana yüklenici", 14}},
		Staff: []DailyStaffRow{{"E. Kılıç", "Şantiye Şefi", "Yüklenici", "Sahada"}, {"M. Aydın", "İSG Uzmanı", "Yüklenici", "Sahada"},
			{"S. Demir", "Kontrol Müh. (Müşavir)", "Müşavir", "Sahada"}, {"T. Koç", "Mak. Müh.", "Yüklenici", "İzinli"}},
		Equipment: []DailyEquipRow{{"Kule vinç", 1, fp(9), ""}, {"Paletli ekskavatör", 2, fp(14), ""},
			{"Beton pompası", 1, fp(4), ""}, {"Mobil vinç", 1, fp(0), "Rüzgâr"}, {"Forklift", 3, fp(18), ""},
			{"Kompresör", 2, fp(12), ""}, {"Jeneratör", 1, fp(4), ""}},
		Work: []DailyWorkRow{
			{"15.150.1001", "A Blok · 3. kat", "Kolon ve perde kalıp imalatı", fp(142), "m²", "Kaba inşaat"},
			{"15.150.1003", "A Blok · 3. kat", "Döşeme donatısı (S420) serimi", fp(6.8), "ton", "Kaba inşaat"},
			{"15.115.1002", "A Blok · 3. kat", "C30/37 hazır beton dökümü — kolonlar", fp(38), "m³", "Kaba inşaat"},
			{"23.007.1101", "B Blok · Zemin", "Gazbeton duvar örgüsü (13,5 cm)", fp(210), "m²", "İnce inşaat"},
			{"26.001.2004", "B Blok · Zemin", "Elektrik tesisatı boru döşemesi", fp(320), "m", "Elektrik"},
			{"24.002.1307", "B Blok · Bodrum", "Temiz su ana hat montajı", fp(96), "m", "Mekanik"},
			{"", "Saha geneli", "Çevre düzeni — hafriyat nakliyesi", fp(24), "sefer", "Altyapı"}},
		Delays: []DailyDelayRow{{"Hava koşulları", 1.5, "Rüzgâr 55 km/s üzeri — mobil vinç çalıştırılmadı."},
			{"Malzeme tedarik", 1.0, "Pompa kamyonu 1 saat geç geldi."}},
		Cash:  []DailyCashRow{{"Yakıt", "4417", 4250}, {"Küçük malzeme", "4418", 1380}},
		Notes: "B Blok zemin kat elektrik pano yeri mimari projeyle çelişiyor; müşavire yazılı bildirim yapıldı, yarın ortak ölçü alınacak. A Blok 4. kat kalıp malzemesi cuma gününe kadar yeterli.",
	}
	for i := 0; i < photos; i++ {
		ph := Photo{Data: testJPEG(1200, 800, color.RGBA{140, 160, 190, 255}), Title: "A Blok · 3. kat — kolon betonu", Sub: "14.10.2026 · 16:20"}
		if i == 0 {
			ph.Featured, ph.Tag = true, "Kapak"
		}
		d.Photos = append(d.Photos, ph)
	}
	return d
}

func TestBuildDaily(t *testing.T) {
	for _, n := range []int{0, 7} {
		pdf := BuildDaily(sampleDaily(n))
		if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.Contains(pdf, []byte("%%EOF")) {
			t.Fatal("geçerli PDF değil")
		}
		if n > 0 && !bytes.Contains(pdf, []byte("/DCTDecode")) {
			t.Fatal("fotoğraflar PDF'e gömülmedi")
		}
		dumpPDF(t, map[int]string{0: "gunluk-fotosuz.pdf", 7: "gunluk.pdf"}[n], pdf)
	}
}
