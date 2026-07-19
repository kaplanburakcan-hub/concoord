package payments

// Faz 11 — Kesinti kataloğu ucu.
//
// Hakediş girişinde kalem atlanmasın diye kesintiler gruplanmış bir listeden
// seçilir. Katalog veritabanındadır (deduction_catalog): yeni kesinti türü
// eklemek kod değişikliği değil, veri girişidir.

import (
	"net/http"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type catalogItem struct {
	GroupCode      string   `json:"group_code"`
	Code           string   `json:"code"`
	Label          string   `json:"label"`
	DeductionType  string   `json:"deduction_type"`
	Nature         string   `json:"nature"`
	ReducesCost    bool     `json:"reduces_cost"`
	DefaultRatePct *float64 `json:"default_rate_pct,omitempty"`
	RefundStage    *string  `json:"refund_stage,omitempty"`
	Note           *string  `json:"note,omitempty"`
}

// GroupLabelsTR — grup kodlarının Türkçe başlıkları (ekran sıralamasıyla).
var GroupLabelsTR = []struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
}{
	{"Tax", "Vergi ve yasal kesintiler", "Kâti — işin maliyetini değiştirmez"},
	{"Advance", "Avans mahsupları", "Mahsup — daha önce ödenen avansın geri alınması"},
	{"Retention", "Teminat kesintileri", "Geçici — kabulde taşerona iade edilir"},
	{"Penalty", "Cezalar", "Kâti — işin maliyetini azaltır"},
	{"GoodsService", "Mal ve hizmet kesintileri", "Kâti — taşerona verilen mal/hizmet bedeli"},
	{"Adjustment", "Düzeltme ve mahsuplaşmalar", "Duruma göre değerlendirilir"},
}

// DeductionCatalog — GET /api/v1/deduction-catalog
// Gruplar ve aktif kalemler; arayüz bunu açılır listede kullanır.
func (h *Handler) DeductionCatalog(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT group_code, code, label, deduction_type, nature, reduces_cost,
		       default_rate_pct::float8, refund_stage, note
		FROM deduction_catalog
		WHERE is_active
		ORDER BY group_code, sort_order, label`)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	items := make([]catalogItem, 0, 48)
	for rows.Next() {
		var it catalogItem
		if err := rows.Scan(&it.GroupCode, &it.Code, &it.Label, &it.DeductionType,
			&it.Nature, &it.ReducesCost, &it.DefaultRatePct, &it.RefundStage, &it.Note); err != nil {
			httpx.Internal(w, r)
			return
		}
		items = append(items, it)
	}
	if rows.Err() != nil {
		httpx.Internal(w, r)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"groups": GroupLabelsTR,
		"items":  items,
	})
}
