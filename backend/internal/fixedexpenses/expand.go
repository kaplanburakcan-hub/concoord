package fixedexpenses

import (
	"time"

	"github.com/google/uuid"
)

// VirtualEvent — cash_events ile aynı görünümde ama DB'de yaşamayan, rapor
// anında hesaplanan bir nakit hareketi (Faz F'nin nakit akış raporu,
// gerçek cash_events satırlarıyla bunu birleştirir).
type VirtualEvent struct {
	SourceEntity string    `json:"source_entity"` // her zaman "fixed_expense"
	SourceID     uuid.UUID `json:"source_id"`
	Description  string    `json:"description"`
	Amount       float64   `json:"amount"`
	Direction    string    `json:"direction"` // her zaman "out"
	EventDate    string    `json:"event_date"`
}

// Expand — verilen sabit gider kayıtlarını [from,to] tarih aralığı için ay
// ay sanal olarak genişletir. Yalnızca active=true kayıtlar, yalnızca
// start_date..end_date (end_date NULL ise sınırsız) aralığına düşen aylar
// dahil edilir. DB'ye dokunmaz — saf fonksiyon.
func Expand(expenses []FixedExpense, from, to time.Time) []VirtualEvent {
	from = dateOnly(from)
	to = dateOnly(to)
	if to.Before(from) {
		return nil
	}
	var out []VirtualEvent
	for _, e := range expenses {
		if !e.Active {
			continue
		}
		start, err := time.Parse("2006-01-02", e.StartDate)
		if err != nil {
			continue
		}
		var end *time.Time
		if e.EndDate != nil {
			if t, err := time.Parse("2006-01-02", *e.EndDate); err == nil {
				end = &t
			}
		}
		cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
		lastMonth := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
		for !cur.After(lastMonth) {
			eventDate := time.Date(cur.Year(), cur.Month(), e.ExpenseDayOfMonth, 0, 0, 0, 0, time.UTC)
			cur = cur.AddDate(0, 1, 0)

			if eventDate.Before(from) || eventDate.After(to) {
				continue
			}
			if eventDate.Before(dateOnly(start)) {
				continue
			}
			if end != nil && eventDate.After(dateOnly(*end)) {
				continue
			}
			out = append(out, VirtualEvent{
				SourceEntity: "fixed_expense",
				SourceID:     e.ID,
				Description:  e.Label,
				Amount:       e.Amount,
				Direction:    "out",
				EventDate:    eventDate.Format("2006-01-02"),
			})
		}
	}
	return out
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
