// Package design — Tasarım ve Projeler (project_design_docs) CRUD.
package design

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ db *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{db: pool} }

type Doc struct {
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Disiplin  string `json:"disiplin"`
	PozNo     string `json:"poz_no"`
	Baslik    string `json:"baslik"`
	RevNo     string `json:"rev_no"`
	Tarih     string `json:"tarih"`
	Durum     string `json:"durum"`
	Aciklama  string `json:"aciklama"`
	Sira      int    `json:"sira"`
}

var validDurum = map[string]bool{
	"taslak": true, "incelemede": true, "onaylı": true,
	"revizyon_gerekli": true, "iptal": true,
}

// List — projeye ait tüm çizimleri disiplin/sıra sırasıyla döner.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, project_id, disiplin,
		       COALESCE(poz_no,''), baslik, rev_no,
		       COALESCE(TO_CHAR(tarih,'YYYY-MM-DD'),''), durum,
		       COALESCE(aciklama,''), sira
		FROM project_design_docs
		WHERE project_id=$1
		ORDER BY disiplin, sira, baslik`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	docs := []Doc{}
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Disiplin, &d.PozNo,
			&d.Baslik, &d.RevNo, &d.Tarih, &d.Durum, &d.Aciklama, &d.Sira); err != nil {
			httpx.Internal(w, r)
			return
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"docs": docs})
}

// Create — yeni çizim/doküman ekler.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}

	var body Doc
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	errs := map[string]string{}
	if body.Baslik == "" {
		errs["baslik"] = "Boş bırakılamaz"
	}
	if body.Disiplin == "" {
		errs["disiplin"] = "Boş bırakılamaz"
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	if body.RevNo == "" {
		body.RevNo = "0"
	}
	if body.Durum == "" || !validDurum[body.Durum] {
		body.Durum = "taslak"
	}

	var id string
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO project_design_docs
		    (project_id, disiplin, poz_no, baslik, rev_no, tarih, durum, aciklama, sira)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`,
		pid, body.Disiplin, nilStr(body.PozNo), body.Baslik, body.RevNo,
		nilDate(body.Tarih), body.Durum, nilStr(body.Aciklama), body.Sira,
	).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

// Update — çizimi günceller.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz doküman ID.", nil)
		return
	}

	var body Doc
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Durum != "" && !validDurum[body.Durum] {
		httpx.ValidationFailed(w, r, map[string]string{"durum": "Geçersiz durum."})
		return
	}

	tag, err := h.db.Exec(r.Context(), `
		UPDATE project_design_docs SET
		    disiplin=COALESCE(NULLIF($3,''), disiplin),
		    poz_no=$4,
		    baslik=COALESCE(NULLIF($5,''), baslik),
		    rev_no=COALESCE(NULLIF($6,''), rev_no),
		    tarih=$7,
		    durum=COALESCE(NULLIF($8,''), durum),
		    aciklama=$9,
		    sira=$10,
		    updated_at=NOW()
		WHERE id=$1 AND project_id=$2`,
		docID, pid,
		body.Disiplin, nilStr(body.PozNo), body.Baslik, body.RevNo,
		nilDate(body.Tarih), body.Durum, nilStr(body.Aciklama), body.Sira,
	)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Doküman bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Delete — çizimi siler.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz doküman ID.", nil)
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM project_design_docs WHERE id=$1 AND project_id=$2`, docID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Doküman bulunamadı.", nil)
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

func nilDate(s string) any {
	if s == "" {
		return nil
	}
	return s
}
