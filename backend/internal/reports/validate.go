package reports

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// delayReasonCodes — günlük rapordaki gecikme nedeni kodları (E16). Sabit
// liste: rapor ve hakediş/EVM tarafında aynı sınıflandırma kullanılır.
var delayReasonCodes = map[string]bool{
	"Hava": true, "Sahip": true, "Tasarim": true, "Taseron": true,
	"Malzeme": true, "Ekipman": true, "Idari": true, "Diger": true,
}

// validateDelayCauses — gecikme kayıtlarını doğrular; hata alanları fields'a yazılır.
func validateDelayCauses(causes []delayCauseDTO, fields map[string]string) {
	total := 0.0
	for i, c := range causes {
		key := fmt.Sprintf("delay_causes[%d]", i)
		if !delayReasonCodes[c.ReasonCode] {
			fields[key+".reason_code"] = "geçerli bir gecikme nedeni seçin"
		}
		if c.Hours <= 0 || c.Hours > 24 {
			fields[key+".hours"] = "0 ile 24 saat arasında olmalı"
		}
		if utf8.RuneCountInString(c.Note) > 500 {
			fields[key+".note"] = "en fazla 500 karakter"
		}
		total += c.Hours
	}
	if total > 24 {
		fields["delay_causes"] = "toplam gecikme 24 saati aşamaz"
	}
}

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
	for i, s := range in.Staff {
		if strings.TrimSpace(s.AdSoyad) == "" {
			fields[fmt.Sprintf("staff[%d].ad_soyad", i)] = "ad soyad zorunlu"
		}
		if s.Grup != "yuklenici" && s.Grup != "musavir" {
			fields[fmt.Sprintf("staff[%d].grup", i)] = "geçersiz grup (yuklenici/musavir)"
		}
		switch s.Durum {
		case "", "mevcut", "devamsiz", "izinli", "raporlu":
		default:
			fields[fmt.Sprintf("staff[%d].durum", i)] = "geçersiz durum"
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
	validateDelayCauses(in.DelayCauses, fields)
	return fields
}
