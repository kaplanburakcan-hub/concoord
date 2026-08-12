// Package stakeholders — Proje Paydaşları (İşveren, Müşavir, Yüklenici,
// Taşeron personeli, Yapı Denetim, Proje Müellifi, Danışmanlar, İSG-OSGB,
// Tedarikçiler). Önceden tamamen tarayıcı localStorage'ındaydı; bu paket
// onu gerçek, projeler ve kullanıcılar arasında paylaşılan bir varlığa
// taşır. Taşeron-firma (kategori_id="alt_yuklenici" + tip="firma") satırları
// bu tabloya DEĞİL, mevcut subcontractors tablosuna gider — o özel yol
// frontend'de zaten doğru şekilde uygulanıyordu, burada dokunulmaz.
package stakeholders

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

type Stakeholder struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	KategoriID   string    `json:"kategori_id"`
	AltKirilimID *string   `json:"alt_kirilim_id,omitempty"`
	Tip          string    `json:"tip"`
	Ad           string    `json:"ad"`
	Soyad        *string   `json:"soyad,omitempty"`
	Unvan        *string   `json:"unvan,omitempty"`
	FirmaAdi     *string   `json:"firma_adi,omitempty"`
	Telefon      *string   `json:"telefon,omitempty"`
	Email        *string   `json:"email,omitempty"`
	Notlar       *string   `json:"notlar,omitempty"`
	RowVersion   int       `json:"row_version"`
	CreatedAt    time.Time `json:"created_at"`
}

const cols = `id, project_id, kategori_id, alt_kirilim_id, tip, ad, soyad, unvan, firma_adi,
	telefon, email, notlar, row_version, created_at`

func scanRow(row pgx.Row, s *Stakeholder) error {
	return row.Scan(&s.ID, &s.ProjectID, &s.KategoriID, &s.AltKirilimID, &s.Tip, &s.Ad, &s.Soyad,
		&s.Unvan, &s.FirmaAdi, &s.Telefon, &s.Email, &s.Notlar, &s.RowVersion, &s.CreatedAt)
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

func requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, false
	}
	return uid, true
}

// ── List ──────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	q := r.URL.Query()
	kategoriID := strings.TrimSpace(q.Get("kategori_id"))
	altKirilimID := strings.TrimSpace(q.Get("alt_kirilim_id"))

	rows, err := h.pool.Query(r.Context(), `
		SELECT `+cols+`
		FROM project_stakeholders
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND ($2='' OR kategori_id=$2)
		  AND ($3='' OR alt_kirilim_id=$3)
		ORDER BY created_at`,
		pid, kategoriID, altKirilimID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Stakeholder{}
	for rows.Next() {
		var s Stakeholder
		if err := scanRow(rows, &s); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, s)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"stakeholders": out})
}

// ── Create ────────────────────────────────────────────────────────────────

type upsertReq struct {
	KategoriID   string  `json:"kategori_id"`
	AltKirilimID *string `json:"alt_kirilim_id"`
	Tip          string  `json:"tip"`
	Ad           string  `json:"ad"`
	Soyad        *string `json:"soyad"`
	Unvan        *string `json:"unvan"`
	FirmaAdi     *string `json:"firma_adi"`
	Telefon      *string `json:"telefon"`
	Email        *string `json:"email"`
	Notlar       *string `json:"notlar"`
	RowVersion   int     `json:"row_version"`
}

func validate(req upsertReq) map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(req.KategoriID) == "" {
		f["kategori_id"] = "zorunlu"
	}
	if req.Tip != "kisi" && req.Tip != "firma" {
		f["tip"] = "kisi veya firma olmalı"
	}
	if strings.TrimSpace(req.Ad) == "" && (req.FirmaAdi == nil || strings.TrimSpace(*req.FirmaAdi) == "") {
		f["ad"] = "ad veya firma adı zorunlu"
	}
	return f
}

func normalize(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req upsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	var s Stakeholder
	err := scanRow(h.pool.QueryRow(r.Context(), `
		INSERT INTO project_stakeholders
			(project_id, kategori_id, alt_kirilim_id, tip, ad, soyad, unvan, firma_adi, telefon, email, notlar, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+cols,
		pid, strings.TrimSpace(req.KategoriID), normalize(req.AltKirilimID), req.Tip, strings.TrimSpace(req.Ad),
		normalize(req.Soyad), normalize(req.Unvan), normalize(req.FirmaAdi), normalize(req.Telefon),
		normalize(req.Email), normalize(req.Notlar), uid), &s)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"stakeholder": s})
}

// ── BulkCreate (demo veri yükleme) ──────────────────────────────────────────

type bulkCreateReq struct {
	Items []upsertReq `json:"items"`
}

func (h *Handler) BulkCreate(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req bulkCreateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	for i, item := range req.Items {
		if f := validate(item); len(f) > 0 {
			httpx.ValidationFailed(w, r, map[string]string{"items": "geçersiz satır #" + strconv.Itoa(i)})
			return
		}
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())
	for _, item := range req.Items {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO project_stakeholders
				(project_id, kategori_id, alt_kirilim_id, tip, ad, soyad, unvan, firma_adi, telefon, email, notlar, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			pid, strings.TrimSpace(item.KategoriID), normalize(item.AltKirilimID), item.Tip, strings.TrimSpace(item.Ad),
			normalize(item.Soyad), normalize(item.Unvan), normalize(item.FirmaAdi), normalize(item.Telefon),
			normalize(item.Email), normalize(item.Notlar), uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"inserted": len(req.Items)})
}

// ── Update ────────────────────────────────────────────────────────────────

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req upsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before Stakeholder
	err = scanRow(tx.QueryRow(r.Context(),
		`SELECT `+cols+` FROM project_stakeholders WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`,
		sid, pid), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Paydaş bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}
	var s Stakeholder
	err = scanRow(tx.QueryRow(r.Context(), `
		UPDATE project_stakeholders SET
			kategori_id=$3, alt_kirilim_id=$4, tip=$5, ad=$6, soyad=$7, unvan=$8,
			firma_adi=$9, telefon=$10, email=$11, notlar=$12, row_version=row_version+1
		WHERE id=$1 AND project_id=$2
		RETURNING `+cols,
		sid, pid, strings.TrimSpace(req.KategoriID), normalize(req.AltKirilimID), req.Tip, strings.TrimSpace(req.Ad),
		normalize(req.Soyad), normalize(req.Unvan), normalize(req.FirmaAdi), normalize(req.Telefon),
		normalize(req.Email), normalize(req.Notlar)), &s)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"stakeholder": s})
}

// ── Delete ────────────────────────────────────────────────────────────────

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE project_stakeholders SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, sid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Paydaş bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
