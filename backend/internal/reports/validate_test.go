package reports

import (
	"bytes"
	"context"
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
// doğrulanabilir. PDF girdisi (weeklyPDFData) snapshot değerlerini birebir taşır;
// çıktı geçerli bir PDF iskeletidir.
func TestWeeklyPDFFromSnapshot(t *testing.T) {
	sub := "Yılmaz İnşaat"
	sn := Snapshot{
		GeneratedAt: time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC),
		ProjectName: "Kule Ofis", ProjectCode: "PRJ-01",
		WeekNo: 27, PeriodStart: "2026-06-29", PeriodEnd: "2026-07-05",
		Days: []SnapDay{
			{Date: "2026-06-29", RevisionNo: 1, Status: "Submitted", ManpowerTotal: 14, WeatherCond: "Açık",
				Manpower:  []manpowerDTO{{Trade: "Demirci", Headcount: 14, SubcontractorName: &sub}},
				Equipment: []equipmentDTO{{EquipmentName: "Beton pompası", Count: 2, WorkingHours: f(7.5)}},
				WorkEntries: []workEntryDTO{{Description: "A blok temel demir bağlama", Qty: f(1250)}},
			},
			{Date: "2026-06-30", RevisionNo: 2, Status: "Submitted", ManpowerTotal: 9},
		},
		Totals: SnapTotals{DaysReported: 2, ManpowerDays: 23, EquipmentHours: 7.5, WorkEntryCount: 1},
		DelayHours: 3.5, PPC: &SnapPPC{Planned: 10, Completed: 7},
		PendingPayments: []SnapPayment{{Subcontractor: "Yılmaz İnşaat", PeriodNo: 3, Status: "Submitted"}},
		OpenTasks: 5, TasksDueWeek: 2, PendingMARs: 1,
	}

	d := weeklyPDFData(sn)
	if d.WeekNo != 27 || d.DaysReported != 2 || d.ManpowerDays != 23 || d.EquipHours != 7.5 ||
		d.DelayHours != 3.5 || d.PPCPlanned != 10 || d.PPCCompleted != 7 {
		t.Fatalf("snapshot değerleri PDF girdisine taşınmadı: %+v", d)
	}
	if len(d.Days) != 7 || !d.Days[0].Reported || d.Days[0].Manpower != 14 || d.Days[6].Reported {
		t.Fatalf("gün gün dağılım hatalı: %+v", d.Days)
	}
	if len(d.PendingPayments) != 1 || d.PendingPayments[0] != "Yılmaz İnşaat #3" {
		t.Fatalf("bekleyen hakediş hatalı: %v", d.PendingPayments)
	}

	pdf := buildWeeklyPDF(context.Background(), nil, sn)
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF")) {
		t.Fatal("geçerli PDF iskeleti üretilmedi")
	}
}

func TestWMOCondition(t *testing.T) {
	if wmoConditionTR(0) != "Açık" || wmoConditionTR(63) != "Yağmurlu" || wmoConditionTR(96) != "Gök gürültülü fırtına" {
		t.Fatal("WMO kod eşlemesi hatalı")
	}
}
