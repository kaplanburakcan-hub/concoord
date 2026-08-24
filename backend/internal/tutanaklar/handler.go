// Package tutanaklar — Saha Tutanakları (kaza/yangın/hırsızlık, ek imalat,
// mesai, yevmiyeli çalışma). Önceden tamamen tarayıcı localStorage'ındaydı;
// bu paket onu gerçek, projeler ve kullanıcılar arasında paylaşılan bir
// varlığa taşır. Fotoğraf ekleri mevcut polimorfik documents motoru
// üzerinden bağlanır (entity_type='saha_tutanagi', entity_id=tutanak.id) —
// ayrı bir dosya/blob tablosu açılmaz.
//
// Onay zinciri (onay_zinciri jsonb) önceden istemcide hesaplanıyordu; artık
// Submit/Decide uçlarında sunucuda hesaplanır — tek doğruluk kaynağı.
package tutanaklar

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
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/validate"
)

type Handler struct {
	pool   *pgxpool.Pool
	notify *notify.Service
}

func NewHandler(pool *pgxpool.Pool, notifySvc *notify.Service) *Handler {
	return &Handler{pool: pool, notify: notifySvc}
}

// TIP_HAKEDIS — hangi tutanak tipleri, tümü onaylandığında hakedişe
// (ilave iş olarak) akmaya adaydır. kaza/yangın/hırsızlık bir olay
// kaydıdır, maliyet kalemi değildir; zimmet de bir mal teslim kaydıdır,
// hakedişe akmaz.
var tipHakedis = map[string]bool{
	"kaza_yangin_hirsizlik": false,
	"ek_imalat":             true,
	"mesai":                 true,
	"yevmiyeli":             true,
	"zimmet":                false,
}

var validTip = map[string]bool{
	"kaza_yangin_hirsizlik": true, "ek_imalat": true, "mesai": true, "yevmiyeli": true, "zimmet": true,
}

type OnayAdim struct {
	Rol   string  `json:"rol"`
	Ad    string  `json:"ad"`
	Durum string  `json:"durum"` // bekliyor | onaylandi | reddedildi
	Tarih *string `json:"tarih,omitempty"`
	Not   *string `json:"not,omitempty"`
}

type Tutanak struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       uuid.UUID  `json:"project_id"`
	Tip             string     `json:"tip"`
	Baslik          string     `json:"baslik"`
	Tarih           string     `json:"tarih"`
	TaseronID       *uuid.UUID `json:"taseron_id,omitempty"`
	TaseronAdi      *string    `json:"taseron_adi,omitempty"`
	PersonelID      *uuid.UUID `json:"personel_id,omitempty"`
	PersonelAdSoyad *string    `json:"personel_ad_soyad,omitempty"`
	PersonelFirma   *string    `json:"personel_firma,omitempty"`
	Kisim           *string    `json:"kisim,omitempty"`
	Aciklama        string     `json:"aciklama"`
	Tutar           *float64   `json:"tutar,omitempty"`
	Birim           *string    `json:"birim,omitempty"`
	Miktar          *float64   `json:"miktar,omitempty"`
	Durum           string     `json:"durum"`
	OnayZinciri     []OnayAdim `json:"onay_zinciri"`
	HakediseEklendi bool       `json:"hakedise_eklendi"`
	CreatedByName   string     `json:"created_by_name"`
	CreatedAt       time.Time  `json:"created_at"`
	RowVersion      int        `json:"row_version"`
}

const listCols = `
	t.id, t.project_id, t.tip, t.baslik, to_char(t.tarih,'YYYY-MM-DD'),
	t.taseron_id, s.company_name, t.personel_id, p.ad_soyad, p.firma, t.kisim, t.aciklama, t.tutar, t.birim, t.miktar,
	t.durum, t.onay_zinciri::text, t.hakedise_eklendi, u.full_name, t.created_at, t.row_version`

