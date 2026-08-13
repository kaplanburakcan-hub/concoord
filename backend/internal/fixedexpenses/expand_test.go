package fixedexpenses

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func sp(s string) *string { return &s }

func TestExpandBasicMonthly(t *testing.T) {
	exp := []FixedExpense{
		{ID: uuid.New(), Label: "Araç kirası", Amount: 10000, Active: true,
			ExpenseDayOfMonth: 5, StartDate: "2026-01-01"},
	}
	got := Expand(exp, d("2026-01-01"), d("2026-03-31"))
	if len(got) != 3 {
		t.Fatalf("beklenen 3 satır, alınan %d: %+v", len(got), got)
	}
	want := []string{"2026-01-05", "2026-02-05", "2026-03-05"}
	for i, v := range got {
		if v.EventDate != want[i] {
			t.Errorf("satır %d: beklenen %s, alınan %s", i, want[i], v.EventDate)
		}
		if v.Direction != "out" || v.SourceEntity != "fixed_expense" {
			t.Errorf("satır %d: direction/source_entity yanlış: %+v", i, v)
		}
	}
}

func TestExpandRespectsStartDateMidMonth(t *testing.T) {
	// Gider ayın 20'sinde düşüyor, ama sözleşme 15 Şubat'ta başlıyor —
	// Ocak ayı satırı düşmemeli, Şubat 20'si düşmeli.
	exp := []FixedExpense{
		{ID: uuid.New(), Label: "Mobilizasyon", Amount: 5000, Active: true,
			ExpenseDayOfMonth: 20, StartDate: "2026-02-15"},
	}
	got := Expand(exp, d("2026-01-01"), d("2026-03-31"))
	if len(got) != 2 {
		t.Fatalf("beklenen 2 satır (Şub+Mar), alınan %d: %+v", len(got), got)
	}
	if got[0].EventDate != "2026-02-20" {
		t.Errorf("ilk satır beklenen 2026-02-20, alınan %s", got[0].EventDate)
	}
}

func TestExpandRespectsEndDate(t *testing.T) {
	exp := []FixedExpense{
		{ID: uuid.New(), Label: "Endirekt personel", Amount: 20000, Active: true,
			ExpenseDayOfMonth: 1, StartDate: "2026-01-01", EndDate: sp("2026-02-15")},
	}
	got := Expand(exp, d("2026-01-01"), d("2026-04-30"))
	// Şubat 1 <= 15 Şubat (end_date) → dahil. Mart 1 > 15 Şubat → hariç.
	if len(got) != 2 {
		t.Fatalf("beklenen 2 satır (Oca+Şub), alınan %d: %+v", len(got), got)
	}
}

func TestExpandExcludesInactive(t *testing.T) {
	exp := []FixedExpense{
		{ID: uuid.New(), Label: "İptal edilmiş kira", Amount: 1000, Active: false,
			ExpenseDayOfMonth: 10, StartDate: "2026-01-01"},
	}
	got := Expand(exp, d("2026-01-01"), d("2026-12-31"))
	if len(got) != 0 {
		t.Fatalf("active=false kayıt hiç dönmemeli, alınan %d satır", len(got))
	}
}

func TestExpandClampsToRequestedRange(t *testing.T) {
	// Gider 2020'de başlamış, sınırsız devam ediyor — yalnızca istenen
	// [from,to] aralığındaki aylar dönmeli, öncesi/sonrası değil.
	exp := []FixedExpense{
		{ID: uuid.New(), Label: "Uzun süreli kira", Amount: 3000, Active: true,
			ExpenseDayOfMonth: 1, StartDate: "2020-01-01"},
	}
	got := Expand(exp, d("2026-06-01"), d("2026-06-30"))
	if len(got) != 1 || got[0].EventDate != "2026-06-01" {
		t.Fatalf("beklenen tek satır 2026-06-01, alınan: %+v", got)
	}
}

func TestExpandEmptyRangeReturnsNil(t *testing.T) {
	exp := []FixedExpense{
		{ID: uuid.New(), Label: "X", Amount: 100, Active: true, ExpenseDayOfMonth: 1, StartDate: "2026-01-01"},
	}
	got := Expand(exp, d("2026-06-30"), d("2026-06-01")) // to < from
	if got != nil {
		t.Fatalf("to < from iken nil dönmeli, alınan: %+v", got)
	}
}
