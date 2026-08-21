// Package contracts — Ana Sözleşme (project_main_contracts) CRUD.
// Her proje için tek kayıt; GET yükler, PUT upsert yapar.
package contracts

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/validate"
)

type Handler struct{ db *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{db: pool} }

// BirimFiyatKalem — birim fiyatlı / karma sözleşme iş kalemi.
type BirimFiyatKalem struct {
	ID         string  `json:"id"`
	Tanim      string  `json:"tanim"`
	Birim      string  `json:"birim"`
	Miktar     float64 `json:"miktar"`
	BirimFiyat float64 `json:"birim_fiyat"`
	ParaBirimi string  `json:"para_birimi"`
}

type Contract struct {
	ID                    string            `json:"id,omitempty"`
	ProjectID             string            `json:"project_id"`
	SozlesmeTuru          string            `json:"sozlesme_turu"`
	FiyatFarkiVar         bool              `json:"fiyat_farki_var"`
	FiyatFarkiFormulu     string            `json:"fiyat_farki_formulu"`
	SozlesmeBedeli        *float64          `json:"sozlesme_bedeli"`
	SozlesmeParaBirimi    string            `json:"sozlesme_para_birimi"`
	BirimFiyatKalemleri   []BirimFiyatKalem `json:"birim_fiyat_kalemleri"`
	SozlesmeTarihi        *string           `json:"sozlesme_tarihi"`
	YerTeslimTarihi       *string           `json:"yer_teslim_tarihi"`
	IsSuresiGun           *int              `json:"is_suresi_gun"`
	GeciciKabulSonrasiGun *int              `json:"gecici_kabul_sonrasi_gun"`
	MaxArtisOrani         *float64          `json:"max_artis_orani"`
	MaxEksilisOrani       *float64          `json:"max_eksilis_orani"`
	SgkIsYeriNo           string            `json:"sgk_is_yeri_no"`
	PdfDosyaURL           string            `json:"pdf_dosya_url"`
	PdfDosyaAdi           string            `json:"pdf_dosya_adi"`
	UpdatedAt             *string           `json:"updated_at,omitempty"`
	IsLocked              bool              `json:"is_locked"`
	UpdatedByName         string            `json:"updated_by_name,omitempty"`
}

// Get — proje ana sözleşmesini döner. Henüz kaydedilmemişse 404.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}

	var c Contract
	var kalemleriJSON []byte
	err := h.db.QueryRow(r.Context(), `
		SELECT c.id, c.project_id,
		       c.sozlesme_turu, c.fiyat_farki_var, COALESCE(c.fiyat_farki_formulu,''),
		       c.sozlesme_bedeli, COALESCE(c.sozlesme_para_birimi,'TRY'),
		       c.birim_fiyat_kalemleri,
		       TO_CHAR(c.sozlesme_tarihi,'YYYY-MM-DD'),
		       TO_CHAR(c.yer_teslim_tarihi,'YYYY-MM-DD'),
		       c.is_suresi_gun, c.gecici_kabul_sonrasi_gun,
		       c.max_artis_orani, c.max_eksilis_orani,
		       COALESCE(c.sgk_is_yeri_no,''),
		       COALESCE(c.pdf_dosya_url,''), COALESCE(c.pdf_dosya_adi,''),
		       TO_CHAR(c.updated_at,'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		       c.is_locked, COALESCE(u.full_name,'')
		FROM project_main_contracts c
		LEFT JOIN users u ON u.id = c.updated_by
		WHERE c.project_id=$1`, pid).
		Scan(&c.ID, &c.ProjectID,
			&c.SozlesmeTuru, &c.FiyatFarkiVar, &c.FiyatFarkiFormulu,
			&c.SozlesmeBedeli, &c.SozlesmeParaBirimi,
			&kalemleriJSON,
			&c.SozlesmeTarihi, &c.YerTeslimTarihi,
			&c.IsSuresiGun, &c.GeciciKabulSonrasiGun,
			&c.MaxArtisOrani, &c.MaxEksilisOrani,
			&c.SgkIsYeriNo,
			&c.PdfDosyaURL, &c.PdfDosyaAdi,
			&c.UpdatedAt,
			&c.IsLocked, &c.UpdatedByName)

	if err == pgx.ErrNoRows {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Ana sözleşme henüz girilmemiş.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	if err := json.Unmarshal(kalemleriJSON, &c.BirimFiyatKalemleri); err != nil {
		c.BirimFiyatKalemleri = []BirimFiyatKalem{}
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"contract": c})
}

