// Package correspondence — Yazışmalar (Gelen/Giden Evrak).
//
// Türkiye'deki kurumsal gelen-giden evrak defteri geleneği ile inşaat
// sektöründeki RFI/Transmittal log pratiğini birleştirir: her kayıt bir
// yöne (gelen/giden) aittir, projeye+yöne göre sıralı bir evrak no taşır
// (PO numaralandırmasıyla aynı advisory-lock deseni), ve "ball-in-court"
// mantığı için cevap_gerekli/cevap_tarihi/durum alanlarına sahiptir.
// Ekler mevcut polimorfik documents motoru üzerinden bağlanır
// (entity_type='correspondence', entity_id=correspondences.id).
package correspondence

import (
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
	kesinkabul "github.com/ipks/ipks/backend/internal/validate"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

// ── DTO ───────────────────────────────────────────────────────────────────────

type Correspondence struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	Direction     string     `json:"direction"`
	EvrakNo       string     `json:"evrak_no"`
	KarsiEvrakNo  *string    `json:"karsi_evrak_no,omitempty"`
	Tarih         string     `json:"tarih"`
	KayitTarihi   string     `json:"kayit_tarihi"`
	KurumKisi     string     `json:"kurum_kisi"`
	Konu          string     `json:"konu"`
	Kategori      string     `json:"kategori"`
	Durum         string     `json:"durum"`
	CevapGerekli  bool       `json:"cevap_gerekli"`
	CevapTarihi   *string    `json:"cevap_tarihi,omitempty"`
	IlgiliYaziID  *uuid.UUID `json:"ilgili_yazi_id,omitempty"`
	IlgiliEvrakNo *string    `json:"ilgili_evrak_no,omitempty"`
	Dagitim       *string    `json:"dagitim,omitempty"`
	Notlar        *string    `json:"notlar,omitempty"`
	TeslimYontemi string     `json:"teslim_yontemi"`
	CreatedByName string     `json:"created_by_name"`
	RowVersion    int        `json:"row_version"`
	CreatedAt     time.Time  `json:"created_at"`
}

const listCols = `
	c.id, c.project_id, c.direction, c.evrak_no, c.karsi_evrak_no,
	to_char(c.tarih,'YYYY-MM-DD'), to_char(c.kayit_tarihi,'YYYY-MM-DD'),
	c.kurum_kisi, c.konu, c.kategori, c.durum, c.cevap_gerekli,
	to_char(c.cevap_tarihi,'YYYY-MM-DD'), c.ilgili_yazi_id, ic.evrak_no,
	c.dagitim, c.notlar, c.teslim_yontemi, u.full_name, c.row_version, c.created_at`

func scanRow(row pgx.Row, c *Correspondence) error {
	return row.Scan(&c.ID, &c.ProjectID, &c.Direction, &c.EvrakNo, &c.KarsiEvrakNo,
		&c.Tarih, &c.KayitTarihi, &c.KurumKisi, &c.Konu, &c.Kategori, &c.Durum, &c.CevapGerekli,
		&c.CevapTarihi, &c.IlgiliYaziID, &c.IlgiliEvrakNo,
		&c.Dagitim, &c.Notlar, &c.TeslimYontemi, &c.CreatedByName, &c.RowVersion, &c.CreatedAt)
}

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	q := r.URL.Query()
	direction := strings.TrimSpace(q.Get("direction"))
	durum := strings.TrimSpace(q.Get("durum"))
	kategori := strings.TrimSpace(q.Get("kategori"))
	teslim := strings.TrimSpace(q.Get("teslim_yontemi"))
	search := strings.TrimSpace(q.Get("q"))

	rows, err := h.pool.Query(r.Context(), `
		SELECT `+listCols+`
		FROM correspondences c
		JOIN users u ON u.id = c.created_by
		LEFT JOIN correspondences ic ON ic.id = c.ilgili_yazi_id
		WHERE c.project_id=$1 AND c.deleted_at IS NULL
		  AND ($2='' OR c.direction=$2)
		  AND ($3='' OR c.durum=$3)
		  AND ($4='' OR c.kategori=$4)
		  AND ($6='' OR c.teslim_yontemi=$6)
		  AND ($5='' OR c.konu ILIKE '%'||$5||'%' OR c.kurum_kisi ILIKE '%'||$5||'%' OR c.evrak_no ILIKE '%'||$5||'%')
		ORDER BY c.tarih DESC, c.created_at DESC`,
		pid, direction, durum, kategori, search, teslim)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Correspondence{}
	for rows.Next() {
		var c Correspondence
		if err := scanRow(rows, &c); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, c)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"correspondences": out})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var c Correspondence
	err := scanRow(h.pool.QueryRow(r.Context(), `
		SELECT `+listCols+`
		FROM correspondences c
		JOIN users u ON u.id = c.created_by
		LEFT JOIN correspondences ic ON ic.id = c.ilgili_yazi_id
		WHERE c.id=$1 AND c.project_id=$2 AND c.deleted_at IS NULL`, id, pid), &c)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Yazışma bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"correspondence": c})
}

