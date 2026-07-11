package payments

import "strings"

// Saf doğrulama — DB'siz, tam test edilebilir (projects paketindeki desenle aynı).

var ContractTypes = map[string]bool{"Main": true, "Sub": true, "Addendum": true}

var PaymentStatuses = map[string]bool{
	"Draft": true, "Submitted": true, "SiteApproved": true, "Finalized": true, "Rejected": true,
}

var DeductionTypes = map[string]bool{
	"AdvanceOffset": true, "Retention": true, "Tax": true, "OHSPenalty": true, "Other": true,
}

// İş akışı geçiş grafiği (Plan §6.4). Kendi durumuna geçiş yok; ileri akış +
// reddetme + taslağa geri çekme desteklenir.
var paymentTransitions = map[string]map[string]bool{
	"Draft":        {"Submitted": true},
	"Submitted":    {"SiteApproved": true, "Rejected": true, "Draft": true},
	"SiteApproved": {"Finalized": true, "Rejected": true, "Submitted": true},
	"Rejected":     {"Draft": true},
	"Finalized":    {}, // terminal — DB trigger de kilitler
}

// CanTransition — from→to geçişi izinli mi?
func CanTransition(from, to string) bool {
	m, ok := paymentTransitions[from]
	if !ok {
		return false
	}
	return m[to]
}

func ValidateSubcontractor(companyName string) map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(companyName) == "" {
		f["company_name"] = "zorunlu"
	}
	return f
}

func ValidateContract(contractNo, ctype string, retentionPct, advanceRatePct float64) map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(contractNo) == "" {
		f["contract_no"] = "zorunlu"
	}
	if !ContractTypes[ctype] {
		f["type"] = "geçersiz tür (Main|Sub|Addendum)"
	}
	if retentionPct < 0 || retentionPct > 100 {
		f["retention_pct"] = "0–100 aralığında olmalı"
	}
	if advanceRatePct < 0 || advanceRatePct > 100 {
		f["advance_rate_pct"] = "0–100 aralığında olmalı"
	}
	return f
}

func ValidateWorkItem(pozNo, description string, contractQty, unitPrice float64) map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(pozNo) == "" {
		f["poz_no"] = "zorunlu"
	}
	if strings.TrimSpace(description) == "" {
		f["description"] = "zorunlu"
	}
	if contractQty < 0 {
		f["contract_qty"] = "negatif olamaz"
	}
	if unitPrice < 0 {
		f["unit_price"] = "negatif olamaz"
	}
	return f
}
