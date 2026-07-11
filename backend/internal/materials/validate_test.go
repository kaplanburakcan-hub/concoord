package materials

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateMAR(t *testing.T) {
	if errs := validateMAR("C30 Beton"); len(errs) != 0 {
		t.Fatalf("geçerli künye hata verdi: %v", errs)
	}
	if errs := validateMAR("   "); errs["material_name"] == "" {
		t.Fatal("boş malzeme adı yakalanmadı")
	}
	if errs := validateMAR(strings.Repeat("a", 301)); errs["material_name"] == "" {
		t.Fatal("aşırı uzun malzeme adı yakalanmadı")
	}
}

func TestValidateDecision_NoteZorunlu(t *testing.T) {
	// Plan Faz 5: karar notu zorunluluğu — notu boş karar reddedilir.
	if errs := validateDecision("Approved", "  "); errs["decision_note"] == "" {
		t.Fatal("boş karar notu yakalanmadı")
	}
	if errs := validateDecision("Approved", "TSE belgesi uygun."); len(errs) != 0 {
		t.Fatalf("geçerli karar hata verdi: %v", errs)
	}
}

func TestValidateDecision_GecerliSonuclar(t *testing.T) {
	for _, d := range []string{"Approved", "ConditionallyApproved", "Rejected"} {
		if errs := validateDecision(d, "not"); len(errs) != 0 {
			t.Fatalf("%s geçerli olmalıydı: %v", d, errs)
		}
	}
	if errs := validateDecision("Maybe", "not"); errs["decision"] == "" {
		t.Fatal("geçersiz karar yakalanmadı")
	}
	// Draft/UnderReview karar DEĞİLDİR — karar ucu bunları kabul etmez.
	if errs := validateDecision("UnderReview", "not"); errs["decision"] == "" {
		t.Fatal("UnderReview karar olarak kabul edildi")
	}
}

func TestResolveSubID_TaseronKapsamiZorlanir(t *testing.T) {
	// Satır seviyesi güvenlik: SubcontractorRep başka firma adına MAR açamaz.
	mine := uuid.New()
	other := uuid.New().String()
	got, errs := resolveSubID(&other, &mine)
	if len(errs) != 0 || got == nil || *got != mine {
		t.Fatalf("taşeron kapsamı zorlanmadı: got=%v errs=%v", got, errs)
	}

	// Kısıtsız kullanıcı: istekteki firma kullanılır.
	got, errs = resolveSubID(&other, nil)
	if len(errs) != 0 || got == nil || got.String() != other {
		t.Fatalf("kısıtsız çözümleme hatalı: got=%v errs=%v", got, errs)
	}

	// Boş istek + kısıtsız: firma bağsız (iç MAR).
	got, errs = resolveSubID(nil, nil)
	if len(errs) != 0 || got != nil {
		t.Fatalf("boş istek nil dönmeliydi: got=%v errs=%v", got, errs)
	}

	bad := "abc"
	if _, errs := resolveSubID(&bad, nil); errs["subcontractor_id"] == "" {
		t.Fatal("geçersiz UUID yakalanmadı")
	}
}
