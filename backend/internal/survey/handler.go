// Package survey — Proje Keşfi (project_survey_items) CRUD.
package survey

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct {
	db  *pgxpool.Pool
	rec *audit.Recorder
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder) *Handler {
	return &Handler{db: pool, rec: rec}
}

type Item struct {
	ID              string  `json:"id,omitempty"`
	ProjectID       string  `json:"project_id,omitempty"`
	Kategori        string  `json:"kategori"`
	PozNo           string  `json:"poz_no"`
	Tanim           string  `json:"tanim"`
	Birim           string  `json:"birim"`
	Miktar          float64 `json:"miktar"`
	BirimFiyat      float64 `json:"birim_fiyat"`
	ParaBirimi      string  `json:"para_birimi"`
	Aciklama        string  `json:"aciklama"`
	Sira            int     `json:"sira"`
	WorkItemID      *string `json:"work_item_id"`
	SubcontractorAd *string `json:"subcontractor_adi"`
}

// List — proje keşif kalemlerini kategori/sıra sırasıyla döner.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT psi.id, psi.project_id, psi.kategori, COALESCE(psi.poz_no,''), psi.tanim,
		       psi.birim, psi.miktar, psi.birim_fiyat, COALESCE(psi.para_birimi,'TRY'),
		       COALESCE(psi.aciklama,''), psi.sira, psi.work_item_id, s.company_name
		FROM project_survey_items psi
		LEFT JOIN work_items wi ON wi.id = psi.work_item_id
		LEFT JOIN subcontractors s ON s.id = wi.subcontractor_id
		WHERE psi.project_id=$1
		ORDER BY psi.kategori, psi.sira, psi.tanim`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ProjectID, &it.Kategori, &it.PozNo, &it.Tanim,
			&it.Birim, &it.Miktar, &it.BirimFiyat, &it.ParaBirimi, &it.Aciklama, &it.Sira,
			&it.WorkItemID, &it.SubcontractorAd); err != nil {
			httpx.Internal(w, r)
			return
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// Create — yeni kalem ekler.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}

	var body Item
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Tanim == "" || body.Kategori == "" {
		httpx.ValidationFailed(w, r, map[string]string{
			"tanim":    "Boş bırakılamaz",
			"kategori": "Boş bırakılamaz",
		})
		return
	}
	if body.Birim == "" {
		body.Birim = "adet"
	}
	if body.ParaBirimi == "" {
		body.ParaBirimi = "TRY"
	}

	var id string
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO project_survey_items
		    (project_id, kategori, poz_no, tanim, birim, miktar, birim_fiyat, para_birimi, aciklama, sira)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id`,
		pid, body.Kategori, nilStr(body.PozNo), body.Tanim,
		body.Birim, body.Miktar, body.BirimFiyat, body.ParaBirimi,
		nilStr(body.Aciklama), body.Sira,
	).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

// Update — kalemi günceller.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kalem ID.", nil)
		return
	}

	var body Item
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	tag, err := h.db.Exec(r.Context(), `
		UPDATE project_survey_items SET
		    kategori=COALESCE(NULLIF($3,''), kategori),
		    poz_no=$4, tanim=COALESCE(NULLIF($5,''), tanim),
		    birim=COALESCE(NULLIF($6,''), birim),
		    miktar=$7, birim_fiyat=$8,
		    para_birimi=COALESCE(NULLIF($9,''), para_birimi),
		    aciklama=$10, sira=$11, updated_at=NOW()
		WHERE id=$1 AND project_id=$2`,
		itemID, pid,
		body.Kategori, nilStr(body.PozNo), body.Tanim,
		body.Birim, body.Miktar, body.BirimFiyat, body.ParaBirimi,
		nilStr(body.Aciklama), body.Sira,
	)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kalem bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Delete — kalemi siler.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kalem ID.", nil)
		return
	}

	tag, err := h.db.Exec(r.Context(), `
		DELETE FROM project_survey_items WHERE id=$1 AND project_id=$2`, itemID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kalem bulunamadı.", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
