// Package idarihakedis — İdari Hakedişler: idare (işveren) tarafından ana
// yükleniciye ödenen hakedişler — Nakit Akış'ın tek "in" (giriş) kaynağı.
//
// Kapsam bilinçli olarak dar tutuldu: idarenin kendi onay süreci sistem
// DIŞINDA gerçekleşir (idare zaten onaylamıştır), bu yüzden burada saha
// tutanaklarındaki gibi ayrı bir çok adımlı onay zinciri KURULMAZ — doğrudan
// "onaylanmış kayıt girişi" formu. Fatura mevcut polimorfik documents
// motoruyla bağlanır (entity_type='idari_hakedis_fatura').
//
// gelen_odeme_tarihi alanı doldurulunca/güncellenince/temizlenince
// cash_events'e (direction='in') karşılık gelen satır yazılır/güncellenir/
// silinir — nakit akışına giriş burada tetiklenir.
package idarihakedis

import (
	"context"
	"errors"
	"net/http"
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

type IdariHakedis struct {
	ID               uuid.UUID `json:"id"`
	ProjectID        uuid.UUID `json:"project_id"`
	DonemNo          int       `json:"donem_no"`
	Aciklama         string    `json:"aciklama"`
	Tutar            float64   `json:"tutar"`
	KdvPct           float64   `json:"kdv_pct"`
	FaturaNo         *string   `json:"fatura_no,omitempty"`
	GelenOdemeTarihi *string   `json:"gelen_odeme_tarihi,omitempty"`
	CreatedByName    string    `json:"created_by_name"`
	CreatedAt        time.Time `json:"created_at"`
	RowVersion       int       `json:"row_version"`
}

const listCols = `
	i.id, i.project_id, i.donem_no, i.aciklama, i.tutar::float8, i.kdv_pct::float8,
	i.fatura_no, to_char(i.gelen_odeme_tarihi,'YYYY-MM-DD'), u.full_name, i.created_at, i.row_version`

func scanRow(row pgx.Row, i *IdariHakedis) error {
	return row.Scan(&i.ID, &i.ProjectID, &i.DonemNo, &i.Aciklama, &i.Tutar, &i.KdvPct,
		&i.FaturaNo, &i.GelenOdemeTarihi, &i.CreatedByName, &i.CreatedAt, &i.RowVersion)
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

// ── List ─────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+listCols+`
		FROM idari_hakedisler i
		JOIN users u ON u.id = i.created_by
		WHERE i.project_id=$1 AND i.deleted_at IS NULL
		ORDER BY i.donem_no DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []IdariHakedis{}
	for rows.Next() {
		var i IdariHakedis
		if err := scanRow(rows, &i); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, i)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"idari_hakedisler": out})
}

// ── Create ───────────────────────────────────────────────────────────────

type createReq struct {
	DonemNo          int      `json:"donem_no"`
	Aciklama         string   `json:"aciklama"`
	Tutar            float64  `json:"tutar"`
	KdvPct           *float64 `json:"kdv_pct"`
	FaturaNo         *string  `json:"fatura_no"`
	GelenOdemeTarihi *string  `json:"gelen_odeme_tarihi"`
}

