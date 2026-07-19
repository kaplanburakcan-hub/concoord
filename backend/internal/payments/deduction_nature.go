package payments

// Faz 11 — Kesinti niteliği (kâti / geçici / mahsup) ve maliyet etkisi.
//
// Türk inşaat uygulamasında taşerondan yapılan kesintiler üç farklı ekonomik
// nitelik taşır ve bunları ayırmak hem hakediş doğruluğu hem EVM için şarttır:
//
//   Offset (mahsup)    — Avans mahsubu. Avans daha ÖNCE ödenmiştir; bu satır o
//                        tutarın geri alınmasıdır. İade yükümlülüğü doğurmaz,
//                        işin maliyetini de azaltmaz (nakit akışı kalemi).
//
//   Temporary (geçici) — Teminat (nakdi teminat / stopaj teminatı). Taşerondan
//                        tutulur ve geçici/kesin kabulde İADE EDİLİR. Ana
//                        yüklenici için borç niteliğindedir; maliyeti azaltmaz.
//
//   Permanent (kâti)   — İade edilmez. İki alt küme vardır:
//                        · maliyeti AZALTANLAR: taşerona verilen mal/hizmet
//                          bedeli (yemek, konaklama, elektrik, su, malzeme) ve
//                          cezalar (İSG cezası, gecikme cezası).
//                        · maliyeti AZALTMAYANLAR: stopaj ve benzeri vergiler.
//                          Stopaj taşeronun gelir vergisi mahsubudur; kaynakta
//                          kesilip onun adına yatırılır. Ana yüklenicinin işe
//                          maliyeti brüt tutar olarak kalır.
//
// EVM Gerçekleşen Maliyet (AC) yalnızca "maliyeti azaltan" kesintileri düşer:
//   AC = GrossThis − Σ(reduces_cost olan kesintiler)      [KDV hariç]

const (
	NatureOffset    = "Offset"
	NatureTemporary = "Temporary"
	NaturePermanent = "Permanent"
)

// DeductionNature — kesinti tipinden varsayılan niteliği türetir.
// "Other" kâti kabul edilir; çağıran gerekiyorsa açıkça geçersiz kılabilir
// (ör. sözleşmeye özel iade edilecek bir kesinti).
func DeductionNature(t string) string {
	switch t {
	case "AdvanceOffset":
		return NatureOffset
	case "Retention":
		return NatureTemporary
	default: // Withholding, Tax, OHSPenalty, Other
		return NaturePermanent
	}
}

// DeductionReducesCost — kesinti EVM Gerçekleşen Maliyet (AC) hesabından
// düşülmeli mi? Yalnızca taşerona sağlanan mal/hizmet bedeli ve cezalar düşülür.
func DeductionReducesCost(t string) bool {
	switch t {
	case "OHSPenalty", "Other":
		return true
	default: // AdvanceOffset, Retention, Withholding, Tax
		return false
	}
}

// NatureLabelTR — arayüz/rapor gösterimi için Türkçe etiket.
func NatureLabelTR(n string) string {
	switch n {
	case NatureOffset:
		return "Mahsup"
	case NatureTemporary:
		return "Geçici (iade edilecek)"
	case NaturePermanent:
		return "Kâti"
	default:
		return n
	}
}

// ResolveWithholding — bu hakedişte stopaj uygulanacak mı?
// paymentOverride hakediş düzeyindeki elle işaretlemedir (nil = işaretlenmemiş);
// nil ise sözleşmenin yıllara sari (is_multi_year) varsayılanı geçerlidir.
func ResolveWithholding(paymentOverride *bool, contractMultiYear bool) bool {
	if paymentOverride != nil {
		return *paymentOverride
	}
	return contractMultiYear
}
