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
