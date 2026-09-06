package schedule

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SCurvePoint — bir zaman kovasındaki dört seri (Bkz. şartname 1.3).
type SCurvePoint struct {
	Date            string  `json:"date"`             // kova sonu, YYYY-MM-DD
	PlannedPhysical float64 `json:"planned_physical"` // 0-100
	ActualPhysical  float64 `json:"actual_physical"`  // 0-100
	PlannedCash     float64 `json:"planned_cash"`     // mutlak tutar (weight birimiyle)
	ActualCash      float64 `json:"actual_cash"`      // mutlak tutar (net_payable toplamı)
}

type itemDates struct {
	ID             uuid.UUID
	ParentID       *uuid.UUID
	Weight         float64
	BaselineStart  *time.Time
	BaselineFinish *time.Time
}

// plannedFraction — bir kalemin [start,finish] aralığında t anındaki
// doğrusal tamamlanma kesri (0-1). Tarih yoksa veya sıfır süreliyse (kilometre
// taşı) start<=t ise 1, değilse 0.
func plannedFraction(start, finish *time.Time, at time.Time) float64 {
	if start == nil || finish == nil {
		return 0
	}
	if !finish.After(*start) {
		if !at.Before(*finish) {
			return 1
		}
		return 0
	}
	if at.Before(*start) {
		return 0
	}
	if !at.Before(*finish) {
		return 1
	}
	return at.Sub(*start).Hours() / finish.Sub(*start).Hours()
}

// virtualRootProgress — ComputeProgress'i DEĞİŞTİRMEDEN, projenin tek bir
// genel yüzdesini almak için tüm kök kalemleri sanal bir tek köke (uuid.Nil)
// bağlar; ağırlıklı ortalama böylece proje genelinde de doğru çalışır.
func virtualRootProgress(items []ItemAgg) float64 {
	augmented := make([]ItemAgg, len(items), len(items)+1)
	copy(augmented, items)
	for i := range augmented {
		if augmented[i].ParentID == nil {
			root := uuid.Nil
			augmented[i].ParentID = &root
		}
	}
	augmented = append(augmented, ItemAgg{ID: uuid.Nil})
	return ComputeProgress(augmented)[uuid.Nil]
}

// plannedPhysicalAt — virtualRootProgress ile AYNI ilkeyi, gerçek yerine
// planlanan (tarih bazlı) ilerlemeyle kullanır: her kalemin ManualProgress'i
// o anki plannedFraction*100 ile değiştirilir, ağaç aynı şekilde katlanır.
func plannedPhysicalAt(dates []itemDates, at time.Time) float64 {
	agg := make([]ItemAgg, len(dates))
	for i, d := range dates {
		pct := plannedFraction(d.BaselineStart, d.BaselineFinish, at) * 100
		agg[i] = ItemAgg{ID: d.ID, ParentID: d.ParentID, ProgressSource: "manual", ManualProgress: &pct, Weight: d.Weight}
	}
	return virtualRootProgress(agg)
}

// plannedCashAt — yalnızca YAPRAK kalemlerin weight'i (spec: "bağlı pozların
// sözleşme tutarı toplamı") üzerinden mutlak planlanan nakit. Üst kalemlerin
// weight'i çocuklarının toplamıyla garantili eşit olmadığından (kullanıcı
// elle girebilir), üst kalemleri toplama katmak çift saymaya yol açar.
func plannedCashAt(dates []itemDates, at time.Time) float64 {
	hasChild := make(map[uuid.UUID]bool, len(dates))
	for _, d := range dates {
		if d.ParentID != nil {
			hasChild[*d.ParentID] = true
		}
	}
	var sum float64
	for _, d := range dates {
		if hasChild[d.ID] {
			continue // yaprak değil
		}
		sum += d.Weight * plannedFraction(d.BaselineStart, d.BaselineFinish, at)
	}
	return sum
}

