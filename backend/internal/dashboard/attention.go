package dashboard

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/httpx"
)

// Panel "dikkat" sayaçları (E11). Her sayaç kendi izniyle korunan ayrı bir uçtur;
// bu sayede kullanıcı yalnız yetkisi olan kartı görür ve veri sızmaz.

// RFIOverdueCount — cevap bekleyen, hâlâ 'Açık' ve yanıt tarihi geçmiş yazışmalar.
// Koşul, uyarı üretimindeki (RunControlAlerts) koşulla aynıdır.
func (h *Handler) RFIOverdueCount(w http.ResponseWriter, r *http.Request) {
	h.countOf(w, r, `
		SELECT count(*) FROM correspondences
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND cevap_gerekli AND durum='Açık'
		  AND cevap_tarihi IS NOT NULL AND cevap_tarihi < CURRENT_DATE`)
}

// PlanLateCount — teslim tarihi geçmiş ve teslim alınmamış tedarik planı kalemleri
// (tedarik planı sayfasındaki 'Gecikmiş' koşuluyla aynı).
func (h *Handler) PlanLateCount(w http.ResponseWriter, r *http.Request) {
	h.countOf(w, r, `
		SELECT count(*) FROM procurement_plan_items
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND planned_delivery_date IS NOT NULL AND planned_delivery_date < CURRENT_DATE
		  AND status <> 'TeslimAlindi'`)
}

func (h *Handler) countOf(w http.ResponseWriter, r *http.Request, query string) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return
	}
	var n int
	if err := h.pool.QueryRow(r.Context(), query, pid).Scan(&n); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"count": n})
}

// TasksOverdueCount — vadesi geçmiş, tamamlanmamış ve silinmemiş görevler.
// Görev listesindeki 'Done' dışı durumlarla aynı koşul.
func (h *Handler) TasksOverdueCount(w http.ResponseWriter, r *http.Request) {
	h.countOf(w, r, `
		SELECT count(*) FROM tasks
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND status <> 'Done' AND due_date IS NOT NULL AND due_date < CURRENT_DATE`)
}

// ThisWeekCounts — önümüzdeki 7 gün içinde (bugün dahil) vadesi gelen kalemler.
// Panelde 'Bu hafta' kartı için tek uç; her sayı kendi kayıt türünün koşuluyla aynıdır.
func (h *Handler) ThisWeekCounts(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return
	}
	var out struct {
		Tasks           int `json:"tasks"`
		Correspondences int `json:"correspondences"`
		Orders          int `json:"orders"`
		Payments        int `json:"payments"`
	}
	if err := h.pool.QueryRow(r.Context(), `
		SELECT
		  (SELECT count(*) FROM tasks WHERE project_id=$1 AND deleted_at IS NULL
		     AND status<>'Done' AND due_date BETWEEN CURRENT_DATE AND CURRENT_DATE+7),
		  (SELECT count(*) FROM correspondences WHERE project_id=$1 AND deleted_at IS NULL
		     AND cevap_gerekli AND durum='Açık' AND cevap_tarihi BETWEEN CURRENT_DATE AND CURRENT_DATE+7),
		  (SELECT count(*) FROM purchase_orders WHERE project_id=$1 AND deleted_at IS NULL
		     AND status IN ('Ordered','PartiallyDelivered') AND expected_date BETWEEN CURRENT_DATE AND CURRENT_DATE+7),
		  (SELECT count(*) FROM progress_payments WHERE project_id=$1 AND deleted_at IS NULL
		     AND status IN ('Draft','Submitted','InApproval','SiteApproved')
		     AND period_end BETWEEN CURRENT_DATE AND CURRENT_DATE+7)`, pid).
		Scan(&out.Tasks, &out.Correspondences, &out.Orders, &out.Payments); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
