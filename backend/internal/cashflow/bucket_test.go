package cashflow

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBuildPeriodsDailyCumulative(t *testing.T) {
	events := []Event{
		{Direction: "in", Amount: 1000, EventDate: "2026-01-01"},
		{Direction: "out", Amount: 300, EventDate: "2026-01-01"},
		{Direction: "out", Amount: 200, EventDate: "2026-01-03"},
	}
	got := BuildPeriods(events, d("2026-01-01"), d("2026-01-03"), "daily")
	if len(got) != 3 {
		t.Fatalf("beklenen 3 gün, alınan %d: %+v", len(got), got)
	}
	if got[0].In != 1000 || got[0].Out != 300 || got[0].Net != 700 || got[0].CumulativeBalance != 700 {
		t.Errorf("gün 1 yanlış: %+v", got[0])
	}
	if got[1].In != 0 || got[1].Out != 0 || got[1].CumulativeBalance != 700 {
		t.Errorf("gün 2 (boş) kümülatif korunmalı: %+v", got[1])
	}
	if got[2].Out != 200 || got[2].CumulativeBalance != 500 {
		t.Errorf("gün 3 yanlış: %+v", got[2])
	}
}

func TestBuildPeriodsMonthly(t *testing.T) {
	events := []Event{
		{Direction: "in", Amount: 5000, EventDate: "2026-01-15"},
		{Direction: "out", Amount: 1000, EventDate: "2026-02-05"},
	}
	got := BuildPeriods(events, d("2026-01-01"), d("2026-03-31"), "monthly")
	if len(got) != 3 {
		t.Fatalf("beklenen 3 ay, alınan %d: %+v", len(got), got)
	}
	if got[0].Label != "2026-01" || got[0].In != 5000 {
		t.Errorf("Ocak yanlış: %+v", got[0])
	}
	if got[1].Label != "2026-02" || got[1].Out != 1000 || got[1].CumulativeBalance != 4000 {
		t.Errorf("Şubat yanlış: %+v", got[1])
	}
	if got[2].Label != "2026-03" || got[2].CumulativeBalance != 4000 {
		t.Errorf("Mart (boş, kümülatif korunmalı) yanlış: %+v", got[2])
	}
}

func TestBuildPeriodsWeekly(t *testing.T) {
	// 2026-01-01 Perşembe. Hafta Pazartesi başlar.
	events := []Event{
		{Direction: "in", Amount: 100, EventDate: "2026-01-01"},
		{Direction: "in", Amount: 200, EventDate: "2026-01-08"}, // sonraki hafta
	}
	got := BuildPeriods(events, d("2026-01-01"), d("2026-01-08"), "weekly")
	if len(got) != 2 {
		t.Fatalf("beklenen 2 hafta, alınan %d: %+v", len(got), got)
	}
	if got[0].In != 100 {
		t.Errorf("1. hafta yanlış: %+v", got[0])
	}
	if got[1].In != 200 || got[1].CumulativeBalance != 300 {
		t.Errorf("2. hafta yanlış: %+v", got[1])
	}
}

func TestBuildPeriodsIgnoresOutOfRangeEvents(t *testing.T) {
	events := []Event{
		{Direction: "in", Amount: 999, EventDate: "2025-12-31"}, // aralık dışı
		{Direction: "in", Amount: 100, EventDate: "2026-01-01"},
	}
	got := BuildPeriods(events, d("2026-01-01"), d("2026-01-01"), "daily")
	if len(got) != 1 || got[0].In != 100 {
		t.Fatalf("aralık dışı olay dahil edilmemeli: %+v", got)
	}
}

func TestBuildPeriodsEmptyRangeReturnsNil(t *testing.T) {
	got := BuildPeriods(nil, d("2026-01-10"), d("2026-01-01"), "daily")
	if got != nil {
		t.Fatalf("to < from iken nil dönmeli, alınan: %+v", got)
	}
}