// Upsert — proje ana sözleşmesini kaydeder (INSERT … ON CONFLICT UPDATE).
// Her başarılı kayıt sözleşmeyi otomatik kilitler (is_locked=true) — ayrı
// bir "kilitle" adımı yok, "Sözleşmeyi Kaydet ve Kilitle" tek eylem.
// Şimdilik revizyonlar da doğrudan kilitlenir; onay hiyerarşisine bağlama
// (revize → onaya gönder → onaylanınca kilitle + bildirim) ileride ayrı
// bir iş olarak ele alınacak.
func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}

	var body Contract
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}

	// Doğrulama
	valid := map[string]string{}
	if body.SozlesmeTuru != "birim_fiyat" && body.SozlesmeTuru != "goturu_bedel" && body.SozlesmeTuru != "karma" {
		valid["sozlesme_turu"] = "Geçerli değer: birim_fiyat, goturu_bedel, karma"
	}
	if len(valid) > 0 {
		httpx.ValidationFailed(w, r, valid)
		return
	}

	kalemleriJSON, err := json.Marshal(body.BirimFiyatKalemleri)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	var id string
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO project_main_contracts (
		    project_id, sozlesme_turu, fiyat_farki_var, fiyat_farki_formulu,
		    sozlesme_bedeli, sozlesme_para_birimi, birim_fiyat_kalemleri,
		    sozlesme_tarihi, yer_teslim_tarihi,
		    is_suresi_gun, gecici_kabul_sonrasi_gun,
		    max_artis_orani, max_eksilis_orani,
		    sgk_is_yeri_no, pdf_dosya_url, pdf_dosya_adi,
		    created_by, updated_by, is_locked, updated_at
		) VALUES (
		    $1,$2,$3,$4,$5,$6,$7,
		    $8::date,$9::date,
		    $10,$11,$12,$13,$14,$15,$16,
		    $17,$17,TRUE,NOW()
		)
		ON CONFLICT (project_id) DO UPDATE SET
		    sozlesme_turu            = EXCLUDED.sozlesme_turu,
		    fiyat_farki_var          = EXCLUDED.fiyat_farki_var,
		    fiyat_farki_formulu      = EXCLUDED.fiyat_farki_formulu,
		    sozlesme_bedeli          = EXCLUDED.sozlesme_bedeli,
		    sozlesme_para_birimi     = EXCLUDED.sozlesme_para_birimi,
		    birim_fiyat_kalemleri    = EXCLUDED.birim_fiyat_kalemleri,
		    sozlesme_tarihi          = EXCLUDED.sozlesme_tarihi,
		    yer_teslim_tarihi        = EXCLUDED.yer_teslim_tarihi,
		    is_suresi_gun            = EXCLUDED.is_suresi_gun,
		    gecici_kabul_sonrasi_gun = EXCLUDED.gecici_kabul_sonrasi_gun,
		    max_artis_orani          = EXCLUDED.max_artis_orani,
		    max_eksilis_orani        = EXCLUDED.max_eksilis_orani,
		    sgk_is_yeri_no           = EXCLUDED.sgk_is_yeri_no,
		    pdf_dosya_url            = EXCLUDED.pdf_dosya_url,
		    pdf_dosya_adi            = EXCLUDED.pdf_dosya_adi,
		    updated_by               = EXCLUDED.updated_by,
		    is_locked                = TRUE,
		    updated_at               = NOW()
		RETURNING id`,
		pid,
		body.SozlesmeTuru, body.FiyatFarkiVar, nilStr(body.FiyatFarkiFormulu),
		body.SozlesmeBedeli, coalesce(body.SozlesmeParaBirimi, "TRY"), kalemleriJSON,
		nilStr(derefStr(body.SozlesmeTarihi)), nilStr(derefStr(body.YerTeslimTarihi)),
		body.IsSuresiGun, body.GeciciKabulSonrasiGun,
		body.MaxArtisOrani, body.MaxEksilisOrani,
		nilStr(body.SgkIsYeriNo), nilStr(body.PdfDosyaURL), nilStr(body.PdfDosyaAdi),
		uid,
	).Scan(&id)

	if err != nil {
		httpx.Internal(w, r)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"id": id})
}

// KesinKabulTarihi — projenin Kesin Kabul tarihini döner (Yer Teslim +
// İşin Süresi + Geçici Kabul Sonrası gün). Ana sözleşme eksikse/yoksa
// kesin_kabul_tarihi=null döner (400/404 değil — çağıran taraf henüz
// hesaplanamadığını anlar, formunda sınır uygulamaz). İş/teslimat tarihi
// giren tüm formlar (hakediş, tutanak, toplantı, milestone, görev, puantaj,
// rapor, depo, İSG bulgusu, yazışma, satınalma) bu ucu kullanarak date
// input'larına max koyar — bkz. internal/validate.NotAfterKesinKabul
// (sunucu tarafı asıl uygulama noktası, bu sadece istemci için ipucu).
func (h *Handler) KesinKabulTarihi(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	kk, err := validate.KesinKabulTarihi(r.Context(), h.db, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	out := map[string]any{"kesin_kabul_tarihi": nil}
	if kk != nil {
		out["kesin_kabul_tarihi"] = kk.Format("2006-01-02")
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ---- yardımcılar ----

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID formatı.", nil)
		return uuid.Nil, false
	}
	return id, true
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func coalesce(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
