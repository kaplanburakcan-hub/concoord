// Package cashflow — Nakit Akış Faz F: gerçek cash_events (Faz A-D) ile
// fixedexpenses.Expand()'in sanal satırlarını (Faz E) birleştirip
// gün/hafta/ay gruplu, kümülatif bakiyeli bir rapor üretir. Ayrıca dört
// farklı kaynaktaki (hakediş/ekstre/PO/idari hakediş) ödeme planlarını tek
// bir listede toplar — "Ödemeler" sekmesinin iki temel görünümü budur.
package cashflow

import "time"

// Event — cash_events ile fixedexpenses.VirtualEvent'i ortak bir görünümde
// birleştirmek için kullanılan minimal iç tip.
type Event struct {
	Direction string // "in" | "out"
	Amount    float64
	EventDate string // YYYY-MM-DD
}

type Period struct {
	Label             string  `json:"label"`
	Start             string  `json:"start"`
	End               string  `json:"end"`
	In                float64 `json:"in"`
	Out               float64 `json:"out"`
	Net               float64 `json:"net"`
	CumulativeBalance float64 `json:"cumulative_balance"`
}

// BuildPeriods — [from,to] aralığını `group` granülaritesinde ("daily" |
// "weekly" | "monthly") ardışık dönemlere böler ve her birine düşen
// events'i toplayıp kümülatif bakiyeyi hesaplar. Kümülatif bakiye [from,to]
// aralığının BAŞINDAN (0) başlar — proje ömrü boyunca tüm geçmişi değil,
// yalnızca istenen aralığı kapsar (sınırsız geçmiş taraması pahalı ve
// genellikle istenen de bu değildir: "bu dönemde nakit nasıl gidiyor").
func BuildPeriods(events []Event, from, to time.Time, group string) []Period {
	from = dateOnly(from)
	to = dateOnly(to)
	if to.Before(from) {
		return nil
	}

	type bucket struct{ start, end time.Time }
	var order []string
	buckets := map[string]bucket{}

	step := func(t time.Time) time.Time {
		switch group {
		case "weekly":
			return t.AddDate(0, 0, 7)
		case "monthly":
			return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		default: // daily
			return t.AddDate(0, 0, 1)
		}
	}

	cur := bucketStart(from, group)
	for !cur.After(to) {
		label, start, end := bucketKey(cur, group)
		if _, ok := buckets[label]; !ok {
			order = append(order, label)
			buckets[label] = bucket{start: start, end: end}
		}
		cur = step(cur)
	}

	sums := map[string][2]float64{} // label -> [in, out]
	for _, e := range events {
		t, err := time.Parse("2006-01-02", e.EventDate)
		if err != nil {
			continue
		}
		t = dateOnly(t)
		if t.Before(from) || t.After(to) {
			continue
		}
		label, _, _ := bucketKey(t, group)
		s := sums[label]
		if e.Direction == "in" {
			s[0] += e.Amount
		} else {
			s[1] += e.Amount
		}
		sums[label] = s
	}

	out := make([]Period, 0, len(order))
	var cumulative float64
	for _, label := range order {
		b := buckets[label]
		s := sums[label]
		net := s[0] - s[1]
		cumulative += net
		out = append(out, Period{
			Label: label, Start: b.start.Format("2006-01-02"), End: b.end.Format("2006-01-02"),
			In: s[0], Out: s[1], Net: net, CumulativeBalance: cumulative,
		})
	}
	return out
}

func bucketStart(t time.Time, group string) time.Time {
	switch group {
	case "weekly":
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return t.AddDate(0, 0, -(wd - 1))
	case "monthly":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return t
	}
}

func bucketKey(t time.Time, group string) (label string, start, end time.Time) {
	switch group {
	case "weekly":
		start = bucketStart(t, "weekly")
		end = start.AddDate(0, 0, 6)
		return start.Format("2006-01-02"), start, end
	case "monthly":
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, -1)
		return start.Format("2006-01"), start, end
	default:
		return t.Format("2006-01-02"), t, t
	}
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
