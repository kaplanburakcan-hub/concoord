package reports

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func f(v float64) *float64 { return &v }

func TestValidateDaily(t *testing.T) {
	ok := dailyInput{
		ReportDate: "2026-07-06",
		TempMin:    f(12), TempMax: f(24),
		Manpower:  []manpowerDTO{{Trade: "Kalıpçı", Headcount: 8}},
		Equipment: []equipmentDTO{{EquipmentName: "Kule vinç", Count: 1, WorkingHours: f(9)}},
		WorkEntries: []workEntryDTO{{Description: "B blok 3. kat perde betonu", Qty: f(42.5)}},
	}
	if fields := validateDaily(ok); len(fields) != 0 {
		t.Fatalf("geçerli girdi reddedildi: %v", fields)
	}

	cases := []struct {
		name string
		mut  func(*dailyInput)
		key  string
	}{
		{"bozuk tarih", func(d *dailyInput) { d.ReportDate = "06.07.2026" }, "report_date"},
		{"gelecek tarih", func(d *dailyInput) {
			d.ReportDate = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
		}, "report_date"},
		{"ters sıcaklık", func(d *dailyInput) { d.TempMin = f(30); d.TempMax = f(10) }, "temperature_max"},
		{"boş branş", func(d *dailyInput) { d.Manpower[0].Trade = "  " }, "manpower[0].trade"},
		{"negatif personel", func(d *dailyInput) { d.Manpower[0].Headcount = -1 }, "manpower[0].headcount"},
		{"boş ekipman adı", func(d *dailyInput) { d.Equipment[0].EquipmentName = "" }, "equipment[0].equipment_name"},
		{"boş imalat açıklaması", func(d *dailyInput) { d.WorkEntries[0].Description = "" }, "work_entries[0].description"},
		{"negatif miktar", func(d *dailyInput) { d.WorkEntries[0].Qty = f(-1) }, "work_entries[0].qty"},
	}
	for _, c := range cases {
		in := ok
		in.Manpower = append([]manpowerDTO{}, ok.Manpower...)
		in.Equipment = append([]equipmentDTO{}, ok.Equipment...)
		in.WorkEntries = append([]workEntryDTO{}, ok.WorkEntries...)
		c.mut(&in)
		fields := validateDaily(in)
		if _, hit := fields[c.key]; !hit {
			t.Errorf("%s: %q alanında hata bekleniyordu, dönen: %v", c.name, c.key, fields)
		}
	}
}

// TestWeeklyPDFFromSnapshot — kabul kriteri: PDF rakamları snapshot'tan
// doğrulanabilir. PDF'e giren toplamların snapshot değerleriyle birebir aynı
// metin olarak yer aldığını doğrular; ayrıca çıktı geçerli bir PDF iskeletidir.
func TestWeeklyPDFFromSnapshot(t *testing.T) {
	sub := "Yılmaz İnşaat"
	sn := Snapshot{
		GeneratedAt: time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC),
		ProjectName: "Kule Ofis", ProjectCode: "PRJ-01",
		WeekNo: 27, PeriodStart: "2026-06-29", PeriodEnd: "2026-07-05",
		Days: []SnapDay{
			{Date: "2026-06-29", RevisionNo: 1, Status: "Submitted", ManpowerTotal: 14,
				Manpower:  []manpowerDTO{{Trade: "Demirci", Headcount: 14, SubcontractorName: &sub}},
				Equipment: []equipmentDTO{{EquipmentName: "Beton pompası", Count: 2, WorkingHours: f(7.5)}},
				WorkEntries: []workEntryDTO{{Description: "A blok temel demir bağlama", Qty: f(1250)}},
			},
			{Date: "2026-06-30", RevisionNo: 2, Status: "Submitted", ManpowerTotal: 9},
		},
		Totals: SnapTotals{DaysReported: 2, ManpowerDays: 23, EquipmentHours: 7.5, WorkEntryCount: 1},
		PendingPayments: []SnapPayment{{Subcontractor: "Yılmaz İnşaat", PeriodNo: 3, Status: "Submitted"}},
		OpenTasks: 5, TasksDueWeek: 2, PendingMARs: 1,
	}

	pdf := BuildWeeklyPDF(sn)
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF")) {
		t.Fatal("geçerli PDF iskeleti üretilmedi")
	}

	s := string(pdf)
	for _, want := range []string{
		"HAFTALIK ILERLEME RAPORU",
		"PRJ-01",
		"Hafta: 27",
		itoa(sn.Totals.DaysReported),
		itoa(sn.Totals.ManpowerDays),
		num(sn.Totals.EquipmentHours, 1), // 7.5
		"rev 2",                          // revizyonlu gün görünür
		"Yilmaz Insaat / Donem 3",        // asciiTR uygulanmış bekleyen hakediş
	} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF içinde snapshot değeri bulunamadı: %q", want)
		}
	}
}

func TestWMOCondition(t *testing.T) {
	if wmoConditionTR(0) != "Açık" || wmoConditionTR(63) != "Yağmurlu" || wmoConditionTR(96) != "Gök gürültülü fırtına" {
		t.Fatal("WMO kod eşlemesi hatalı")
	}
}
