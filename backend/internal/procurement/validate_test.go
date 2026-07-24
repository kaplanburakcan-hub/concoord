package procurement

import "testing"

func TestValidatePR(t *testing.T) {
	ok := prReq{NeededByDate: "2026-08-01", Items: []prItemReq{
		{MaterialName: "C30 hazır beton", Qty: 120, Unit: "m³"},
	}}
	if _, errs := validatePR(ok); len(errs) != 0 {
		t.Fatalf("geçerli PR reddedildi: %v", errs)
	}

	cases := []struct {
		name  string
		req   prReq
		field string
	}{
		{"tarih yok", prReq{Items: ok.Items}, "needed_by_date"},
		{"bozuk tarih", prReq{NeededByDate: "01.08.2026", Items: ok.Items}, "needed_by_date"},
		{"kalem yok", prReq{NeededByDate: "2026-08-01"}, "items"},
		{"adsız kalem", prReq{NeededByDate: "2026-08-01",
			Items: []prItemReq{{MaterialName: "  ", Qty: 1, Unit: "ad"}}}, "items"},
		{"sıfır miktar", prReq{NeededByDate: "2026-08-01",
			Items: []prItemReq{{MaterialName: "Çimento", Qty: 0, Unit: "ton"}}}, "items"},
		{"birimsiz", prReq{NeededByDate: "2026-08-01",
			Items: []prItemReq{{MaterialName: "Çimento", Qty: 5, Unit: ""}}}, "items"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, errs := validatePR(c.req); errs[c.field] == "" {
				t.Fatalf("%s alanında hata bekleniyordu, errs=%v", c.field, errs)
			}
		})
	}
}

func TestValidatePO(t *testing.T) {
	amt := 250000.0
	exp := "2026-07-20"
	if e, errs := validatePO(poReq{SupplierName: "Yılmaz İnşaat Malz.", Amount: &amt, ExpectedDate: &exp}); len(errs) != 0 || e == nil {
		t.Fatalf("geçerli PO reddedildi: %v", errs)
	}
	if _, errs := validatePO(poReq{SupplierName: " "}); errs["supplier_name"] == "" {
		t.Fatal("tedarikçi adı zorunluluğu yakalanmadı")
	}
	neg := -1.0
	if _, errs := validatePO(poReq{SupplierName: "X", Amount: &neg}); errs["amount"] == "" {
		t.Fatal("negatif tutar yakalanmadı")
	}
	badCur := "TL"
	if _, errs := validatePO(poReq{SupplierName: "X", Currency: &badCur}); errs["currency"] == "" {
		t.Fatal("geçersiz para birimi yakalanmadı")
	}
	badDate := "20/07/2026"
	if _, errs := validatePO(poReq{SupplierName: "X", ExpectedDate: &badDate}); errs["expected_date"] == "" {
		t.Fatal("geçersiz beklenen tarih yakalanmadı")
	}
}

func TestValidateDelivery(t *testing.T) {
	// Faz 11: mal kabulde iki kanıt zorunlu — irsaliye ve malzeme fotoğrafı.
	doc, mat := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	ok := deliveryReq{
		DeliveryNoteNo:          "İRS-2026-0042",
		DocumentID:              &doc,
		MaterialPhotoDocumentID: &mat,
	}
	if _, errs := validateDelivery(ok); len(errs) != 0 {
		t.Fatalf("geçerli teslimat reddedildi: %v", errs)
	}
	if _, errs := validateDelivery(deliveryReq{}); errs["delivery_note_no"] == "" {
		t.Fatal("irsaliye no zorunluluğu yakalanmadı")
	}
	bad := "dün"
	r := ok
	r.DeliveredAt = &bad
	if _, errs := validateDelivery(r); errs["delivered_at"] == "" {
		t.Fatal("geçersiz teslim zamanı yakalanmadı")
	}

	// --- Faz 11 kuralları ---
	if _, errs := validateDelivery(deliveryReq{DeliveryNoteNo: "A"}); errs["document_id"] == "" ||
		errs["material_photo_document_id"] == "" {
		t.Fatal("zorunlu fotoğraflar yakalanmadı")
	}

	// Uygunsuz teslimatta açıklama zorunludur.
	short := ok
	short.Condition = "Short"
	if _, errs := validateDelivery(short); errs["discrepancy_note"] == "" {
		t.Fatal("eksik teslimatta açıklama zorunluluğu yakalanmadı")
	}
	note := "3 palet eksik geldi, tedarikçi bilgilendirildi."
	short.DiscrepancyNote = &note
	if _, errs := validateDelivery(short); len(errs) != 0 {
		t.Fatalf("açıklamalı eksik teslimat reddedildi: %v", errs)
	}

	// Kabul + red, gelen miktarı aşamaz.
	over := ok
	over.Items = []deliveryItemReq{
		{MaterialName: "Çimento", ReceivedQty: 10, AcceptedQty: 8, RejectedQty: 5},
	}
	if _, errs := validateDelivery(over); errs["items[0].qty"] == "" {
		t.Fatal("miktar tutarsızlığı yakalanmadı")
	}

	// Geçersiz teslim yeri ve durumu.
	badType := ok
	badType.ReceiptType = "Uzay"
	if _, errs := validateDelivery(badType); errs["receipt_type"] == "" {
		t.Fatal("geçersiz teslim yeri yakalanmadı")
	}
	badCond := ok
	badCond.Condition = "Kayıp"
	if _, errs := validateDelivery(badCond); errs["condition"] == "" {
		t.Fatal("geçersiz teslim durumu yakalanmadı")
	}
}

func TestItoa(t *testing.T) {
	for n, want := range map[int]string{0: "0", 1: "1", 12: "12", 199: "199"} {
		if got := itoa(n); got != want {
			t.Fatalf("itoa(%d)=%s, beklenen %s", n, got, want)
		}
	}
}
