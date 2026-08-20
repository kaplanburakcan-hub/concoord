// Package idarihakedis — İdari Hakedişler: idare (işveren) tarafından ana
// yükleniciye ödenen hakedişler — Nakit Akış'ın tek "in" (giriş) kaynağı.
//
// Kapsam bilinçli olarak dar tutuldu: idarenin kendi onay süreci sistem
// DIŞINDA gerçekleşir (idare zaten onaylamıştır), bu yüzden burada saha
// tutanaklarındaki gibi ayrı bir çok adımlı onay zinciri KURULMAZ — doğrudan
// "onaylanmış kayıt girişi" formu. Fatura ve hakediş belgesi (imzalı kapak
// sayfası/komple hakediş) mevcut polimorfik documents motoruyla bağlanır
// (entity_type='idari_hakedis_fatura', kategori IdariHakedisFatura/
// IdariHakedisBelgesi).
//
// gelen_odeme_tarihi alanı doldurulunca/güncellenince/temizlenince
// cash_events'e (direction='in') karşılık gelen satır yazılır/güncellenir/
// silinir — nakit akışına giriş burada tetiklenir.
//
// tutar KULLANICIDAN ALINMAZ — gerçek hakediş raporu düzenine göre
// (Sözleşme Fiyatları+Fiyat Farkı=C, C-Önceki=E, E×KDV%=F, E+F=G,
// Σkesintiler=H, G-H=Yükleniciye Ödenecek) burada hesaplanır (bkz. calc).
package idarihakedis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// KesintiKalem — rapordaki a-i kesinti/mahsup kalemleri + serbest ek satırlar.
type KesintiKalem struct {
	Ad    string  `json:"ad"`
	Tutar float64 `json:"tutar"`
}

type IdariHakedis struct {
	ID                      uuid.UUID      `json:"id"`
	ProjectID               uuid.UUID      `json:"project_id"`
	DonemNo                 int            `json:"donem_no"`
	Aciklama                string         `json:"aciklama"`
	HakedisTarihi           *string        `json:"hakedis_tarihi,omitempty"`
	SozlesmeFiyatlariTutari float64        `json:"sozlesme_fiyatlari_tutari"`
	FiyatFarkiTutari        float64        `json:"fiyat_farki_tutari"`
	OncekiHakedisToplami    float64        `json:"onceki_hakedis_toplami"`
	KdvPct                  float64        `json:"kdv_pct"`
	Kesintiler              []KesintiKalem `json:"kesintiler"`
	Tutar                   float64        `json:"tutar"` // = Yükleniciye Ödenecek Tutar (G-H)
	FaturaNo                *string        `json:"fatura_no,omitempty"`
	GelenOdemeTarihi        *string        `json:"gelen_odeme_tarihi,omitempty"`
	CreatedByName           string         `json:"created_by_name"`
	CreatedAt               time.Time      `json:"created_at"`
	RowVersion              int            `json:"row_version"`
}

const listCols = `
	i.id, i.project_id, i.donem_no, i.aciklama,
	to_char(i.hakedis_tarihi,'YYYY-MM-DD'),
	i.sozlesme_fiyatlari_tutari::float8, i.fiyat_farki_tutari::float8,
	i.onceki_hakedis_toplami::float8, i.kdv_pct::float8, i.kesintiler,
	i.tutar::float8, i.fatura_no, to_char(i.gelen_odeme_tarihi,'YYYY-MM-DD'),
	u.full_name, i.created_at, i.row_version`