func (h *Handler) loadItemDates(ctx context.Context, projectID uuid.UUID) ([]itemDates, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id, parent_id, weight::float8, baseline_start, baseline_finish
		FROM schedule_items WHERE project_id=$1 AND deleted_at IS NULL`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []itemDates{}
	for rows.Next() {
		var d itemDates
		if err := rows.Scan(&d.ID, &d.ParentID, &d.Weight, &d.BaselineStart, &d.BaselineFinish); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// loadItemAggregatesAsOf — LoadItemAggregates ile AYNI sorgu, tek farkla:
// yalnızca belirtilen tarihte veya öncesinde biten (period_end<=asOf)
// Finalized hakedişler sayılır — S-eğrisinin geçmiş kovalarında o anki
// gerçek ilerlemeyi yeniden kurmak için.
func (h *Handler) loadItemAggregatesAsOf(ctx context.Context, projectID uuid.UUID, asOf time.Time) ([]ItemAgg, error) {
	rows, err := h.pool.Query(ctx, `
		WITH latest_cum AS (
			SELECT DISTINCT ON (ppi.work_item_id)
				ppi.work_item_id, ppi.cum_qty
			FROM progress_payment_items ppi
			JOIN progress_payments pp ON pp.id = ppi.progress_payment_id
			WHERE pp.status = 'Finalized' AND pp.period_end <= $2
			ORDER BY ppi.work_item_id, pp.period_no DESC
		)
		SELECT si.id, si.parent_id, si.progress_source, si.manual_progress, si.weight::float8,
		       COALESCE(agg.contract_qty_sum, 0)::float8, COALESCE(agg.cum_qty_sum, 0)::float8
		FROM schedule_items si
		LEFT JOIN LATERAL (
			SELECT SUM(wi.contract_qty) AS contract_qty_sum,
			       SUM(COALESCE(lc.cum_qty, 0)) AS cum_qty_sum
			FROM schedule_item_pozlar sip
			JOIN work_items wi ON wi.id = sip.poz_id
			LEFT JOIN latest_cum lc ON lc.work_item_id = wi.id
			WHERE sip.schedule_item_id = si.id
		) agg ON true
		WHERE si.project_id = $1 AND si.deleted_at IS NULL`, projectID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ItemAgg{}
	for rows.Next() {
		var it ItemAgg
		var manualProgress *float64
		if err := rows.Scan(&it.ID, &it.ParentID, &it.ProgressSource, &manualProgress,
			&it.Weight, &it.ContractQtySum, &it.CumQtySum); err != nil {
			return nil, err
		}
		it.ManualProgress = manualProgress
		out = append(out, it)
	}
	return out, rows.Err()
}

// actualCashAt — proje genelinde, period_end<=at olan TÜM Finalized
// hakedişlerin net_payable toplamı (taşeron bağımsız — gerçekten ödenen
// tutar).
func (h *Handler) actualCashAt(ctx context.Context, projectID uuid.UUID, at time.Time) (float64, error) {
	var sum float64
	if err := h.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(net_payable), 0)::float8
		FROM progress_payments
		WHERE project_id=$1 AND status='Finalized' AND deleted_at IS NULL AND period_end <= $2`,
		projectID, at,
	).Scan(&sum); err != nil {
		return 0, err
	}
	return sum, nil
}

// bucketDates — from'dan to'ya (dahil), "week" (haftalık, Pazartesi bitişli)
// veya "month" (ay sonu) kova sonu tarihlerini üretir.
func bucketDates(from, to time.Time, bucket string) []time.Time {
	if !to.After(from) {
		return []time.Time{to}
	}
	var out []time.Time
	if bucket == "week" {
		cur := from
		for !cur.After(to) {
			out = append(out, cur)
			cur = cur.AddDate(0, 0, 7)
		}
	} else {
		cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1)
		for !cur.After(to) {
			out = append(out, cur)
			cur = time.Date(cur.Year(), cur.Month()+2, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
		}
	}
	if len(out) == 0 || out[len(out)-1].Before(to) {
		out = append(out, to)
	}
	return out
}

// ComputeSCurve — dört seriyi (planlanan/gerçekleşen × fiziksel/nakit) kova
// kova döndürür. Materialized view/cache YOK — istek anında hesaplanır
// (Plan gereği, progress.go ile aynı ilke).
func (h *Handler) ComputeSCurve(ctx context.Context, projectID uuid.UUID, from, to time.Time, bucket string) ([]SCurvePoint, error) {
	dates, err := h.loadItemDates(ctx, projectID)
	if err != nil {
		return nil, err
	}

	buckets := bucketDates(from, to, bucket)
	out := make([]SCurvePoint, 0, len(buckets))
	for _, b := range buckets {
		agg, err := h.loadItemAggregatesAsOf(ctx, projectID, b)
		if err != nil {
			return nil, err
		}
		cash, err := h.actualCashAt(ctx, projectID, b)
		if err != nil {
			return nil, err
		}
		out = append(out, SCurvePoint{
			Date:            b.Format("2006-01-02"),
			PlannedPhysical: plannedPhysicalAt(dates, b),
			ActualPhysical:  virtualRootProgress(agg),
			PlannedCash:     plannedCashAt(dates, b),
			ActualCash:      cash,
		})
	}
	return out, nil
}
