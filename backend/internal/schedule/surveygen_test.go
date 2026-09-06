package schedule

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildSurveyPlan_KategoriMantikSirasi(t *testing.T) {
	items := []SurveyItem{
		{ID: uuid.New(), Kategori: "Elektrik", Tanim: "Pano", Sira: 10, Miktar: 1, BirimFiyat: 100},
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "Kolon", Sira: 10, Miktar: 1, BirimFiyat: 100},
		{ID: uuid.New(), Kategori: "Peyzaj", Tanim: "Çim", Sira: 10, Miktar: 1, BirimFiyat: 100},
		{ID: uuid.New(), Kategori: "Diğer", Tanim: "Genel", Sira: 10, Miktar: 1, BirimFiyat: 100},
		{ID: uuid.New(), Kategori: "Mimari", Tanim: "Sıva", Sira: 10, Miktar: 1, BirimFiyat: 100},
	}
	plan := BuildSurveyPlan(items)
	got := make([]string, len(plan))
	for i, c := range plan {
		got[i] = c.Name
	}
	want := []string{"Betonarme", "Mimari", "Elektrik", "Peyzaj", "Diğer"}
	if len(got) != len(want) {
		t.Fatalf("kategori sayısı = %d, beklenen %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sıra[%d] = %q, beklenen %q (tam sıra: %v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildSurveyPlan_BilinmeyenKategoriPeyzajDiginiArasindaFallback(t *testing.T) {
	items := []SurveyItem{
		{ID: uuid.New(), Kategori: "Diğer", Tanim: "X", Miktar: 1, BirimFiyat: 1},
		{ID: uuid.New(), Kategori: "Peyzaj", Tanim: "Y", Miktar: 1, BirimFiyat: 1},
		{ID: uuid.New(), Kategori: "GaripYeniKategori", Tanim: "Z", Miktar: 1, BirimFiyat: 1},
	}
	plan := BuildSurveyPlan(items)
	got := make([]string, len(plan))
	for i, c := range plan {
		got[i] = c.Name
	}
	want := []string{"Peyzaj", "GaripYeniKategori", "Diğer"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sıra[%d] = %q, beklenen %q (tam sıra: %v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildSurveyPlan_KategoriIciSiraSonraTanim(t *testing.T) {
	items := []SurveyItem{
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "B-kalemi", Sira: 20, Miktar: 1, BirimFiyat: 1},
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "A-kalemi", Sira: 10, Miktar: 1, BirimFiyat: 1},
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "Z-kalemi-esit-sira", Sira: 20, Miktar: 1, BirimFiyat: 1},
	}
	plan := BuildSurveyPlan(items)
	if len(plan) != 1 {
		t.Fatalf("1 kategori bekleniyordu, %d geldi", len(plan))
	}
	names := make([]string, len(plan[0].Items))
	for i, it := range plan[0].Items {
		names[i] = it.Name
	}
	want := []string{"A-kalemi", "B-kalemi", "Z-kalemi-esit-sira"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("kalem sırası[%d] = %q, beklenen %q (tam: %v)", i, names[i], want[i], names)
		}
	}
}

func TestBuildSurveyPlan_AgirlikMiktarCarpiBirimFiyatVeKategoriToplami(t *testing.T) {
	items := []SurveyItem{
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "A", Miktar: 10, BirimFiyat: 5}, // 50
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "B", Miktar: 2, BirimFiyat: 25}, // 50
	}
	plan := BuildSurveyPlan(items)
	if plan[0].Weight != 100 {
		t.Fatalf("kategori ağırlığı = %v, beklenen 100", plan[0].Weight)
	}
	if plan[0].Items[0].Weight != 50 || plan[0].Items[1].Weight != 50 {
		t.Fatalf("kalem ağırlıkları yanlış: %+v", plan[0].Items)
	}
}

func TestBuildSurveyPlan_WorkItemIDOlanDerivedIsaretlenir(t *testing.T) {
	wid := uuid.New()
	items := []SurveyItem{
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "Atanmış", Miktar: 1, BirimFiyat: 1, WorkItemID: &wid},
		{ID: uuid.New(), Kategori: "Betonarme", Tanim: "Atanmamış", Miktar: 1, BirimFiyat: 1},
	}
	plan := BuildSurveyPlan(items)
	var atanmis, atanmamis *PlannedItem
	for i := range plan[0].Items {
		it := &plan[0].Items[i]
		if it.Name == "Atanmış" {
			atanmis = it
		} else {
			atanmamis = it
		}
	}
	if atanmis == nil || atanmis.WorkItemID == nil || *atanmis.WorkItemID != wid {
		t.Fatalf("atanmış kalemin WorkItemID'si beklendiği gibi taşınmadı: %+v", atanmis)
	}
	if atanmamis == nil || atanmamis.WorkItemID != nil {
		t.Fatalf("atanmamış kalemin WorkItemID'si nil olmalı: %+v", atanmamis)
	}
}

func TestBuildSurveyPlan_BosGirdiBosSonucVerir(t *testing.T) {
	plan := BuildSurveyPlan(nil)
	if len(plan) != 0 {
		t.Fatalf("boş girdi boş sonuç vermeli, %d kategori geldi", len(plan))
	}
}