func scanRow(row pgx.Row, i *IdariHakedis) error {
	var kesintilerJSON []byte
	if err := row.Scan(&i.ID, &i.ProjectID, &i.DonemNo, &i.Aciklama,
		&i.HakedisTarihi,
		&i.SozlesmeFiyatlariTutari, &i.FiyatFarkiTutari,
		&i.OncekiHakedisToplami, &i.KdvPct, &kesintilerJSON,
		&i.Tutar, &i.FaturaNo, &i.GelenOdemeTarihi,
		&i.CreatedByName, &i.CreatedAt, &i.RowVersion); err != nil {
		return err
	}
	if err := json.Unmarshal(kesintilerJSON, &i.Kesintiler); err != nil {
		i.Kesintiler = []KesintiKalem{}
	}
	return nil
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

// calc — rapor düzenindeki A-H hesap zinciri.
// c=A+B, e=c-d(önceki), f=e×kdv%, g=e+f, hSum=Σkesintiler, odenecek=g-hSum.
func calc(a, b, d, kdvPct float64, kesintiler []KesintiKalem) (c, e, f, g, hSum, odenecek float64) {
	c = a + b
	e = c - d
	f = e * kdvPct / 100
	g = e + f
	for _, k := range kesintiler {
		hSum += k.Tutar
	}
	odenecek = g - hSum
	return
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

type reqBody struct {
	DonemNo                 int            `json:"donem_no"`
	Aciklama                string         `json:"aciklama"`
	HakedisTarihi           *string        `json:"hakedis_tarihi"`
	SozlesmeFiyatlariTutari float64        `json:"sozlesme_fiyatlari_tutari"`
	FiyatFarkiTutari        float64        `json:"fiyat_farki_tutari"`
	OncekiHakedisToplami    float64        `json:"onceki_hakedis_toplami"`
	KdvPct                  *float64       `json:"kdv_pct"`
	Kesintiler              []KesintiKalem `json:"kesintiler"`
	FaturaNo                *string        `json:"fatura_no"`
	GelenOdemeTarihi        *string        `json:"gelen_odeme_tarihi"`
	RowVersion              int            `json:"row_version"`
}

func validate(donemNo int, sozlesmeFiyatlariTutari float64, hakedisTarihi, gelenOdemeTarihi *string) map[string]string {
	f := map[string]string{}
	if donemNo <= 0 {
		f["donem_no"] = "0'dan büyük olmalı"
	}
	if sozlesmeFiyatlariTutari < 0 {
		f["sozlesme_fiyatlari_tutari"] = "negatif olamaz"
	}
	if s := strDeref(hakedisTarihi); s != "" {
		if _, err := time.Parse("2006-01-02", s); err != nil {
			f["hakedis_tarihi"] = "geçerli bir tarih (YYYY-MM-DD) girin"
		}
	}
	if s := strDeref(gelenOdemeTarihi); s != "" {
		if _, err := time.Parse("2006-01-02", s); err != nil {
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
	var req reqBody
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req.DonemNo, req.SozlesmeFiyatlariTutari, req.HakedisTarihi, req.GelenOdemeTarihi); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	kdvPct := 20.0
	if req.KdvPct != nil {
		kdvPct = *req.KdvPct
	}
	aciklama := strings.TrimSpace(req.Aciklama)
	if aciklama == "" {
		aciklama = fmt.Sprintf("Hakediş No %d", req.DonemNo)
	}
	if req.Kesintiler == nil {
		req.Kesintiler = []KesintiKalem{}
	}
	_, _, _, _, _, odenecek := calc(req.SozlesmeFiyatlariTutari, req.FiyatFarkiTutari, req.OncekiHakedisToplami, kdvPct, req.Kesintiler)
	kesintilerJSON, err := json.Marshal(req.Kesintiler)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	gelenTarih := strDeref(req.GelenOdemeTarihi)
	hakedisTarih := strDeref(req.HakedisTarihi)

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO idari_hakedisler
			(project_id, donem_no, aciklama, hakedis_tarihi,
			 sozlesme_fiyatlari_tutari, fiyat_farki_tutari, onceki_hakedis_toplami,
			 kdv_pct, kesintiler, tutar, fatura_no, gelen_odeme_tarihi, created_by)
		VALUES ($1,$2,$3,NULLIF($4,'')::date,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,'')::date,$13)
		RETURNING id`,
		pid, req.DonemNo, aciklama, hakedisTarih,
		req.SozlesmeFiyatlariTutari, req.FiyatFarkiTutari, req.OncekiHakedisToplami,
		kdvPct, kesintilerJSON, odenecek, req.FaturaNo, gelenTarih, uid,
	).Scan(&id); err != nil {
		if isUniqueViolation(err) {
			httpx.ValidationFailed(w, r, map[string]string{"donem_no": "bu dönem no zaten kullanılıyor"})
			return
		}
		httpx.Internal(w, r)
		return
	}
	if gelenTarih != "" {
		if err := writeCashEvent(r.Context(), tx, pid, id, aciklama, odenecek, gelenTarih, uid); err != nil {
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
	var req reqBody
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// donem_no update edilmiyor (kimlik alanı gibi davranır — kaza ile
	// dönem numarasını değiştirmek nakit akışı geçmişini karıştırabilir).
	if f := validate(1, req.SozlesmeFiyatlariTutari, req.HakedisTarihi, req.GelenOdemeTarihi); len(f) > 0 {
		delete(f, "donem_no")
		httpx.ValidationFailed(w, r, f)
		return
	}
	kdvPct := 20.0
	if req.KdvPct != nil {
		kdvPct = *req.KdvPct
	}
	aciklama := strings.TrimSpace(req.Aciklama)
	if aciklama == "" {
		aciklama = "Hakediş"
	}
	if req.Kesintiler == nil {
		req.Kesintiler = []KesintiKalem{}
	}
	_, _, _, _, _, odenecek := calc(req.SozlesmeFiyatlariTutari, req.FiyatFarkiTutari, req.OncekiHakedisToplami, kdvPct, req.Kesintiler)
	kesintilerJSON, err := json.Marshal(req.Kesintiler)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	gelenTarih := strDeref(req.GelenOdemeTarihi)
	hakedisTarih := strDeref(req.HakedisTarihi)

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE idari_hakedisler SET
			aciklama=$3, hakedis_tarihi=NULLIF($4,'')::date,
			sozlesme_fiyatlari_tutari=$5, fiyat_farki_tutari=$6, onceki_hakedis_toplami=$7,
			kdv_pct=$8, kesintiler=$9, tutar=$10, fatura_no=$11,
			gelen_odeme_tarihi=NULLIF($12,'')::date, updated_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND row_version=$13`,
		id, pid, aciklama, hakedisTarih,
		req.SozlesmeFiyatlariTutari, req.FiyatFarkiTutari, req.OncekiHakedisToplami,
		kdvPct, kesintilerJSON, odenecek, req.FaturaNo, gelenTarih, req.RowVersion)
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
		if err := writeCashEvent(r.Context(), tx, pid, id, aciklama, odenecek, gelenTarih, uid); err != nil {
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