// ── Create ────────────────────────────────────────────────────────────────────

type upsertReq struct {
	Direction     string  `json:"direction"`
	KarsiEvrakNo  *string `json:"karsi_evrak_no"`
	Tarih         string  `json:"tarih"`
	KurumKisi     string  `json:"kurum_kisi"`
	Konu          string  `json:"konu"`
	Kategori      *string `json:"kategori"`
	Durum         *string `json:"durum"`
	CevapGerekli  bool    `json:"cevap_gerekli"`
	CevapTarihi   *string `json:"cevap_tarihi"`
	IlgiliYaziID  *string `json:"ilgili_yazi_id"`
	Dagitim       *string `json:"dagitim"`
	Notlar        *string `json:"notlar"`
	TeslimYontemi *string `json:"teslim_yontemi"`
	RowVersion    int     `json:"row_version"`
}

var validKategori = map[string]bool{"Genel": true, "Teknik": true, "İdari": true, "Mali": true, "İSG": true, "Onay Talebi": true}
var validDurum = map[string]bool{"Açık": true, "Cevaplandı": true, "Kapalı": true, "Bilgi Amaçlı": true}
var validTeslim = map[string]bool{"eposta": true, "fiziksel": true}

func validate(req upsertReq) map[string]string {
	f := map[string]string{}
	if req.Direction != "gelen" && req.Direction != "giden" {
		f["direction"] = "gelen veya giden olmalı"
	}
	if _, err := time.Parse("2006-01-02", req.Tarih); err != nil {
		f["tarih"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	}
	if strings.TrimSpace(req.KurumKisi) == "" {
		f["kurum_kisi"] = "kurum/kişi zorunlu"
	}
	if strings.TrimSpace(req.Konu) == "" {
		f["konu"] = "konu zorunlu"
	}
	if req.Kategori != nil && *req.Kategori != "" && !validKategori[*req.Kategori] {
		f["kategori"] = "geçersiz kategori"
	}
	if req.Durum != nil && *req.Durum != "" && !validDurum[*req.Durum] {
		f["durum"] = "geçersiz durum"
	}
	if req.TeslimYontemi != nil && *req.TeslimYontemi != "" && !validTeslim[*req.TeslimYontemi] {
		f["teslim_yontemi"] = "geçersiz teslim yöntemi"
	}
	if req.CevapGerekli && (req.CevapTarihi == nil || strings.TrimSpace(*req.CevapTarihi) == "") {
		f["cevap_tarihi"] = "cevap gerekliyse cevap tarihi zorunlu"
	}
	return f
}

func optUUID(s *string) (*uuid.UUID, bool) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*s))
	if err != nil {
		return nil, false
	}
	return &id, true
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// checkKesinKabul — yazışma tarihi ve cevap tarihi projenin Kesin Kabul
// tarihini aşamaz (proje bittikten sonraki bir tarihe yazışma/cevap termini
// koymak anlamsız). dateStr boşsa kontrol atlanır.
func checkKesinKabul(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, pid uuid.UUID, dateStr, fieldName string) bool {
	if dateStr == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{fieldName: "geçersiz tarih biçimi"})
		return false
	}
	if errs, err := kesinkabul.NotAfterKesinKabul(r.Context(), pool, pid, t, fieldName); err != nil {
		httpx.Internal(w, r)
		return false
	} else if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return false
	}
	return true
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
	ilgiliID, okID := optUUID(req.IlgiliYaziID)
	if !okID {
		httpx.ValidationFailed(w, r, map[string]string{"ilgili_yazi_id": "geçersiz UUID"})
		return
	}
	kategori := "Genel"
	if req.Kategori != nil && *req.Kategori != "" {
		kategori = *req.Kategori
	}
	durum := "Açık"
	if req.Durum != nil && *req.Durum != "" {
		durum = *req.Durum
	}
	teslimYontemi := "eposta"
	if req.TeslimYontemi != nil && *req.TeslimYontemi != "" {
		teslimYontemi = *req.TeslimYontemi
	}
	if !checkKesinKabul(w, r, h.pool, pid, req.Tarih, "tarih") {
		return
	}
	if !checkKesinKabul(w, r, h.pool, pid, strDeref(req.CevapTarihi), "cevap_tarihi") {
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	// Aynı proje+yön içinde eşzamanlı iki kayıt aynı numarayı almasın (PO
	// numaralandırmasıyla aynı desen).
	if _, err := tx.Exec(r.Context(),
		`SELECT pg_advisory_xact_lock(hashtext('evrak_no:' || $1::text || ':' || $2))`,
		pid, req.Direction); err != nil {
		httpx.Internal(w, r)
		return
	}
	prefix := "GE"
	if req.Direction == "giden" {
		prefix = "GİE"
	}
	var evrakNo string
	if err := tx.QueryRow(r.Context(), `
		SELECT $1 || '-' || lpad((count(*)+1)::text, 3, '0')
		FROM correspondences WHERE project_id=$2 AND direction=$3`,
		prefix, pid, req.Direction).Scan(&evrakNo); err != nil {
		httpx.Internal(w, r)
		return
	}

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO correspondences
			(project_id, direction, evrak_no, karsi_evrak_no, tarih, kurum_kisi, konu,
			 kategori, durum, cevap_gerekli, cevap_tarihi, ilgili_yazi_id, dagitim, notlar, teslim_yontemi, created_by)
		VALUES ($1,$2,$3,NULLIF(btrim(coalesce($4,'')),''),$5::date,$6,$7,
			$8,$9,$10,NULLIF($11,'')::date,$12,NULLIF(btrim(coalesce($13,'')),''),NULLIF(btrim(coalesce($14,'')),''),$15,$16)
		RETURNING id`,
		pid, req.Direction, evrakNo, req.KarsiEvrakNo, req.Tarih, strings.TrimSpace(req.KurumKisi), strings.TrimSpace(req.Konu),
		kategori, durum, req.CevapGerekli, strDeref(req.CevapTarihi), ilgiliID, req.Dagitim, req.Notlar, teslimYontemi, uid).Scan(&id)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "İlgili yazı bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"id": id, "evrak_no": evrakNo})
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
	var req upsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	ilgiliID, okID := optUUID(req.IlgiliYaziID)
	if !okID {
		httpx.ValidationFailed(w, r, map[string]string{"ilgili_yazi_id": "geçersiz UUID"})
		return
	}
	kategori := "Genel"
	if req.Kategori != nil && *req.Kategori != "" {
		kategori = *req.Kategori
	}
	durum := "Açık"
	if req.Durum != nil && *req.Durum != "" {
		durum = *req.Durum
	}
	teslimYontemi := "eposta"
	if req.TeslimYontemi != nil && *req.TeslimYontemi != "" {
		teslimYontemi = *req.TeslimYontemi
	}
	if !checkKesinKabul(w, r, h.pool, pid, req.Tarih, "tarih") {
		return
	}
	if !checkKesinKabul(w, r, h.pool, pid, strDeref(req.CevapTarihi), "cevap_tarihi") {
		return
	}

	var rowVersion int
	err := h.pool.QueryRow(r.Context(),
		`SELECT row_version FROM correspondences WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		id, pid).Scan(&rowVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Yazışma bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != rowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": rowVersion})
		return
	}

	ct, err := h.pool.Exec(r.Context(), `
		UPDATE correspondences SET
			karsi_evrak_no=NULLIF(btrim(coalesce($3,'')),''),
			tarih=$4::date,
			kurum_kisi=$5,
			konu=$6,
			kategori=$7,
			durum=$8,
			cevap_gerekli=$9,
			cevap_tarihi=NULLIF($10,'')::date,
			ilgili_yazi_id=$11,
			dagitim=NULLIF(btrim(coalesce($12,'')),''),
			notlar=NULLIF(btrim(coalesce($13,'')),''),
			teslim_yontemi=$14,
			row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		id, pid, req.KarsiEvrakNo, req.Tarih, strings.TrimSpace(req.KurumKisi), strings.TrimSpace(req.Konu),
		kategori, durum, req.CevapGerekli, strDeref(req.CevapTarihi), ilgiliID, req.Dagitim, req.Notlar, teslimYontemi)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "İlgili yazı bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Yazışma bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE correspondences SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Yazışma bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── Yardımcılar ───────────────────────────────────────────────────────────────

func requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, false
	}
	return uid, true
}

func isFKViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23503"
	}
	return false
}
