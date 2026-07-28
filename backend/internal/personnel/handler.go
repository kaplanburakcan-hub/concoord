// Package personnel — Personel kaydı ve günlük puantaj CRUD.
package personnel

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ db *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{db: pool} }

// ── Personel ──────────────────────────────────────────────────────────────────

type Personel struct {
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	AdSoyad   string `json:"ad_soyad"`
	Gorev     string `json:"gorev"`
	Firma     string `json:"firma"`
	IsAktif   bool   `json:"is_aktif"`
	Sira      int    `json:"sira"`
}

func (h *Handler) ListPersonel(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, project_id, ad_soyad, gorev, COALESCE(firma,''), is_aktif, sira
		FROM project_personnel
		WHERE project_id=$1
		ORDER BY sira, ad_soyad`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	list := []Personel{}
	for rows.Next() {
		var p Personel
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.AdSoyad, &p.Gorev, &p.Firma, &p.IsAktif, &p.Sira); err != nil {
			httpx.Internal(w, r)
			return
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"personnel": list})
}

func (h *Handler) CreatePersonel(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	var body Personel
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.AdSoyad == "" {
		httpx.ValidationFailed(w, r, map[string]string{"ad_soyad": "Boş bırakılamaz"})
		return
	}
	if body.Gorev == "" {
		body.Gorev = "İşçi"
	}

	var id string
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO project_personnel (project_id, ad_soyad, gorev, firma, is_aktif, sira)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		pid, body.AdSoyad, body.Gorev, nilStr(body.Firma), body.IsAktif, body.Sira,
	).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *Handler) UpdatePersonel(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	personelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz personel ID.", nil)
		return
	}
	var body Personel
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	tag, err := h.db.Exec(r.Context(), `
		UPDATE project_personnel SET
		    ad_soyad=COALESCE(NULLIF($3,''), ad_soyad),
		    gorev=COALESCE(NULLIF($4,''), gorev),
		    firma=$5, is_aktif=$6, sira=$7, updated_at=NOW()
		WHERE id=$1 AND project_id=$2`,
		personelID, pid, body.AdSoyad, body.Gorev, nilStr(body.Firma), body.IsAktif, body.Sira,
	)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Personel bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) DeletePersonel(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	personelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz personel ID.", nil)
		return
	}
	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM project_personnel WHERE id=$1 AND project_id=$2`, personelID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Personel bulunamadı.", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Puantaj ───────────────────────────────────────────────────────────────────

type PuantajEntry struct {
	ID         string  `json:"id,omitempty"`
	PersonelID string  `json:"personel_id"`
	Tarih      string  `json:"tarih"`
	Durum      string  `json:"durum"`
	MesaiSaat  float64 `json:"mesai_saat"`
	NotMetin   string  `json:"not_metin"`
}

// GetPuantaj — belirli tarih aralığındaki tüm girişleri döner.
func (h *Handler) GetPuantaj(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "from ve to parametreleri gereklidir.", nil)
		return
	}

	rows, err := h.db.Query(r.Context(), `
		SELECT id, personel_id, TO_CHAR(tarih,'YYYY-MM-DD'), durum, mesai_saat, COALESCE(not_metin,'')
		FROM project_puantaj
		WHERE project_id=$1 AND tarih BETWEEN $2::date AND $3::date
		ORDER BY tarih, personel_id`, pid, from, to)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	entries := []PuantajEntry{}
	for rows.Next() {
		var e PuantajEntry
		if err := rows.Scan(&e.ID, &e.PersonelID, &e.Tarih, &e.Durum, &e.MesaiSaat, &e.NotMetin); err != nil {
			httpx.Internal(w, r)
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// UpsertPuantaj — tek bir giriş ekler/günceller (personel_id + tarih unique).
func (h *Handler) UpsertPuantaj(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	var body PuantajEntry
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.PersonelID == "" || body.Tarih == "" {
		httpx.ValidationFailed(w, r, map[string]string{
			"personel_id": "Gereklidir",
			"tarih":       "Gereklidir",
		})
		return
	}
	validDurum := map[string]bool{"mevcut": true, "devamsiz": true, "izinli": true, "raporlu": true}
	if !validDurum[body.Durum] {
		body.Durum = "mevcut"
	}
	personelID, err := uuid.Parse(body.PersonelID)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz personel ID.", nil)
		return
	}

	var id string
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO project_puantaj (project_id, personel_id, tarih, durum, mesai_saat, not_metin)
		VALUES ($1,$2,$3::date,$4,$5,$6)
		ON CONFLICT (personel_id, tarih) DO UPDATE SET
		    durum=EXCLUDED.durum, mesai_saat=EXCLUDED.mesai_saat,
		    not_metin=EXCLUDED.not_metin, updated_at=NOW()
		RETURNING id`,
		pid, personelID, body.Tarih, body.Durum, body.MesaiSaat, nilStr(body.NotMetin),
	).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id})
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
