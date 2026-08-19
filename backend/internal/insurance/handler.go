// Package insurance — Sigorta ve Poliçeler: proje bazlı sigorta poliçesi
// takibi (İnşaat All Risk/CAR-EAR, İşveren Mali Sorumluluk, Üçüncü Şahıs
// Mali Sorumluluk, Nakliyat, Diğer). Onay süreci yok — Ana Sözleşme'deki
// gibi doğrudan giriş; PDF eki de Ana Sözleşme'deki gibi ad/url alanı
// olarak tutulur (polimorfik documents motoruna bağlanmaz).
package insurance

import (
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

var validTurler = map[string]bool{
	"car_ear": true, "isveren_mali_sorumluluk": true,
	"ucuncu_sahis_mali_sorumluluk": true, "nakliyat": true, "diger": true,
}
var validDurumlar = map[string]bool{"aktif": true, "suresi_doldu": true, "iptal": true}

type Policy struct {
	ID                uuid.UUID `json:"id"`
	ProjectID         uuid.UUID `json:"project_id"`
	PoliceTuru        string    `json:"police_turu"`
	SigortaSirketi    string    `json:"sigorta_sirketi"`
	PoliceNo          string    `json:"police_no"`
	BaslangicTarihi   *string   `json:"baslangic_tarihi,omitempty"`
	BitisTarihi       *string   `json:"bitis_tarihi,omitempty"`
	TeminatBedeli     *float64  `json:"teminat_bedeli,omitempty"`
	TeminatParaBirimi string    `json:"teminat_para_birimi"`
	PrimTutari        *float64  `json:"prim_tutari,omitempty"`
	PrimParaBirimi    string    `json:"prim_para_birimi"`
	Durum             string    `json:"durum"`
	Aciklama          string    `json:"aciklama"`
	PdfDosyaURL       string    `json:"pdf_dosya_url"`
	PdfDosyaAdi       string    `json:"pdf_dosya_adi"`
	CreatedByName     string    `json:"created_by_name"`
	CreatedAt         time.Time `json:"created_at"`
	RowVersion        int       `json:"row_version"`
}

const listCols = `
	p.id, p.project_id, p.police_turu, p.sigorta_sirketi, p.police_no,
	to_char(p.baslangic_tarihi,'YYYY-MM-DD'), to_char(p.bitis_tarihi,'YYYY-MM-DD'),
	p.teminat_bedeli::float8, p.teminat_para_birimi,
	p.prim_tutari::float8, p.prim_para_birimi,
	p.durum, COALESCE(p.aciklama,''),
	COALESCE(p.pdf_dosya_url,''), COALESCE(p.pdf_dosya_adi,''),
	COALESCE(u.full_name,''), p.created_at, p.row_version`

func scanRow(row pgx.Row, p *Policy) error {
	return row.Scan(&p.ID, &p.ProjectID, &p.PoliceTuru, &p.SigortaSirketi, &p.PoliceNo,
		&p.BaslangicTarihi, &p.BitisTarihi,
		&p.TeminatBedeli, &p.TeminatParaBirimi,
		&p.PrimTutari, &p.PrimParaBirimi,
		&p.Durum, &p.Aciklama,
		&p.PdfDosyaURL, &p.PdfDosyaAdi,
		&p.CreatedByName, &p.CreatedAt, &p.RowVersion)
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
		FROM insurance_policies p
		JOIN users u ON u.id = p.created_by
		WHERE p.project_id=$1 AND p.deleted_at IS NULL
		ORDER BY p.bitis_tarihi ASC NULLS LAST, p.created_at DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Policy{}
	for rows.Next() {
		var p Policy
		if err := scanRow(rows, &p); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"policies": out})
}

// ── Create / Update ordak gövde ─────────────────────────────────────────────

type reqBody struct {
	PoliceTuru        string   `json:"police_turu"`
	SigortaSirketi    string   `json:"sigorta_sirketi"`
	PoliceNo          string   `json:"police_no"`
	BaslangicTarihi   *string  `json:"baslangic_tarihi"`
	BitisTarihi       *string  `json:"bitis_tarihi"`
	TeminatBedeli     *float64 `json:"teminat_bedeli"`
	TeminatParaBirimi string   `json:"teminat_para_birimi"`
	PrimTutari        *float64 `json:"prim_tutari"`
	PrimParaBirimi    string   `json:"prim_para_birimi"`
	Durum             string   `json:"durum"`
	Aciklama          string   `json:"aciklama"`
	PdfDosyaURL       string   `json:"pdf_dosya_url"`
	PdfDosyaAdi       string   `json:"pdf_dosya_adi"`
	RowVersion        int      `json:"row_version"`
}

