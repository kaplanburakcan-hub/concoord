// Package statements — Tedarikçi ekstreler (supplier_statements) CRUD.
package statements

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
)

type Handler struct {
	db *pgxpool.Pool
	nt *notify.Service
}

func NewHandler(pool *pgxpool.Pool, nt *notify.Service) *Handler { return &Handler{db: pool, nt: nt} }

type Statement struct {
	ID           string  `json:"id,omitempty"`
	ProjectID    string  `json:"project_id,omitempty"`
	TedarikciAdi string  `json:"tedarikci_adi"`
	EkstreNo     string  `json:"ekstre_no"`
	EkstreTarihi string  `json:"ekstre_tarihi"`
	VadeTarihi   string  `json:"vade_tarihi"`
	ToplamTutar  float64 `json:"toplam_tutar"`
	OdenenTutar  float64 `json:"odenen_tutar"`
	ParaBirimi   string  `json:"para_birimi"`
	OdemeDurumu  string  `json:"odeme_durumu"`
	Aciklama     string  `json:"aciklama"`
}

var validDurum = map[string]bool{
	"bekliyor": true, "kismi_odendi": true, "odendi": true, "iptal": true,
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, project_id, tedarikci_adi, ekstre_no,
		       TO_CHAR(ekstre_tarihi,'YYYY-MM-DD'),
		       COALESCE(TO_CHAR(vade_tarihi,'YYYY-MM-DD'),''),
		       toplam_tutar, odenen_tutar, para_birimi, odeme_durumu,
		       COALESCE(aciklama,'')
		FROM supplier_statements
		WHERE project_id=$1
		ORDER BY ekstre_tarihi DESC, tedarikci_adi`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	list := []Statement{}
	for rows.Next() {
		var s Statement
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.TedarikciAdi, &s.EkstreNo,
			&s.EkstreTarihi, &s.VadeTarihi, &s.ToplamTutar, &s.OdenenTutar,
			&s.ParaBirimi, &s.OdemeDurumu, &s.Aciklama); err != nil {
			httpx.Internal(w, r)
			return
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"statements": list})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	var body Statement
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	errs := map[string]string{}
	if body.TedarikciAdi == "" {
		errs["tedarikci_adi"] = "Boş bırakılamaz"
	}
	if body.EkstreNo == "" {
		errs["ekstre_no"] = "Boş bırakılamaz"
	}
	if body.EkstreTarihi == "" {
		errs["ekstre_tarihi"] = "Boş bırakılamaz"
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	if body.ParaBirimi == "" {
		body.ParaBirimi = "TRY"
	}
	if !validDurum[body.OdemeDurumu] {
		body.OdemeDurumu = "bekliyor"
	}

	var id string
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO supplier_statements
		    (project_id, tedarikci_adi, ekstre_no, ekstre_tarihi, vade_tarihi,
		     toplam_tutar, odenen_tutar, para_birimi, odeme_durumu, aciklama)
		VALUES ($1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10)
		RETURNING id`,
		pid, body.TedarikciAdi, body.EkstreNo, body.EkstreTarihi,
		nilDate(body.VadeTarihi), body.ToplamTutar, body.OdenenTutar,
		body.ParaBirimi, body.OdemeDurumu, nilStr(body.Aciklama),
	).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	stmtID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ekstre ID.", nil)
		return
	}
	var body Statement
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.OdemeDurumu != "" && !validDurum[body.OdemeDurumu] {
		httpx.ValidationFailed(w, r, map[string]string{"odeme_durumu": "Geçersiz durum."})
		return
	}
	if body.ParaBirimi == "" {
		body.ParaBirimi = "TRY"
	}

	tag, err := h.db.Exec(r.Context(), `
		UPDATE supplier_statements SET
		    tedarikci_adi=COALESCE(NULLIF($3,''), tedarikci_adi),
		    ekstre_no=COALESCE(NULLIF($4,''), ekstre_no),
		    ekstre_tarihi=COALESCE(NULLIF($5,'')::date, ekstre_tarihi),
		    vade_tarihi=$6,
		    toplam_tutar=$7, odenen_tutar=$8,
		    para_birimi=COALESCE(NULLIF($9,''), para_birimi),
		    odeme_durumu=COALESCE(NULLIF($10,''), odeme_durumu),
		    aciklama=$11, updated_at=NOW()
		WHERE id=$1 AND project_id=$2`,
		stmtID, pid,
		body.TedarikciAdi, body.EkstreNo, body.EkstreTarihi,
		nilDate(body.VadeTarihi), body.ToplamTutar, body.OdenenTutar,
		body.ParaBirimi, body.OdemeDurumu, nilStr(body.Aciklama),
	)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ekstre bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	stmtID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ekstre ID.", nil)
		return
	}
	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM supplier_statements WHERE id=$1 AND project_id=$2`, stmtID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ekstre bulunamadı.", nil)
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
