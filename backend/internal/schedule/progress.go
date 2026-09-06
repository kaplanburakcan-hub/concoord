package schedule

import (
	"context"

	"github.com/google/uuid"
)

// ItemAgg — bir WBS kaleminin ilerleme hesabı için gereken ham veri.
// ContractQtySum/CumQtySum yalnızca yaprak kalemler için anlamlıdır (bağlı
// pozların toplamı); üst kalemler ağırlıklı ortalamayla hesaplanır.
type ItemAgg struct {
	ID             uuid.UUID
	ParentID       *uuid.UUID
	ProgressSource string // "manual" | "derived"
	ManualProgress *float64
	Weight         float64
	ContractQtySum float64 // Σ work_items.contract_qty (bağlı pozlar)
	CumQtySum      float64 // Σ (bağlı pozların EN SON Finalized hakedişteki cum_qty'si)
}

// ComputeProgress — internal/payments/calc.go ile AYNI ilke: saf (DB'siz)
// fonksiyon, istek anında çağrılır. items ağacını (ParentID ile) bottom-up
// dolaşıp her kalemin ilerleme yüzdesini (0-100 aralığına sıkıştırılmış)
// hesaplar.
//
// Yaprak kalem:
//   - progress_source='manual' → ManualProgress (yoksa 0)
//   - progress_source='derived' → CumQtySum/ContractQtySum*100 (ContractQtySum
//     0 ise 0)
//
// Üst kalem: Σ(alt.ilerleme × alt.weight) / Σ(alt.weight); tüm ağırlıklar 0
// ise düz aritmetik ortalama.
func ComputeProgress(items []ItemAgg) map[uuid.UUID]float64 {
	byID := make(map[uuid.UUID]*ItemAgg, len(items))
	childrenOf := make(map[uuid.UUID][]uuid.UUID)
	var roots []uuid.UUID
	for i := range items {
		it := &items[i]
		byID[it.ID] = it
		if it.ParentID != nil {
			childrenOf[*it.ParentID] = append(childrenOf[*it.ParentID], it.ID)
		} else {
			roots = append(roots, it.ID)
		}
	}

	result := make(map[uuid.UUID]float64, len(items))
	visited := make(map[uuid.UUID]bool, len(items))

	var visit func(id uuid.UUID) float64
	visit = func(id uuid.UUID) float64 {
		if visited[id] {
			return result[id]
		}
		visited[id] = true // döngüsel (bozuk) veriye karşı koruma
		it, ok := byID[id]
		if !ok {
			return 0
		}
		kids := childrenOf[id]
		var pct float64
		if len(kids) == 0 {
			switch it.ProgressSource {
			case "manual":
				if it.ManualProgress != nil {
					pct = *it.ManualProgress
				}
			default: // "derived"
				if it.ContractQtySum > 0 {
					pct = (it.CumQtySum / it.ContractQtySum) * 100
				}
			}
		} else {
			var weightedSum, weightSum, plainSum float64
			for _, cid := range kids {
				cp := visit(cid)
				cw := byID[cid].Weight
				weightedSum += cp * cw
				weightSum += cw
				plainSum += cp
			}
			if weightSum > 0 {
				pct = weightedSum / weightSum
			} else {
				pct = plainSum / float64(len(kids))
			}
		}
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
		result[id] = pct
		return pct
	}

	for _, id := range roots {
		visit(id)
	}
	// Kök olmayan ama (parent_id silinmiş bir kayda işaret ediyorsa) hiç
	// ziyaret edilmemiş kalemler için de hesapla.
	for i := range items {
		if !visited[items[i].ID] {
			visit(items[i].ID)
		}
	}
	return result
}

// LoadItemAggregates — ComputeProgress'in ihtiyaç duyduğu ham veriyi tek
// sorguda toplar. "cum_qty" bir pozun EN SON Finalized hakedişindeki
// kümülatif miktarıdır — birden çok döneme yayılmış Finalized hakedişlerin
// cum_qty'lerini TOPLAMAK yanlış olurdu (cum_qty zaten o döneme dek olan
// kümülatiftir, dönemleri toplamak miktarı katlar).
func (h *Handler) LoadItemAggregates(ctx context.Context, projectID uuid.UUID) ([]ItemAgg, error) {
	rows, err := h.pool.Query(ctx, `
		WITH latest_cum AS (
			SELECT DISTINCT ON (ppi.work_item_id)
				ppi.work_item_id, ppi.cum_qty
			FROM progress_payment_items ppi
			JOIN progress_payments pp ON pp.id = ppi.progress_payment_id
			WHERE pp.status = 'Finalized'
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
		WHERE si.project_id = $1 AND si.deleted_at IS NULL`, projectID)
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
