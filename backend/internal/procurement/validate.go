// Package procurement — Faz 7: Satınalma ve Tedarik Zinciri (Plan §6.6, §8).
//
// Zincir: PR (Draft→Submitted→Approved|Rejected→Converted) → PO
// (Ordered→PartiallyDelivered→Delivered | Cancelled) → teslimatlar.
// Kesinleşmiş kayıtlar DB trigger'larıyla kilitlenir; her geçiş
// workflow_transitions'a yazılır; bildirimler merkezi notify servisiyledir.
package procurement

import (
	"strings"
	"time"
)

type prItemReq struct {
	MaterialName string  `json:"material_name"`
	Spec         *string `json:"spec"`
	Qty          float64 `json:"qty"`
	Unit         string  `json:"unit"`
	Note         *string `json:"note"`
}

type prReq struct {
	NeededByDate string      `json:"needed_by_date"` // YYYY-MM-DD
	Note         *string     `json:"note"`
	Items        []prItemReq `json:"items"`
}

// validatePR — PR başlık + kalem doğrulaması. Alan bazlı hata haritası döner
// (httpx.ValidationFailed sözleşmesi).
func validatePR(req prReq) (time.Time, map[string]string) {
	errs := map[string]string{}
	needed, err := time.Parse("2006-01-02", strings.TrimSpace(req.NeededByDate))
	if err != nil {
		errs["needed_by_date"] = "geçerli bir tarih girin (YYYY-MM-DD)"
	}
	if len(req.Items) == 0 {
		errs["items"] = "en az bir kalem girin"
	}
	if len(req.Items) > 200 {
		errs["items"] = "en çok 200 kalem girilebilir"
	}
	for i, it := range req.Items {
		if strings.TrimSpace(it.MaterialName) == "" {
			errs["items"] = "kalem " + itoa(i+1) + ": malzeme adı zorunludur"
			break
		}
		if it.Qty <= 0 {
			errs["items"] = "kalem " + itoa(i+1) + ": miktar sıfırdan büyük olmalıdır"
			break
		}
		if strings.TrimSpace(it.Unit) == "" {
			errs["items"] = "kalem " + itoa(i+1) + ": birim zorunludur"
			break
		}
	}
	return needed, errs
}

type poReq struct {
	SupplierName string   `json:"supplier_name"`
	Amount       *float64 `json:"amount"`
	Currency     *string  `json:"currency"`
	ExpectedDate *string  `json:"expected_date"` // YYYY-MM-DD | null
	Note         *string  `json:"note"`
}

// validatePO — PO başlık doğrulaması. expected nil olabilir (tarih belirsiz).
func validatePO(req poReq) (*time.Time, map[string]string) {
	errs := map[string]string{}
	if strings.TrimSpace(req.SupplierName) == "" {
		errs["supplier_name"] = "tedarikçi adı zorunludur"
	}
	if req.Amount != nil && *req.Amount < 0 {
		errs["amount"] = "tutar negatif olamaz"
	}
	if req.Currency != nil && *req.Currency != "" && len(strings.TrimSpace(*req.Currency)) != 3 {
		errs["currency"] = "para birimi 3 harfli ISO kodu olmalıdır (örn. TRY)"
	}
	var expected *time.Time
	if req.ExpectedDate != nil && strings.TrimSpace(*req.ExpectedDate) != "" {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(*req.ExpectedDate))
		if err != nil {
			errs["expected_date"] = "geçerli bir tarih girin (YYYY-MM-DD)"
		} else {
			expected = &t
		}
	}
	return expected, errs
}

type deliveryReq struct {
	DeliveryNoteNo string  `json:"delivery_note_no"`
	DeliveredAt    *string `json:"delivered_at"` // RFC3339 | null (=now)
	DocumentID     *string `json:"document_id"`  // irsaliye fotoğrafı (Faz 2 doküman)
	Note           *string `json:"note"`
	MarkDelivered  bool    `json:"mark_delivered"` // bu teslimat siparişi kapatır
}

func validateDelivery(req deliveryReq) (time.Time, map[string]string) {
	errs := map[string]string{}
	if strings.TrimSpace(req.DeliveryNoteNo) == "" {
		errs["delivery_note_no"] = "irsaliye numarası zorunludur"
	}
	at := time.Now()
	if req.DeliveredAt != nil && strings.TrimSpace(*req.DeliveredAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.DeliveredAt))
		if err != nil {
			errs["delivered_at"] = "geçerli bir zaman girin (RFC3339)"
		} else {
			at = t
		}
	}
	return at, errs
}

// itoa — küçük yardımcı (strconv importu tek kullanım için gereksiz değil ama
// hata mesajı üretimini sade tutar).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
