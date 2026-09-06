package schedule

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func f64(v float64) *float64 { return &v }

func TestComputeProgress_AgirlikliOrtalama(t *testing.T) {
	parent := uuid.New()
	c1, c2 := uuid.New(), uuid.New()
	items := []ItemAgg{
		{ID: parent},
		{ID: c1, ParentID: &parent, ProgressSource: "manual", ManualProgress: f64(40), Weight: 2},
		{ID: c2, ParentID: &parent, ProgressSource: "manual", ManualProgress: f64(80), Weight: 3},
	}
	got := ComputeProgress(items)
	// (40*2 + 80*3) / 5 = 64
	if got[parent] != 64 {
		t.Fatalf("üst kalem ağırlıklı ortalama = %v, beklenen 64", got[parent])
	}
	if got[c1] != 40 || got[c2] != 80 {
		t.Fatalf("yaprak kalemler manual_progress'i yansıtmalı, got c1=%v c2=%v", got[c1], got[c2])
	}
}

func TestComputeProgress_TumAgirliklarSifirsaDuzOrtalama(t *testing.T) {
	parent := uuid.New()
	c1, c2 := uuid.New(), uuid.New()
	items := []ItemAgg{
		{ID: parent},
		{ID: c1, ParentID: &parent, ProgressSource: "manual", ManualProgress: f64(20)},
		{ID: c2, ParentID: &parent, ProgressSource: "manual", ManualProgress: f64(60)},
	}
	got := ComputeProgress(items)
	if got[parent] != 40 {
		t.Fatalf("ağırlıksız durumda düz ortalama = %v, beklenen 40", got[parent])
	}
}

func TestComputeProgress_DerivedYaprakSozlesmeMiktarindanHesaplar(t *testing.T) {
	leaf := uuid.New()
	items := []ItemAgg{
		{ID: leaf, ProgressSource: "derived", ContractQtySum: 200, CumQtySum: 50},
	}
	got := ComputeProgress(items)
	if got[leaf] != 25 {
		t.Fatalf("derived ilerleme = %v, beklenen 25 (50/200*100)", got[leaf])
	}
}

func TestComputeProgress_SozlesmeMiktariSifirsaSifirKabulEdilir(t *testing.T) {
	leaf := uuid.New()
	items := []ItemAgg{
		{ID: leaf, ProgressSource: "derived", ContractQtySum: 0, CumQtySum: 0},
	}
	got := ComputeProgress(items)
	if got[leaf] != 0 {
		t.Fatalf("sözleşme miktarı 0 iken ilerleme %v, beklenen 0", got[leaf])
	}
}

func TestComputeProgress_YuzdeAraligaSikistirilir(t *testing.T) {
	leaf := uuid.New()
	// Kayıt hatasıyla cum_qty > contract_qty olsa bile (örn. metraj düzeltmesi
	// öncesi geçici tutarsızlık), sonuç 100'ü geçmemeli.
	items := []ItemAgg{
		{ID: leaf, ProgressSource: "derived", ContractQtySum: 100, CumQtySum: 150},
	}
	got := ComputeProgress(items)
	if got[leaf] != 100 {
		t.Fatalf("ilerleme 100'e sıkıştırılmalı, got %v", got[leaf])
	}
}

func TestComputeProgress_BozukParentDonguyuSonsuzDonguyeSokmaz(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	// Bozuk/tutarsız veri: a'nın parent'ı b, b'nin parent'ı a (normalde DB
	// düzeyinde böyle bir kayıt oluşamaz, ama savunmacı olmalı).
	items := []ItemAgg{
		{ID: a, ParentID: &b, ProgressSource: "manual", ManualProgress: f64(30)},
		{ID: b, ParentID: &a, ProgressSource: "manual", ManualProgress: f64(70)},
	}
	done := make(chan map[uuid.UUID]float64, 1)
	go func() { done <- ComputeProgress(items) }()
	select {
	case <-done:
		// sonsuz döngüye girmeden döndü — yeterli.
	case <-time.After(2 * time.Second):
		t.Fatal("ComputeProgress bozuk parent_id döngüsünde sonsuz döngüye girdi")
	}
}