func scanRow(row pgx.Row, t *Tutanak) error {
	var onayJSON string
	if err := row.Scan(&t.ID, &t.ProjectID, &t.Tip, &t.Baslik, &t.Tarih,
		&t.TaseronID, &t.TaseronAdi, &t.PersonelID, &t.PersonelAdSoyad, &t.PersonelFirma,
		&t.Kisim, &t.Aciklama, &t.Tutar, &t.Birim, &t.Miktar,
		&t.Durum, &onayJSON, &t.HakediseEklendi, &t.CreatedByName, &t.CreatedAt, &t.RowVersion); err != nil {
		return err
	}
	return json.Unmarshal([]byte(onayJSON), &t.OnayZinciri)
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

// ── List ──────────────────────────────────────────────────────────────────────

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	q := r.URL.Query()
	tip := strings.TrimSpace(q.Get("tip"))
	durum := strings.TrimSpace(q.Get("durum"))
	var taseronID *uuid.UUID
	if s := strings.TrimSpace(q.Get("taseron_id")); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			taseronID = &id
		}
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT `+listCols+`
		FROM saha_tutanaklari t
		JOIN users u ON u.id = t.created_by
		LEFT JOIN subcontractors s ON s.id = t.taseron_id
		LEFT JOIN project_personnel p ON p.id = t.personel_id
		WHERE t.project_id=$1 AND t.deleted_at IS NULL
		  AND ($2='' OR t.tip=$2)
		  AND ($3='' OR t.durum=$3)
		  AND ($4::uuid IS NULL OR t.taseron_id=$4::uuid)
		ORDER BY t.tarih DESC, t.created_at DESC`,
		pid, tip, durum, taseronID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Tutanak{}
	for rows.Next() {
		var t Tutanak
		if err := scanRow(rows, &t); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, t)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"tutanaklar": out})
}

// ── Create ────────────────────────────────────────────────────────────────────

type createReq struct {
	Tip          string   `json:"tip"`
	Baslik       string   `json:"baslik"`
	Tarih        string   `json:"tarih"`
	TaseronID    *string  `json:"taseron_id"`
	PersonelID   *string  `json:"personel_id"`
	Kisim        *string  `json:"kisim"`
	Aciklama     string   `json:"aciklama"`
	Tutar        *float64 `json:"tutar"`
	Birim        *string  `json:"birim"`
	Miktar       *float64 `json:"miktar"`
	KisimSefiVar bool     `json:"kisim_sefi_var"`
}

// onayZinciriOlustur — ön yüzdeki eski localStorage sürümüyle birebir aynı
// kural: kısım şefi onayı isteğe bağlı, şantiye şefi + proje müdürü her
// zaman zincirde.
func onayZinciriOlustur(kisimSefiVar bool) []OnayAdim {
	var zincir []OnayAdim
	if kisimSefiVar {
		zincir = append(zincir, OnayAdim{Rol: "Kısım Şefi", Durum: "bekliyor"})
	}
	zincir = append(zincir, OnayAdim{Rol: "Şantiye Şefi", Durum: "bekliyor"})
	zincir = append(zincir, OnayAdim{Rol: "Proje Müdürü", Durum: "bekliyor"})
	return zincir
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
	f := map[string]string{}
	if !validTip[req.Tip] {
		f["tip"] = "geçersiz tutanak tipi"
	}
	if strings.TrimSpace(req.Baslik) == "" {
		f["baslik"] = "başlık zorunlu"
	}
	if strings.TrimSpace(req.Aciklama) == "" {
		f["aciklama"] = "açıklama zorunlu"
	}
	var tarih time.Time
	if t, err := time.Parse("2006-01-02", req.Tarih); err != nil {
		f["tarih"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	} else {
		tarih = t
	}
	if len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	if errs, err := validate.NotAfterKesinKabul(r.Context(), h.pool, pid, tarih, "tarih"); err != nil {
		httpx.Internal(w, r)
		return
	} else if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	var taseronID *uuid.UUID
	if req.TaseronID != nil && strings.TrimSpace(*req.TaseronID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.TaseronID))
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"taseron_id": "geçersiz UUID"})
			return
		}
		taseronID = &id
	}
	var personelID *uuid.UUID
	var personelAdSoyad, personelFirma string
	if req.PersonelID != nil && strings.TrimSpace(*req.PersonelID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.PersonelID))
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"personel_id": "geçersiz UUID"})
			return
		}
		var firma *string
		if err := h.pool.QueryRow(r.Context(),
			`SELECT ad_soyad, firma FROM project_personnel WHERE id=$1 AND project_id=$2`,
			id, pid).Scan(&personelAdSoyad, &firma); err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"personel_id": "personel bulunamadı"})
			return
		}
		if firma != nil {
			personelFirma = *firma
		}
		personelID = &id
	}
	zincir, _ := json.Marshal(onayZinciriOlustur(req.KisimSefiVar))

	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO saha_tutanaklari
			(project_id, tip, baslik, tarih, taseron_id, personel_id, kisim, aciklama, tutar, birim, miktar,
			 durum, onay_zinciri, created_by)
		VALUES ($1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10,$11,'taslak',$12::jsonb,$13)
		RETURNING id`,
		pid, req.Tip, strings.TrimSpace(req.Baslik), req.Tarih, taseronID, personelID, req.Kisim,
		strings.TrimSpace(req.Aciklama), req.Tutar, req.Birim, req.Miktar, zincir, uid,
	).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}

	if req.Tip == "zimmet" && personelFirma != "" {
		h.notifyZimmetFirma(r.Context(), pid, id, req.Baslik, personelAdSoyad, personelFirma)
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

// notifyZimmetFirma — personelin firma alanını (serbest metin) subcontractors
// tablosuna ada göre eşleştirir (Sözleşme Takip'teki poz_no eşleşmesiyle aynı
// ruhta: gerçek bir FK değil, isimden yumuşak eşleştirme), o firmanın
// project_members'taki (subcontractor_id dolu) kullanıcılarına bildirim
// gönderir. Eşleşme yoksa sessizce atlanır — hata değildir.
func (h *Handler) notifyZimmetFirma(ctx context.Context, pid, tutanakID uuid.UUID, baslik, personelAdSoyad, firma string) {
	rows, err := h.pool.Query(ctx, `
		SELECT DISTINCT pm.user_id
		FROM subcontractors s
		JOIN project_members pm ON pm.subcontractor_id = s.id AND pm.project_id = s.project_id AND pm.deleted_at IS NULL
		WHERE s.project_id = $1 AND s.deleted_at IS NULL AND lower(s.company_name) = lower($2)`,
		pid, firma)
	if err != nil {
		return
	}
	defer rows.Close()
	var userIDs []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if rows.Scan(&uid) == nil {
			userIDs = append(userIDs, uid)
		}
	}
	if len(userIDs) == 0 {
		return
	}
	h.notify.Send(ctx, notify.Input{
		UserIDs:    userIDs,
		Type:       notify.TypeZimmetCreated,
		Title:      "Yeni Zimmet Tutanağı",
		Body:       fmt.Sprintf("%s adına \"%s\" zimmet tutanağı oluşturuldu.", personelAdSoyad, baslik),
		EntityType: "saha_tutanagi",
		EntityID:   &tutanakID,
		ProjectID:  &pid,
	})
}

