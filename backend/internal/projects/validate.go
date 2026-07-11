package projects

// Saf doğrulama fonksiyonları — DB'siz, tam test edilebilir. Handler'lar bunları
// çağırır; testler doğrudan doğrular (Plan: kritik kurallar kod düzeyinde garanti).

// ProjectStatuses / MilestoneStatuses — izinli statü kümeleri (DB CHECK ile birebir).
var ProjectStatuses = map[string]bool{
	"Planning": true, "Active": true, "OnHold": true, "Closed": true, "Archived": true,
}

var MilestoneStatuses = map[string]bool{
	"Planned": true, "InProgress": true, "Completed": true, "Delayed": true,
}

// ValidateProject — proje künyesi alan doğrulaması. Boş map = geçerli.
func ValidateProject(code, name, currency, status string) map[string]string {
	f := map[string]string{}
	if code == "" {
		f["code"] = "zorunlu"
	}
	if name == "" {
		f["name"] = "zorunlu"
	}
	if len(currency) != 3 {
		f["currency"] = "3 harfli ISO kodu olmalı (ör. TRY, USD, EUR)"
	}
	if !ProjectStatuses[status] {
		f["status"] = "geçersiz statü"
	}
	return f
}

// ValidateMilestone — milestone alan doğrulaması.
func ValidateMilestone(name, status string, weightPct *float64) map[string]string {
	f := map[string]string{}
	if name == "" {
		f["name"] = "zorunlu"
	}
	if !MilestoneStatuses[status] {
		f["status"] = "geçersiz statü"
	}
	if weightPct != nil && (*weightPct < 0 || *weightPct > 100) {
		f["weight_pct"] = "0–100 aralığında olmalı"
	}
	return f
}