func validate(donemNo int, aciklama string, tutar float64, gelenOdemeTarihi *string) map[string]string {
	f := map[string]string{}
	if donemNo <= 0 {
		f["donem_no"] = "0'dan büyük olmalı"
	}
	if strings.TrimSpace(aciklama) == "" {
		f["aciklama"] = "açıklama zorunlu"
	}
	if tutar <= 0 {
		f["tutar"] = "0'dan büyük olmalı"
	}
	if gelenOdemeTarihi != nil && strings.TrimSpace(*gelenOdemeTarihi) != "" {
		if _, err := time.Parse("2006-01-02", *gelenOdemeTarihi); err != nil {
			f["gelen_odeme_tarihi"] = "geçerli bir tarih (YYYY-MM-DD) girin"
		}
	}
	return f
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
	var req createReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req.DonemNo, req.Aciklama, req.Tutar, req.GelenOdemeTarihi); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	kdvPct := 20.0
	if req.KdvPct != nil {
		kdvPct = *req.KdvPct
	}
	gelenTarih := strDeref(req.GelenOdemeTarihi)

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO idari_hakedisler
			(project_id, donem_no, aciklama, tutar, kdv_pct, fatura_no, gelen_odeme_tarihi, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::date,$8)
		RETURNING id`,
		pid, req.DonemNo, strings.TrimSpace(req.Aciklama), req.Tutar, kdvPct, req.FaturaNo, gelenTarih, uid,
	).Scan(&id); err != nil {
		if isUniqueViolation(err) {
			httpx.ValidationFailed(w, r, map[string]string{"donem_no": "bu dönem no zaten kullanılıyor"})
			return
		}
		httpx.Internal(w, r)
		return
	}
	if gelenTarih != "" {
		if err := writeCashEvent(r.Context(), tx, pid, id, req.Aciklama, req.Tutar, gelenTarih, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

// ── Update ───────────────────────────────────────────────────────────────

type updateReq struct {
	Aciklama         string   `json:"aciklama"`
	Tutar            float64  `json:"tutar"`
	KdvPct           *float64 `json:"kdv_pct"`
	FaturaNo         *string  `json:"fatura_no"`
	GelenOdemeTarihi *string  `json:"gelen_odeme_tarihi"`
	RowVersion       int      `json:"row_version"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req updateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// donem_no update edilmiyor (kimlik alanı gibi davranır — kaza ile
	// dönem numarasını değiştirmek nakit akışı geçmişini karıştırabilir).
	if f := validate(1, req.Aciklama, req.Tutar, req.GelenOdemeTarihi); len(f) > 0 {
		delete(f, "donem_no")
		httpx.ValidationFailed(w, r, f)
		return
	}
	kdvPct := 20.0
	if req.KdvPct != nil {
		kdvPct = *req.KdvPct
	}
	gelenTarih := strDeref(req.GelenOdemeTarihi)

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE idari_hakedisler SET
			aciklama=$3, tutar=$4, kdv_pct=$5, fatura_no=$6,
			gelen_odeme_tarihi=NULLIF($7,'')::date, updated_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND row_version=$8`,
		id, pid, strings.TrimSpace(req.Aciklama), req.Tutar, kdvPct, req.FaturaNo, gelenTarih, req.RowVersion)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt bulunamadı ya da başka biri tarafından güncellenmiş (sayfayı yenileyin).", nil)
		return
	}
	// cash_events'i güncel duruma eşitle: her zaman sil, tarih hâlâ doluysa yeniden yaz.
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM cash_events WHERE source_entity='idari_hakedis' AND source_id=$1`, id); err != nil {
		httpx.Internal(w, r)
		return
	}
	if gelenTarih != "" {
		if err := writeCashEvent(r.Context(), tx, pid, id, req.Aciklama, req.Tutar, gelenTarih, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ── Delete ───────────────────────────────────────────────────────────────

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE idari_hakedisler SET deleted_at=now()
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kayıt bulunamadı.", nil)
		return
	}
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM cash_events WHERE source_entity='idari_hakedis' AND source_id=$1`, id); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── Yardımcılar ──────────────────────────────────────────────────────────

// writeCashEvent — idari hakedişin gelen ödeme tarihini nakit akışı
// defterine "in" satırı olarak yazar (tek doğruluk kaynağı: bu fonksiyon).
func writeCashEvent(ctx context.Context, tx pgx.Tx, pid, id uuid.UUID, aciklama string, tutar float64, eventDate string, uid uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO cash_events (project_id, direction, source_entity, source_id, description, amount, event_date, created_by)
		VALUES ($1,'in','idari_hakedis',$2,$3,$4,$5::date,$6)`,
		pid, id, "İdari Hakediş — "+aciklama, tutar, eventDate, uid)
	return err
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
