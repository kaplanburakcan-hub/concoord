package schedule

import (
	"testing"

	"github.com/google/uuid"
)

// Kabul kriteri 4 (şartname): "Döngüsel bağımlılık eklenemiyor."
func TestWouldCreateCycle(t *testing.T) {
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	t.Run("bos_grafikte_dongu_yok", func(t *testing.T) {
		if WouldCreateCycle(nil, a, b) {
			t.Fatal("boş grafikte a->b döngü oluşturmamalı")
		}
	})

	t.Run("dogrudan_ters_kenar_donguyu_kapatir", func(t *testing.T) {
		existing := []Edge{{Predecessor: a, Successor: b}}
		if !WouldCreateCycle(existing, b, a) {
			t.Fatal("a->b varken b->a eklemek döngü oluşturmalı")
		}
	})

	t.Run("gecisken_zincir_donguyu_kapatir", func(t *testing.T) {
		// a->b->c zaten var; c->a eklenirse a->b->c->a döngüsü oluşur.
		existing := []Edge{{Predecessor: a, Successor: b}, {Predecessor: b, Successor: c}}
		if !WouldCreateCycle(existing, c, a) {
			t.Fatal("a->b->c varken c->a eklemek geçişken bir döngü oluşturmalı")
		}
	})

	t.Run("kendine_bagimlilik_her_zaman_dongu", func(t *testing.T) {
		if !WouldCreateCycle(nil, a, a) {
			t.Fatal("bir kalemin kendine bağımlı olması her zaman döngü sayılmalı")
		}
	})

	t.Run("ilgisiz_zincir_engellemez", func(t *testing.T) {
		// a->b->c var; d bağımsız bir düğüm — d->a hiçbir döngü oluşturmaz.
		existing := []Edge{{Predecessor: a, Successor: b}, {Predecessor: b, Successor: c}}
		if WouldCreateCycle(existing, d, a) {
			t.Fatal("ilgisiz bir kenar yanlışlıkla döngü olarak işaretlendi")
		}
	})

	t.Run("paralel_bagimlilik_engellemez", func(t *testing.T) {
		// a->c ve b->c aynı successor'ı paylaşıyor; a->b eklemek döngü OLUŞTURMAZ.
		existing := []Edge{{Predecessor: a, Successor: c}, {Predecessor: b, Successor: c}}
		if WouldCreateCycle(existing, a, b) {
			t.Fatal("ortak successor'lu paralel bağımlılık yanlışlıkla döngü sayıldı")
		}
	})

	t.Run("ayni_kenarin_tekrari_dongu_degil", func(t *testing.T) {
		existing := []Edge{{Predecessor: a, Successor: b}}
		if WouldCreateCycle(existing, a, b) {
			t.Fatal("var olan aynı kenarın tekrar eklenmesi döngü sayılmamalı (üst katman UNIQUE ile engeller)")
		}
	})
}

func TestSequentialDependencies(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()

	t.Run("tek_kalem_bagimlilik_yok", func(t *testing.T) {
		if got := sequentialDependencies([]uuid.UUID{a}); got != nil {
			t.Fatalf("tek kalemde bağımlılık üretilmemeli, got %v", got)
		}
	})

	t.Run("bos_liste_bagimlilik_yok", func(t *testing.T) {
		if got := sequentialDependencies(nil); got != nil {
			t.Fatalf("boş listede bağımlılık üretilmemeli, got %v", got)
		}
	})

	t.Run("art_arda_zincir", func(t *testing.T) {
		got := sequentialDependencies([]uuid.UUID{a, b, c})
		want := []Edge{{Predecessor: a, Successor: b}, {Predecessor: b, Successor: c}}
		if len(got) != len(want) {
			t.Fatalf("kenar sayısı = %d, beklenen %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("kenar[%d] = %+v, beklenen %+v", i, got[i], want[i])
			}
		}
		// zincir döngüsel değil — WouldCreateCycle ile çapraz doğrula.
		if !WouldCreateCycle(got, c, a) {
			t.Fatal("üretilen zincire c->a eklemek döngü oluşturmalı (çapraz kontrol)")
		}
	})
}
