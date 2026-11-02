package reportpdf

import (
	"bytes"
	"fmt"
	"image/color"
	"testing"
	"time"
)

func sampleCurve() []CurvePt {
	pv := []float64{3, 8, 14, 20, 26, 32, 37.5, 42.6, 48, 54, 60, 66}
	ev := []float64{2.5, 6.5, 12, 17.5, 23.5, 29.5, 35, 40.1}
	ac := []float64{2.6, 6.8, 12.4, 17.9, 23, 28.6, 34, 39}
	const bac = 48.5e6
	var out []CurvePt
	for i, v := range pv {
		c := CurvePt{Month: fmt.Sprintf("2026-%02d", i+3), PV: v / 100 * bac}
		if i < len(ev) {
			c.EV, c.AC = ev[i]/100*bac, ac[i]/100*bac
		}
		out = append(out, c)
	}
	return out
}

func samplePeriod(kind string, photos int) PeriodData {
	d := PeriodData{
		Kind: kind, ProjectName: "Vadi Konutları Etap-2", ProjectCode: "DEMO-02", Currency: "TRY",
		Title: "Ekim 2026", Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		BAC: 48.5e6, PV: 20.68e6, EV: 19.4485e6, AC: 18.9e6, SPI: 0.94, CPI: 1.029, EAC: 47.133e6, ETC: 28.233e6,
		ProgressPct: 40.1, BudgetVariance: 1.367e6, PlanSource: "manual", Curve: sampleCurve(), AsOfMonth: "2026-10",
		Payments: []PayRow{{"Yılmaz Kalıp Ltd.", 4, 1820000, 214600, 1605400}, {"Öz-Yapı İnşaat", 2, 940000, 118200, 821800}, {"Akım Elektrik", 3, 612000, 71900, 540100}},
		MonthGross: 3372000, MonthDed: 404700, MonthNet: 2967300,
		Deductions: []DedRow{{"Teminat", 168600}, {"Avans mahsubu", 140000}, {"Stopaj", 96100}},
		Milestones: []MsRow{
			{"Temel betonu", time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), "Completed", false},
			{"Zemin kat döşemesi", time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), "Completed", false},
			{"3. kat kaba inşaat", time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), time.Time{}, "InProgress", true},
			{"Çatı betonu", time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC), time.Time{}, "Planned", false}},
		OHSOpened: 11, OHSClosed: 14, OHSOpen: 9, OHSCritical: 1,
		POsOrdered: 18, POsDelivered: 15, POsOverdue: 2, PRsPending: 3,
		Delays:  []DailyDelayRow{{"Hava koşulları", 16, ""}, {"Malzeme tedarik", 9.5, ""}, {"Tasarım / proje değişikliği", 7, ""}, {"Ekipman arızası", 6, ""}},
		CPIMin: 0.9, SPIMin: 0.9,
		Attention: []AttnRow{
			{"warn", "3. kat kaba inşaat geç", "Kilometre taşı planlanan tarihte tamamlanmadı (plan 15.10.2026)."},
			{"warn", "SPI 1,00'ın altında", "Temmuz 0,90 → Ağustos 0,92 → Eylül 0,93 → Ekim 0,94. Toparlanıyor; kaynak artırımı değerlendirilmeli."},
			{"info", "1 açık kritik İSG bulgusu", "Kritik bulgular kapatılana dek İSG performansı izlenmeli."}},
	}
	if kind == "Quarterly" {
		d.Title, d.Start, d.End = "2026 Q3", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
		d.AsOfMonth, d.SPI, d.CPI = "2026-09", 0.933, 1.03
		d.PV, d.EV, d.AC, d.ProgressPct = 18.19e6, 16.975e6, 16.49e6, 35.0
		d.Narrative = []string{
			"Üçüncü çeyrekte proje fiziki olarak %17,5 ilerledi ve kümülatif gerçekleşme %35,0 oldu; planın (%37,5) 2,5 puan gerisinde.",
			"Maliyet tarafı sağlıklı: CPI 1,03 ve tamamlanma tahmini bütçenin 1,37 M altında. En büyük gecikme nedeni hava koşulları (28,0 sa)."}
		d.Months = []MonthRow{
			{"Temmuz", 6, 0.90, 1.02, 2.35e6, 1980, 21, 9, 8, []string{"Zemin kat kolon–perde betonları tamamlandı", "Kesinleşen hakediş net 2,35 M", "21,0 saat gecikme kaydedildi"}},
			{"Ağustos", 6, 0.92, 1.03, 2.62e6, 2140, 29, 12, 11, []string{"Zemin kat döşemesi tamamlandı", "Kesinleşen hakediş net 2,62 M"}},
			{"Eylül", 5.5, 0.93, 1.03, 2.87e6, 2210, 17, 10, 13, []string{"1. kat kaba inşaat tamamlandı", "İSG: 10 bulgu açıldı, 13 kapatıldı"}}}
		d.Risks = []RiskRow{{1, 4, 4, "Tahsilat gecikmesi", "Açık"}, {2, 3, 3, "Kış hava kaybı", "İzleniyor"}, {3, 3, 4, "Çelik fiyat artışı", "Açık"}}
		d.Decisions = []DecisionRow{{time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), "Kalıp ekibine 8 kişilik takviye yapılmasına karar verildi."}}
		d.Next = &NextRow{Label: "2026 Q4", ProgressFrom: 35, ProgressTo: 54, Milestones: []MsRow{{Name: "Çatı betonu", Planned: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)}, {Name: "Kaba inşaat teslimi", Planned: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)}}}
	}
	if photos > 0 {
		var g []PhotoGroup
		for _, lab := range []string{"Temmuz", "Ağustos", "Eylül"} {
			grp := PhotoGroup{Label: lab}
			for i := 0; i < photos; i++ {
				grp.Photos = append(grp.Photos, Photo{Data: testJPEG(900, 600, color.RGBA{150, 165, 190, 255}), Title: "A Blok · güneydoğu", Sub: "30." + "09.2026"})
			}
			g = append(g, grp)
		}
		if kind != "Quarterly" {
			g = []PhotoGroup{{Photos: g[0].Photos}}
		}
		d.Photos = g
	}
	return d
}

func TestBuildPeriod(t *testing.T) {
	for _, tc := range []struct {
		kind, name string
		photos     int
	}{{"Monthly", "aylik", 8}, {"Monthly", "aylik-fotosuz", 0}, {"Quarterly", "ceyrek", 3}, {"Quarterly", "ceyrek-fotosuz", 0}, {"Custom", "ozel", 0}} {
		pdf := BuildPeriod(samplePeriod(tc.kind, tc.photos))
		if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.Contains(pdf, []byte("%%EOF")) {
			t.Fatalf("%s: geçerli PDF değil", tc.name)
		}
		dumpPDF(t, tc.name+".pdf", pdf)
	}
}
