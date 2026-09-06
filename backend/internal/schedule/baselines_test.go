package schedule

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// Kabul kriteri 3 (şartname): "Baseline dondurulduktan sonra tarih değişse
// bile dondurulmuş revizyon değişmiyor." FreezeBaseline'ın DB'ye yazdığı asıl
// []byte, buildBaselineSnapshot'ın (saf, DB'siz) ürettiği JSON'dur — bu test
// DB'ye dokunmadan tam olarak bunu doğrular: snapshot alındıktan SONRA
// kaynak ItemDTO'ları (ve onların işaret ettiği tarih/isim alanlarını)
// mutasyona uğratıp, önceden alınmış JSON'un DEĞİŞMEDİĞİNİ kanıtlar.
func TestBuildBaselineSnapshot_DondurmaSonrasiMutasyonEtkilemez(t *testing.T) {
	itemID := uuid.New()
	start := "2026-01-01"
	items := []ItemDTO{
		{ID: itemID, WbsCode: "1", Name: "Temel", BaselineStart: &start, Weight: 5, Progress: 40},
	}

	snapJSON, err := buildBaselineSnapshot(items)
	if err != nil {
		t.Fatalf("buildBaselineSnapshot hata: %v", err)
	}

	// Dondurma SONRASI: aynı alttaki string'i ve slice elemanını mutasyona uğrat
	// (gerçek dünyada bu, UpdateItem'ın schedule_items satırını değiştirmesine
	// karşılık gelir — snapshot artık o satıra hiçbir şekilde bağlı değildir).
	start = "2027-06-15"
	items[0].Name = "Temel (değişti)"
	items[0].Progress = 99
	mutatedStart := "2099-12-31"
	items[0].BaselineStart = &mutatedStart

	var decoded []baselineSnapshotItem
	if err := json.Unmarshal(snapJSON, &decoded); err != nil {
		t.Fatalf("snapshot JSON çözümlenemedi: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("snapshot %d kalem içeriyor, beklenen 1", len(decoded))
	}
	got := decoded[0]

	if got.Name != "Temel" {
		t.Fatalf("snapshot'taki ad değişmiş: %q, beklenen orijinal %q", got.Name, "Temel")
	}
	if got.Progress != 40 {
		t.Fatalf("snapshot'taki ilerleme değişmiş: %v, beklenen orijinal 40", got.Progress)
	}
	if got.BaselineStart == nil || *got.BaselineStart != "2026-01-01" {
		t.Fatalf("snapshot'taki tarih değişmiş: %v, beklenen orijinal 2026-01-01", got.BaselineStart)
	}
}

// Aynı ilke, çoklu revizyon senaryosu: iki ayrı anda alınan iki snapshot
// birbirinden bağımsız olmalı (ikinci dondurma birinciyi geriye dönük etkilememeli).
func TestBuildBaselineSnapshot_IkiAyriRevizyonBirbirindenBagimsiz(t *testing.T) {
	itemID := uuid.New()
	items := []ItemDTO{{ID: itemID, WbsCode: "1", Name: "Temel", Weight: 1, Progress: 10}}

	rev1, err := buildBaselineSnapshot(items)
	if err != nil {
		t.Fatalf("rev1 hata: %v", err)
	}

	items[0].Progress = 90 // "ilerleme, ikinci revizyon öncesi güncellendi"
	rev2, err := buildBaselineSnapshot(items)
	if err != nil {
		t.Fatalf("rev2 hata: %v", err)
	}

	var d1, d2 []baselineSnapshotItem
	_ = json.Unmarshal(rev1, &d1)
	_ = json.Unmarshal(rev2, &d2)

	if d1[0].Progress != 10 {
		t.Fatalf("rev1'in ilerlemesi rev2'nin etkisiyle değişmiş: %v, beklenen 10", d1[0].Progress)
	}
	if d2[0].Progress != 90 {
		t.Fatalf("rev2 güncel değeri yansıtmıyor: %v, beklenen 90", d2[0].Progress)
	}
}
