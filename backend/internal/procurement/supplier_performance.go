package procurement

import (
	"net/http"

	"github.com/ipks/ipks/backend/internal/httpx"
)

// SupplierPerformanceDTO — tedarikçi bazında teslim performansı (E11). Salt okunur;
// yalnız beklenen teslim tarihi girilmiş, iptal edilmemiş siparişler sayılır.
type SupplierPerformanceDTO struct {
	Supplier    string   `json:"supplier"`
	Total       int      `json:"total"`
	OnTime      int      `json:"on_time"`
	Late        int      `json:"late"`
	OpenPending int      `json:"open_pending"`
	OnTimeRate  *float64 `json:"on_time_rate,omitempty"` // on_time / (on_time + late); kapanmış sipariş yoksa boş
}

// SupplierPerformance — teslim tarihi kıyası: kapanmış siparişte son teslimat tarihi
// beklenen tarihe eşit veya öncesindeyse zamanında. Açık siparişte beklenen tarih
// geçmişse geç sayılır (ve oran paydasına girmez: henüz kapanmadı).
func (h *Handler) SupplierPerformance(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		WITH po AS (
		  SELECT COALESCE(NULLIF(btrim(o.supplier_name),''), '—') AS supplier,
		         o.status, o.expected_date,
		         (SELECT max(d.delivered_at)::date FROM deliveries d
		           WHERE d.po_id = o.id AND d.deleted_at IS NULL) AS last_delivery
		  FROM purchase_orders o
		  WHERE o.project_id=$1 AND o.deleted_at IS NULL
		    AND o.status <> 'Cancelled' AND o.expected_date IS NOT NULL
		)
		SELECT supplier,
		       count(*)::int,
		       count(*) FILTER (WHERE status='Delivered' AND last_delivery <= expected_date)::int,
		       count(*) FILTER (WHERE (status='Delivered' AND last_delivery > expected_date)
		                        OR (status IN ('Ordered','PartiallyDelivered') AND expected_date < CURRENT_DATE))::int,
		       count(*) FILTER (WHERE status IN ('Ordered','PartiallyDelivered') AND expected_date >= CURRENT_DATE)::int
		FROM po
		GROUP BY supplier
		ORDER BY count(*) FILTER (WHERE (status='Delivered' AND last_delivery > expected_date)
		                          OR (status IN ('Ordered','PartiallyDelivered') AND expected_date < CURRENT_DATE)) DESC,
		         supplier`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []SupplierPerformanceDTO{}
	for rows.Next() {
		var d SupplierPerformanceDTO
		if err := rows.Scan(&d.Supplier, &d.Total, &d.OnTime, &d.Late, &d.OpenPending); err != nil {
			httpx.Internal(w, r)
			return
		}
		if closed := d.OnTime + d.Late; closed > 0 {
			rate := float64(d.OnTime) / float64(closed)
			d.OnTimeRate = &rate
		}
		out = append(out, d)
	}
	if rows.Err() != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"suppliers": out})
}
