// Package manhour — Verimlilik Normları: 5 geçmiş projeden (Adana, Yozgat,
// Elazığ, İkitelli, Bursa) derlenmiş, imalat kalemi başına gerçekleşen
// birim adam-saat referans veritabanı. Firma çapında, projeden bağımsız
// salt-okunur veri (kullanıcının paylaştığı "Manhour Database.xlsx",
// migration 000066 ile tek seferlik yüklendi).
package manhour

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ db *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{db: pool} }

type Norm struct {
	ID           string  `json:"id"`
	Kategori     string  `json:"kategori"`
	Imalat       string  `json:"imalat_aciklamasi"`
	Birim        *string `json:"birim"`
	Araligi      *string `json:"araligi"`
	OrtalamaAdSa float64 `json:"ortalama_adam_saat"`
}

// ListNorms — genel verimlilik normları (kategori sırasına göre).
func (h *Handler) ListNorms(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id, kategori, imalat_aciklamasi, birim, araligi, ortalama_adam_saat
		FROM manhour_norms ORDER BY sort_order`)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Norm{}
	for rows.Next() {
		var n Norm
		if err := rows.Scan(&n.ID, &n.Kategori, &n.Imalat, &n.Birim, &n.Araligi, &n.OrtalamaAdSa); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, n)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"norms": out})
}

type ProjectHistoryRow struct {
	ID       string   `json:"id"`
	Kategori *string  `json:"kategori"`
	PozNo    string   `json:"poz_no"`
	Imalat   string   `json:"imalat"`
	Adana    *float64 `json:"adana"`
	Yozgat   *float64 `json:"yozgat"`
	Elazig   *float64 `json:"elazig"`
	Ikitelli *float64 `json:"ikitelli"`
	Bursa    *float64 `json:"bursa"`
	Ortalama *float64 `json:"ortalama"`
}

// ListProjectHistory — 5 geçmiş projenin poz bazlı gerçekleşen birim
// adam-saat kıyaslaması (İcmal sayfası).
func (h *Handler) ListProjectHistory(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `
		SELECT id, kategori, poz_no, imalat, adana, yozgat, elazig, ikitelli, bursa, ortalama
		FROM manhour_project_history ORDER BY sort_order`)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []ProjectHistoryRow{}
	for rows.Next() {
		var p ProjectHistoryRow
		if err := rows.Scan(&p.ID, &p.Kategori, &p.PozNo, &p.Imalat,
			&p.Adana, &p.Yozgat, &p.Elazig, &p.Ikitelli, &p.Bursa, &p.Ortalama); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"history": out})
}
