package reportpdf

import (
	"bytes"
	"image/color"
	"testing"
	"time"
)

func sampleWeekly(photos int) WeeklyData {
	mon := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	mans := []int{78, 81, 84, 86, 83, 52, 0}
	temps := []float64{12, 14, 17, 15, 11, 9, 0}
	conds := []string{"Açık", "Bulutlu", "Parçalı bulutlu", "Açık", "Yağmurlu", "Rüzgârlı", ""}
	d := WeeklyData{
		ProjectName: "Vadi Konutları Etap-2", ProjectCode: "DEMO-02", WeekNo: 42,
		Start: mon, End: mon.AddDate(0, 0, 6), DaysReported: 6, ManpowerDays: 464, EquipHours: 352, WorkEntries: 41, DelayHours: 9.5,
		PPCPlanned: 38, PPCCompleted: 31,
		Disciplines: []WeeklyDisc{
			{"Kaba inşaat", []string{"A Blok 3. kat kalıp–donatı–beton tamamlandı", "Döşeme kalıbı %85"}},
			{"İnce inşaat", []string{"B Blok zemin kat duvar örgüsü %70"}},
			{"Mekanik", []string{"Bodrum temiz su ana hattı bitti"}},
			{"Elektrik", []string{"Zemin kat boru tesisatı sürüyor"}},
		},
		Look: []WeeklyLook{
			{"A Blok 4. kat kaba", mon.AddDate(0, 0, 7), mon.AddDate(0, 0, 20)},
			{"B Blok ince işler", mon.AddDate(0, 0, 9), mon.AddDate(0, 0, 25)},
			{"Zemin kat elektrik", mon.AddDate(0, 0, 7), mon.AddDate(0, 0, 16)},
			{"Çatı su yalıtımı", mon.AddDate(0, 0, 18), mon.AddDate(0, 0, 27)},
		},
		Deliveries: []WeeklyDelivery{{"Anadolu Hazır Beton", "C30/37 beton", "m³", 212}, {"Kardemir Çelik", "S420 inşaat demiri", "ton", 34}, {"Ytong Bayi", "Gazbeton blok 13,5", "palet", 6}},
		LowStock:   []WeeklyStock{{"Kalıp yağı", "lt", 12, 20}},
		OpenTasks:  14, TasksDue: 5, PendingMARs: 3, PendingPayments: []string{"Yılmaz Kalıp #4", "Öz-Yapı #2"}, PendingPOs: 4, OverduePOs: 1,
		Tasks: []WeeklyTask{
			{"B Blok zemin kat elektrik pano yeri — mimari çelişki", "S. Demir", "Urgent", "InProgress", time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)},
			{"Kalıp yağı ve ayırıcı stok yenilemesi", "Satın alma", "High", "Todo", time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)},
		},
	}
	for i := 0; i < 7; i++ {
		dy := WeeklyDay{Date: mon.AddDate(0, 0, i), Reported: mans[i] > 0, Manpower: mans[i], Cond: conds[i]}
		if mans[i] > 0 {
			dy.TempMax = fp(temps[i])
		}
		d.Days = append(d.Days, dy)
	}
	for i := 0; i < photos; i++ {
		ph := Photo{Data: testJPEG(1000, 700, color.RGBA{150, 165, 190, 255}), Title: "Saha fotoğrafı", Sub: "15.10.2026 · Saha", Tag: dayShort[(i+1)%7]}
		if i == 0 {
			ph.Featured, ph.Tag = true, "Öne çıkan"
		}
		d.Photos = append(d.Photos, ph)
	}
	return d
}

func TestBuildWeekly(t *testing.T) {
	for _, n := range []int{0, 9} {
		pdf := BuildWeekly(sampleWeekly(n))
		if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
			t.Fatal("geçerli PDF değil")
		}
		dumpPDF(t, map[int]string{0: "haftalik-fotosuz.pdf", 9: "haftalik.pdf"}[n], pdf)
	}
}
