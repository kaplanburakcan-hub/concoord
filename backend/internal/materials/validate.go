package materials

import (
	"strings"

	"github.com/google/uuid"
)

// Geçerli karar sonuçları (Plan §6.5).
var decisions = map[string]bool{
	"Approved":              true,
	"ConditionallyApproved": true,
	"Rejected":              true,
}

var statusLabels = map[string]string{
	"Submitted":             "Sunuldu",
	"UnderReview":           "İncelemede",
	"Approved":              "Onaylandı",
	"ConditionallyApproved": "Şartlı Onay",
	"Rejected":              "Reddedildi",
}

func statusLabel(s string) string {
	if l, ok := statusLabels[s]; ok {
		return l
	}
	return s
}

// validateMAR — künye doğrulaması.
func validateMAR(materialName string) map[string]string {
	errs := map[string]string{}
	name := strings.TrimSpace(materialName)
	if name == "" {
		errs["material_name"] = "malzeme adı zorunlu"
	} else if len(name) > 300 {
		errs["material_name"] = "en fazla 300 karakter"
	}
	return errs
}

// validateDecision — karar + zorunlu karar notu (Plan Faz 5: karar notu zorunluluğu).
func validateDecision(decision, note string) map[string]string {
	errs := map[string]string{}
	if !decisions[decision] {
		errs["decision"] = "geçersiz karar (Approved | ConditionallyApproved | Rejected)"
	}
	if strings.TrimSpace(note) == "" {
		errs["decision_note"] = "karar notu zorunludur"
	} else if len(note) > 2000 {
		errs["decision_note"] = "en fazla 2000 karakter"
	}
	return errs
}

// resolveSubID — istekten gelen taşeron kimliğini kapsamla bağdaştırır.
// SubcontractorRep (scopedSub dolu) yalnızca KENDİ firması adına MAR açabilir;
// istekte farklı/boş firma verilmişse kendi firması zorlanır.
func resolveSubID(reqSub *string, scoped *uuid.UUID) (*uuid.UUID, map[string]string) {
	if scoped != nil {
		return scoped, nil
	}
	if reqSub == nil || strings.TrimSpace(*reqSub) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*reqSub))
	if err != nil {
		return nil, map[string]string{"subcontractor_id": "geçersiz UUID"}
	}
	return &id, nil
}