// ── Submit (taslak → onay_sureci) ───────────────────────────────────────────

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE saha_tutanaklari SET durum='onay_sureci', row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND durum='taslak'`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Tutanak bulunamadı ya da zaten onay sürecinde/karara bağlanmış.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "onay_sureci"})
}

// ── Decide (bekleyen adımı onayla/reddet) ───────────────────────────────────

type decideReq struct {
	Karar string  `json:"karar"` // onaylandi | reddedildi
	Not   *string `json:"not"`
}

func (h *Handler) Decide(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req decideReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Karar != "onaylandi" && req.Karar != "reddedildi" {
		httpx.ValidationFailed(w, r, map[string]string{"karar": "onaylandi veya reddedildi olmalı"})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var tip, onayJSON string
	if err := tx.QueryRow(r.Context(), `
		SELECT tip, onay_zinciri::text FROM saha_tutanaklari
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND durum='onay_sureci' FOR UPDATE`,
		id, pid).Scan(&tip, &onayJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound,
				"Tutanak bulunamadı ya da onay sürecinde değil.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	var zincir []OnayAdim
	if err := json.Unmarshal([]byte(onayJSON), &zincir); err != nil {
		httpx.Internal(w, r)
		return
	}
	pending := -1
	for i, a := range zincir {
		if a.Durum == "bekliyor" {
			pending = i
			break
		}
	}
	if pending == -1 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bekleyen onay adımı yok.", nil)
		return
	}
	now := time.Now().Format(time.RFC3339)
	zincir[pending].Durum = req.Karar
	zincir[pending].Tarih = &now
	if req.Not != nil && strings.TrimSpace(*req.Not) != "" {
		zincir[pending].Not = req.Not
	}

	hepsiOnaylandi, biriReddetti := true, false
	for _, a := range zincir {
		if a.Durum == "reddedildi" {
			biriReddetti = true
		}
		if a.Durum != "onaylandi" {
			hepsiOnaylandi = false
		}
	}
	yeniDurum := "onay_sureci"
	if biriReddetti {
		yeniDurum = "reddedildi"
	} else if hepsiOnaylandi {
		yeniDurum = "onaylandi"
	}
	hakediseEklendi := hepsiOnaylandi && tipHakedis[tip]

	zincirJSON, _ := json.Marshal(zincir)
	if _, err := tx.Exec(r.Context(), `
		UPDATE saha_tutanaklari SET
			onay_zinciri=$3::jsonb, durum=$4, hakedise_eklendi=$5, row_version=row_version+1
		WHERE id=$1 AND project_id=$2`,
		id, pid, zincirJSON, yeniDurum, hakediseEklendi); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": yeniDurum, "hakedise_eklendi": hakediseEklendi})
}

// ── Delete (yalnızca taslak) ─────────────────────────────────────────────────

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
		UPDATE saha_tutanaklari SET deleted_at=now()
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND durum='taslak'`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Tutanak bulunamadı ya da yalnızca taslak durumundaki tutanaklar silinebilir.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
