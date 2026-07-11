package reports

import (
	"fmt"
	"strings"
	"time"
)

// validateDaily — günlük rapor girdisi doğrulaması. Alan adı → hata mesajı.
func validateDaily(in dailyInput) map[string]string {
	fields := map[string]string{}

	if _, err := time.Parse("2006-01-02", in.ReportDate); err != nil {
		fields["report_date"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	} else if d, _ := time.Parse("2006-01-02", in.ReportDate); d.After(time.Now().Add(24 * time.Hour)) {
		fields["report_date"] = "gelecek tarihe rapor girilemez"
	}

	if in.TempMin != nil && (*in.TempMin < -60 || *in.TempMin > 60) {
		fields["temperature_min"] = "geçerli bir sıcaklık girin (-60..60)"
	}
	if in.TempMax != nil && (*in.TempMax < -60 || *in.TempMax > 60) {
		fields["temperature_max"] = "geçerli bir sıcaklık girin (-60..60)"
	}
	if in.TempMin != nil && in.TempMax != nil && *in.TempMin > *in.TempMax {
		fields["temperature_max"] = "en yüksek sıcaklık en düşükten küçük olamaz"
	}

	for i, m := range in.Manpower {
		if strings.TrimSpace(m.Trade) == "" {
			fields[fmt.Sprintf("manpower[%d].trade", i)] = "meslek/branş zorunlu"
		}
		if m.Headcount < 0 {
			fields[fmt.Sprintf("manpower[%d].headcount", i)] = "kişi sayısı negatif olamaz"
		}
	}
	for i, e := range in.Equipment {
		if strings.TrimSpace(e.EquipmentName) == "" {
			fields[fmt.Sprintf("equipment[%d].equipment_name", i)] = "ekipman adı zorunlu"
		}
		if e.Count < 0 {
			fields[fmt.Sprintf("equipment[%d].count", i)] = "adet negatif olamaz"
		}
		if e.WorkingHours != nil && (*e.WorkingHours < 0 || *e.WorkingHours > 24*100) {
			fields[fmt.Sprintf("equipment[%d].working_hours", i)] = "geçersiz çalışma saati"
		}
	}
	for i, w := range in.WorkEntries {
		if strings.TrimSpace(w.Description) == "" {
			fields[fmt.Sprintf("work_entries[%d].description", i)] = "imalat açıklaması zorunlu"
		}
		if w.Qty != nil && *w.Qty < 0 {
			fields[fmt.Sprintf("work_entries[%d].qty", i)] = "miktar negatif olamaz"
		}
	}
	return fields
}
