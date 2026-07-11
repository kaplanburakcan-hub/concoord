package ohs

import (
	"strings"
)

// --- checklist şablonu -------------------------------------------------------

// TemplateItem — items JSONB'nin tek öğesi. Kontrol tanımı VERİDİR (Plan §7):
// yeni kontrol eklemek kod değil konfigürasyon işidir.
type TemplateItem struct {
	No       int    `json:"no"`
	Text     string `json:"text"`
	Critical bool   `json:"critical,omitempty"` // kritik maddede 'fail' → Major bulgu önerisi (UI)
}

func validateTemplateItems(items []TemplateItem) map[string]string {
	errs := map[string]string{}
	if len(items) == 0 {
		errs["items"] = "en az bir kontrol maddesi gerekli"
		return errs
	}
	seen := map[int]bool{}
	for _, it := range items {
		if it.No <= 0 {
			errs["items"] = "madde numaraları 1'den başlayan pozitif tam sayı olmalı"
			return errs
		}
		if seen[it.No] {
			errs["items"] = "madde numaraları benzersiz olmalı"
			return errs
		}
		seen[it.No] = true
		if strings.TrimSpace(it.Text) == "" {
			errs["items"] = "madde metni boş olamaz"
			return errs
		}
	}
	return errs
}

// --- denetim sonuçları -------------------------------------------------------

// ResultItem — results JSONB'nin tek öğesi.
type ResultItem struct {
	No     int    `json:"no"`
	Answer string `json:"answer"` // ok | fail | na
	Note   string `json:"note,omitempty"`
}

var validAnswers = map[string]bool{"ok": true, "fail": true, "na": true}

// validateResults — her şablon maddesine tam olarak bir yanıt ister.
func validateResults(tmpl []TemplateItem, results []ResultItem) map[string]string {
	errs := map[string]string{}
	byNo := map[int]string{}
	for _, r := range results {
		if !validAnswers[r.Answer] {
			errs["results"] = "geçersiz yanıt (ok/fail/na bekleniyor)"
			return errs
		}
		if _, dup := byNo[r.No]; dup {
			errs["results"] = "aynı maddeye birden çok yanıt verilmiş"
			return errs
		}
		byNo[r.No] = r.Answer
	}
	for _, it := range tmpl {
		if _, ok := byNo[it.No]; !ok {
			errs["results"] = "tüm maddeler yanıtlanmalı (eksik madde var)"
			return errs
		}
	}
	if len(results) != len(tmpl) {
		errs["results"] = "şablonda olmayan maddeye yanıt verilmiş"
		return errs
	}
	return errs
}

// Score — uygunluk yüzdesi: ok / (ok + fail). 'na' payda dışıdır.
// Tüm maddeler 'na' ise skor tanımsızdır (nil döner).
func Score(results []ResultItem) *float64 {
	var ok, applicable int
	for _, r := range results {
		switch r.Answer {
		case "ok":
			ok++
			applicable++
		case "fail":
			applicable++
		}
	}
	if applicable == 0 {
		return nil
	}
	s := float64(int(float64(ok)/float64(applicable)*10000+0.5)) / 100 // 2 hane
	return &s
}

// --- bulgu yaşam döngüsü -----------------------------------------------------

var findingTransitions = map[string][]string{
	"Open":       {"InProgress", "Closed"},
	"InProgress": {"Closed"},
	"Closed":     {},
}

func CanTransitionFinding(from, to string) bool {
	for _, t := range findingTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

var Severities = map[string]bool{
	"Observation": true, "Minor": true, "Major": true, "Critical": true,
}

// --- ceza --------------------------------------------------------------------

var PenaltyLevels = map[string]bool{"Warning": true, "Fine": true}

// validatePenalty — Fine seviyesinde tutar ZORUNLU ve pozitif; Warning'de
// tutar verilmez (tutanak paraya bağlanmaz, kesinti önerisi doğmaz).
func validatePenalty(level string, amount *float64, violationType string) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(violationType) == "" {
		errs["violation_type"] = "ihlal türü zorunludur"
	}
	if !PenaltyLevels[level] {
		errs["penalty_level"] = "Warning ya da Fine olmalı"
		return errs
	}
	if level == "Fine" {
		if amount == nil || *amount <= 0 {
			errs["amount"] = "para cezasında tutar zorunlu ve pozitif olmalı"
		}
	} else if amount != nil {
		errs["amount"] = "uyarı tutanağında tutar girilmez"
	}
	return errs
}