// Kabul kriteri 2 (şartname): "Bir poza ait hakediş onaylandığında, bağlı WBS
// kaleminin ilerlemesi sayfa yenilendiğinde otomatik değişiyor." İlerleme
// materialized view/cache OLMADIĞI için (bkz. progress.go), bunu ispatlamanın
// yolu: AYNI girdi kümesinde yalnızca hakedişten gelen CumQtySum değiştiğinde,
// bir sonraki ComputeProgress çağrısının (elle invalidation olmadan) yeni
// değeri yansıtması.
func TestComputeProgress_HakedisOnaylandiginda_IlerlemeOtomatikDegisir(t *testing.T) {
	leaf := uuid.New()
	before := ComputeProgress([]ItemAgg{
		{ID: leaf, ProgressSource: "derived", ContractQtySum: 1000, CumQtySum: 100},
	})[leaf]
	if before != 10 {
		t.Fatalf("onay öncesi ilerleme = %v, beklenen 10", before)
	}

	// Hakediş onaylandı: LoadItemAggregates artık daha yüksek bir cum_qty
	// döndürecek (bkz. LoadItemAggregates'in DISTINCT ON ... status='Finalized'
	// filtresi) — burada aynı etkiyi girdiyi değiştirerek simüle ediyoruz.
	after := ComputeProgress([]ItemAgg{
		{ID: leaf, ProgressSource: "derived", ContractQtySum: 1000, CumQtySum: 400},
	})[leaf]
	if after != 40 {
		t.Fatalf("onay sonrası ilerleme = %v, beklenen 40", after)
	}
	if after == before {
		t.Fatal("hakediş onayı sonrası ilerleme değişmedi — cache/stale sonuç şüphesi")
	}
}

// Kabul kriteri 1 (şartname): "200 kalemlik bir WBS ağacında ilerleme hesabı
// 500 ms altında dönüyor." ComputeProgress saf/DB'siz olduğundan bu, gerçek
// bir tavan; DB sorgusu (LoadItemAggregates) dahil gerçek uçtan uca süre her
// zaman bundan yüksektir ama tek bir sorgu + O(n) saf hesaplama olduğundan
// birkaç yüz kalemde pratikte sorun yaratmaz (bkz. Adım 2 canlı doğrulama).
func TestComputeProgress_200KalemlikAgacta500msAltinda(t *testing.T) {
	items := build200ItemTree()

	start := time.Now()
	result := ComputeProgress(items)
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Fatalf("ComputeProgress 200 kalemde %v sürdü, beklenen < 500ms", elapsed)
	}
	if len(result) != len(items) {
		t.Fatalf("sonuç kümesi eksik: %d/%d kalem", len(result), len(items))
	}
}

// build200ItemTree — 1 kök + 9 dal (her biri ağırlıklı) + dallara dağıtılmış
// 190 yaprak = tam 200 kalem. Yapraklar dönüşümlü olarak manual/derived.
func build200ItemTree() []ItemAgg {
	const branchCount = 9
	const leafCount = 190
	root := uuid.New()
	items := make([]ItemAgg, 0, 1+branchCount+leafCount)
	items = append(items, ItemAgg{ID: root})

	branches := make([]uuid.UUID, branchCount)
	for i := range branches {
		branches[i] = uuid.New()
		items = append(items, ItemAgg{ID: branches[i], ParentID: &root, Weight: float64(i + 1)})
	}
	for j := 0; j < leafCount; j++ {
		leaf := uuid.New()
		branch := branches[j%branchCount]
		if j%2 == 0 {
			pct := float64(j % 101)
			items = append(items, ItemAgg{
				ID: leaf, ParentID: &branch, ProgressSource: "manual",
				ManualProgress: &pct, Weight: float64(j%5 + 1),
			})
		} else {
			items = append(items, ItemAgg{
				ID: leaf, ParentID: &branch, ProgressSource: "derived",
				ContractQtySum: 100, CumQtySum: float64(j % 100), Weight: float64(j%5 + 1),
			})
		}
	}
	if len(items) != 1+branchCount+leafCount {
		panic(fmt.Sprintf("test kurulum hatası: %d kalem üretildi, beklenen %d", len(items), 1+branchCount+leafCount))
	}
	return items
}
