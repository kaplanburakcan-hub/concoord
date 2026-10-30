package procurement

import (
	"net/http"

	"github.com/ipks/ipks/backend/internal/httpx"
)

// ListUnmatchedPlanItems — salt okunur (E8 / B8). Bir plan kalemi, aynı
// projedeki iptal edilmemiş bir PO'nun PR'ında aynı malzeme adıyla
// (büyük/küçük harf ve baştaki-sondaki boşluk yok sayılarak) bulunuyorsa
// eşleşmiş sayılır; bulunmayanlar döner.
//
// Not: PO ve PR kalemlerinde poz alanı yoktur; eşleşme bu nedenle malzeme
// adına dayanır. Poz eşleşmesi, PO satırlarına poz eklenince eklenebilir.
func (h *Handler) ListUnmatchedPlanItems(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), planItemSelect+`
		WHERE p.project_id=$1 AND p.deleted_at IS NULL
		  AND NOT EXISTS (
		      SELECT 1
		      FROM purchase_request_items pri
		      JOIN purchase_requests pr ON pr.id = pri.pr_id AND pr.deleted_at IS NULL
		      JOIN purchase_orders po  ON po.pr_id = pr.id
		      WHERE pr.project_id = p.project_id
		        AND pri.deleted_at IS NULL
		        AND po.deleted_at IS NULL AND po.status <> 'Cancelled'
		        AND lower(btrim(pri.material_name)) = lower(btrim(p.description))
		  )
		ORDER BY p.planned_delivery_date NULLS LAST, p.created_at`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []planItemDTO{}
	for rows.Next() {
		var d planItemDTO
		if err := scanPlanItem(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	if rows.Err() != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"items": out})
}