func validate(b reqBody) map[string]string {
	f := map[string]string{}
	if !validTurler[b.PoliceTuru] {
		f["police_turu"] = "Geçerli bir poliçe türü seçin."
	}
	if strings.TrimSpace(b.SigortaSirketi) == "" {
		f["sigorta_sirketi"] = "Sigorta şirketi zorunlu."
	}
	if strings.TrimSpace(b.PoliceNo) == "" {
		f["police_no"] = "Poliçe no zorunlu."
	}
	if b.Durum != "" && !validDurumlar[b.Durum] {
		f["durum"] = "Geçerli bir durum seçin."
	}
	if s := strDeref(b.BaslangicTarihi); s != "" {
		if _, err := time.Parse("2006-01-02", s); err != nil {
			f["baslangic_tarihi"] = "Geçerli bir tarih girin."
		}
	}
	if s := strDeref(b.BitisTarihi); s != "" {
		if _, err := time.Parse("2006-01-02", s); err != nil {
			f["bitis_tarihi"] = "Geçerli bir tarih girin."
		}
	}
	if b.TeminatBedeli != nil && *b.TeminatBedeli < 0 {
		f["teminat_bedeli"] = "0 veya daha büyük olmalı."
	}
	if b.PrimTutari != nil && *b.PrimTutari < 0 {
		f["prim_tutari"] = "0 veya daha büyük olmalı."
	}
	return f
}

// ── Create ───────────────────────────────────────────────────────────────

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var body reqBody
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if f := validate(body); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	durum := body.Durum
	if durum == "" {
		durum = "aktif"
	}

	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO insurance_policies
			(project_id, police_turu, sigorta_sirketi, police_no,
			 baslangic_tarihi, bitis_tarihi,
			 teminat_bedeli, teminat_para_birimi, prim_tutari, prim_para_birimi,
			 durum, aciklama, pdf_dosya_url, pdf_dosya_adi, created_by)
		VALUES ($1,$2,$3,$4,
		        NULLIF($5,'')::date, NULLIF($6,'')::date,
		        $7, COALESCE(NULLIF($8,''),'TRY'), $9, COALESCE(NULLIF($10,''),'TRY'),
		        $11,$12,$13,$14,$15)
		RETURNING id`,
		pid, body.PoliceTuru, strings.TrimSpace(body.SigortaSirketi), strings.TrimSpace(body.PoliceNo),
		strDeref(body.BaslangicTarihi), strDeref(body.BitisTarihi),
		body.TeminatBedeli, body.TeminatParaBirimi, body.PrimTutari, body.PrimParaBirimi,
		durum, body.Aciklama, body.PdfDosyaURL, body.PdfDosyaAdi, uid,
	).Scan(&id); err != nil {
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
	var body reqBody
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if f := validate(body); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	durum := body.Durum
	if durum == "" {
		durum = "aktif"
	}

	ct, err := h.pool.Exec(r.Context(), `
		UPDATE insurance_policies SET
			police_turu=$3, sigorta_sirketi=$4, police_no=$5,
			baslangic_tarihi=NULLIF($6,'')::date, bitis_tarihi=NULLIF($7,'')::date,
			teminat_bedeli=$8, teminat_para_birimi=COALESCE(NULLIF($9,''),'TRY'),
			prim_tutari=$10, prim_para_birimi=COALESCE(NULLIF($11,''),'TRY'),
			durum=$12, aciklama=$13, pdf_dosya_url=$14, pdf_dosya_adi=$15,
			updated_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND row_version=$16`,
		id, pid, body.PoliceTuru, strings.TrimSpace(body.SigortaSirketi), strings.TrimSpace(body.PoliceNo),
		strDeref(body.BaslangicTarihi), strDeref(body.BitisTarihi),
		body.TeminatBedeli, body.TeminatParaBirimi, body.PrimTutari, body.PrimParaBirimi,
		durum, body.Aciklama, body.PdfDosyaURL, body.PdfDosyaAdi, body.RowVersion)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt bulunamadı ya da başka biri tarafından güncellenmiş (sayfayı yenileyin).", nil)
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
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE insurance_policies SET deleted_at=now()
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kayıt bulunamadı.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── Yardımcılar ──────────────────────────────────────────────────────────

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
