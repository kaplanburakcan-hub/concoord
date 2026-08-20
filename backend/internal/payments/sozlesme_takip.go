package payments

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/httpx"
)

// Sözleşme Takip — Proje Keşfi (project_survey_items) kalemlerini poz_no
// üzerinden work_items'e, oradan subcontractors + contracts'a bağlayıp
// her keşif kalemi karşısında eşleşen taşeron adı + sözleşme bilgisini
// gösterir. Salt okunur — hiçbir yeni veri yazmaz, sadece mevcut üç
// tablonun (Proje Keşfi × İş Kalemleri × Sözleşmeler) poz_no eşleşmesi.

type sozlesmeTakipEslesme struct {
	TaseronAdi     string  `json:"taseron_adi"`
	SozlesmeNo     *string `json:"sozlesme_no,omitempty"`
	SozlesmeTuru   *string `json:"sozlesme_turu,omitempty"`
	SozlesmeTarihi *string `json:"sozlesme_tarihi,omitempty"`
}

type sozlesmeTakipItem struct {
	ID         uuid.UUID              `json:"id"`
	Kategori   string                 `json:"kategori"`
	PozNo      string                 `json:"poz_no"`
	Tanim      string                 `json:"tanim"`
	Birim      string                 `json:"birim"`
	Miktar     float64                `json:"miktar"`
	Eslesmeler []sozlesmeTakipEslesme `json:"eslesmeler"`
}

func (h *Handler) SozlesmeTakip(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT psi.id, psi.kategori, COALESCE(psi.poz_no,''), psi.tanim, psi.birim, psi.miktar,
		       s.company_name, c.contract_no, c.type, to_char(c.sign_date,'YYYY-MM-DD')
		FROM project_survey_items psi
		LEFT JOIN work_items wi ON wi.project_id = psi.project_id
		    AND wi.poz_no = psi.poz_no AND wi.deleted_at IS NULL
		    AND psi.poz_no IS NOT NULL AND psi.poz_no <> ''
		LEFT JOIN subcontractors s ON s.id = wi.subcontractor_id AND s.deleted_at IS NULL
		LEFT JOIN contracts c ON c.id = wi.contract_id AND c.deleted_at IS NULL
		WHERE psi.project_id = $1
		ORDER BY psi.kategori, psi.sira, psi.tanim`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	// Sıra korunarak grupla: id'ler ilk görülme sırasıyla, satırlar ise psi
	// tarafında zaten kategori/sira/tanim sıralı geliyor. Map'te pointer
	// tutmak yerine index tutuyoruz — slice büyüyüp yeniden ayrılsa bile
	// (append) index geçerliliğini korur, pointer korumaz.
	order := []uuid.UUID{}
	byID := map[uuid.UUID]int{}
	items := map[uuid.UUID]sozlesmeTakipItem{}
	for rows.Next() {
		var (
			id                                                   uuid.UUID
			kategori, pozNo, tanim, birim                        string
			miktar                                               float64
			taseronAdi, sozlesmeNo, sozlesmeTuru, sozlesmeTarihi *string
		)
		if err := rows.Scan(&id, &kategori, &pozNo, &tanim, &birim, &miktar,
			&taseronAdi, &sozlesmeNo, &sozlesmeTuru, &sozlesmeTarihi); err != nil {
			httpx.Internal(w, r)
			return
		}
		idx, seen := byID[id]
		if !seen {
			idx = len(order)
			order = append(order, id)
			byID[id] = idx
			items[id] = sozlesmeTakipItem{
				ID: id, Kategori: kategori, PozNo: pozNo, Tanim: tanim, Birim: birim, Miktar: miktar,
				Eslesmeler: []sozlesmeTakipEslesme{},
			}
		}
		if taseronAdi != nil {
			it := items[id]
			it.Eslesmeler = append(it.Eslesmeler, sozlesmeTakipEslesme{
				TaseronAdi: *taseronAdi, SozlesmeNo: sozlesmeNo, SozlesmeTuru: sozlesmeTuru, SozlesmeTarihi: sozlesmeTarihi,
			})
			items[id] = it
		}
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}
	out := make([]sozlesmeTakipItem, 0, len(order))
	for _, id := range order {
		out = append(out, items[id])
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}
