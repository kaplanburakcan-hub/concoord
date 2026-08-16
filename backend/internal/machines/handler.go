package machines

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/equipmenttransfers"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
)

type Handler struct {
	pool *pgxpool.Pool
	nt   *notify.Service
}

func NewHandler(pool *pgxpool.Pool, nt *notify.Service) *Handler { return &Handler{pool: pool, nt: nt} }

// ── DTOs ──────────────────────────────────────────────────────────────────────

// Machine — bir projeye ait aktif makine ATAMASINI temsil eder (id =
// project_machines.id). Kimlik/mülkiyet/durum alanları artık
// company_equipment'ten JOIN ile gelir (bkz. migration 000047/000048);
// CompanyEquipmentID ile envanterdeki kanonik kayda işaret eder.
type Machine struct {
	ID                 uuid.UUID `json:"id"`
	ProjectID          uuid.UUID `json:"project_id"`
	CompanyEquipmentID uuid.UUID `json:"company_equipment_id"`
	Tip                string    `json:"tip"`
	Ad                 string    `json:"ad"`
	Plaka              *string   `json:"plaka,omitempty"`
	Marka              *string   `json:"marka,omitempty"`
	Model              *string   `json:"model,omitempty"`
	SeriNo             *string   `json:"seri_no,omitempty"`
	DemirbasNo         *string   `json:"demirbas_no,omitempty"`
	UretimYili         *int      `json:"uretim_yili,omitempty"`
	Sahiplik           string    `json:"sahiplik"`
	Tedarikci          *string   `json:"tedarikci,omitempty"`
	GunlukUcret        *float64  `json:"gunluk_ucret,omitempty"`
	Durum              string    `json:"durum"`
	SonBakimTarihi     *string   `json:"son_bakim_tarihi,omitempty"`
	SonrakiBakimTarihi *string   `json:"sonraki_bakim_tarihi,omitempty"`
	Aciklama           *string   `json:"aciklama,omitempty"`
	AtanmaTarihi       string    `json:"atanma_tarihi"`
	IsBasiTarihi       *string   `json:"is_basi_tarihi,omitempty"`
	// FromTransferID doluysa bu atama onaylı bir proje-arası transferden
	// geldi; TeslimAlindiTarihi boşken frontend "Teslim Alındı" butonunu
	// gösterir (bkz. MarkDelivered).
	FromTransferID     *uuid.UUID `json:"from_transfer_id,omitempty"`
	TeslimAlindiTarihi *string    `json:"teslim_alindi_tarihi,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// machineCols/machineJoin — ListMachines/CreateMachine/UpdateMachine'in
// hepsi aynı şekli döndürür; tekrarı önlemek için tek yerde tanımlı.
const machineSelectCols = `
	pm.id, pm.project_id, ce.id, ce.tip, ce.ad, ce.plaka, ce.marka, ce.model,
	ce.seri_no, ce.demirbas_no, ce.uretim_yili, ce.sahiplik, ce.tedarikci,
	ce.gunluk_ucret::float8, ce.durum,
	TO_CHAR(ce.son_bakim_tarihi,'YYYY-MM-DD'),
	TO_CHAR(ce.sonraki_bakim_tarihi,'YYYY-MM-DD'),
	ce.aciklama, TO_CHAR(pm.atanma_tarihi,'YYYY-MM-DD'),
	TO_CHAR(pm.is_basi_tarihi,'YYYY-MM-DD'),
	pm.from_transfer_id, TO_CHAR(pm.teslim_alindi_tarihi,'YYYY-MM-DD'),
	pm.created_at, pm.updated_at`

func scanMachine(row interface{ Scan(...any) error }, m *Machine) error {
	return row.Scan(&m.ID, &m.ProjectID, &m.CompanyEquipmentID, &m.Tip, &m.Ad, &m.Plaka,
		&m.Marka, &m.Model, &m.SeriNo, &m.DemirbasNo, &m.UretimYili, &m.Sahiplik,
		&m.Tedarikci, &m.GunlukUcret, &m.Durum, &m.SonBakimTarihi, &m.SonrakiBakimTarihi,
		&m.Aciklama, &m.AtanmaTarihi, &m.IsBasiTarihi, &m.FromTransferID, &m.TeslimAlindiTarihi,
		&m.CreatedAt, &m.UpdatedAt)
}

type MachineLog struct {
	ID             uuid.UUID `json:"id"`
	ProjectID      uuid.UUID `json:"project_id"`
	MachineID      uuid.UUID `json:"machine_id"`
	MachineAd      string    `json:"machine_ad"`
	Tarih          string    `json:"tarih"`
	CalismaMiktari float64   `json:"calisma_miktari"`
	CalismaBirimi  string    `json:"calisma_birimi"`
	Operator       *string   `json:"operator,omitempty"`
	Aciklama       *string   `json:"aciklama,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

// ── Machines ──────────────────────────────────────────────────────────────────

func (h *Handler) ListMachines(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	tip := r.URL.Query().Get("tip")
	query := `SELECT ` + machineSelectCols + `
		FROM project_machines pm
		JOIN company_equipment ce ON ce.id = pm.company_equipment_id
		WHERE pm.project_id=$1 AND pm.ayrilma_tarihi IS NULL`
	args := []any{pid}
	if tip != "" {
		query += ` AND ce.tip=$2 ORDER BY ce.ad`
		args = append(args, tip)
	} else {
		query += ` ORDER BY ce.tip, ce.ad`
	}
	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Machine{}
	for rows.Next() {
		var m Machine
		if err := scanMachine(rows, &m); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, m)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"machines": out})
}

type machineBody struct {
	Tip                string   `json:"tip"`
	Ad                 string   `json:"ad"`
	Plaka              *string  `json:"plaka"`
	Marka              *string  `json:"marka"`
	Model              *string  `json:"model"`
	SeriNo             *string  `json:"seri_no"`
	DemirbasNo         *string  `json:"demirbas_no"`
	UretimYili         *int     `json:"uretim_yili"`
	Sahiplik           string   `json:"sahiplik"`
	Tedarikci          *string  `json:"tedarikci"`
	GunlukUcret        *float64 `json:"gunluk_ucret"`
	Durum              string   `json:"durum"`
	SonBakimTarihi     *string  `json:"son_bakim_tarihi"`
	SonrakiBakimTarihi *string  `json:"sonraki_bakim_tarihi"`
	Aciklama           *string  `json:"aciklama"`
	// KayitYeri — "proje" (varsayılan, mevcut davranış: envanter kaydı +
	// bu projeye atama birlikte oluşturulur) veya "firma_envanteri"
	// (sadece envanter kaydı oluşturulur, hiçbir projeye atanmaz —
	// bkz. Faz B: daha sonra bir projeye transfer edilebilir).
	KayitYeri string `json:"kayit_yeri"`
}

// findMatchingEquipment — tipe göre eşleştirme anahtarıyla (araç→plaka,
// iş makinesi→seri no, ekipman→seri no ya da demirbaş no) envanterde
// zaten var olan bir kayıt arar. Anahtar boşsa (örn. plaka girilmedi)
// eşleştirme yapılamaz — nil döner, YENİ kayıt açılır.
func (h *Handler) findMatchingEquipment(ctx context.Context, tx pgx.Tx, tip string, plaka, seriNo, demirbasNo *string) (equipmentID uuid.UUID, currentProjectID *uuid.UUID, found bool) {
	var query string
	var arg *string
	switch tip {
	case "arac":
		if plaka == nil || *plaka == "" {
			return
		}
		query, arg = `SELECT id, current_project_id FROM company_equipment WHERE tip='arac' AND plaka=$1 LIMIT 1`, plaka
	case "is_makinesi":
		if seriNo == nil || *seriNo == "" {
			return
		}
		query, arg = `SELECT id, current_project_id FROM company_equipment WHERE tip='is_makinesi' AND seri_no=$1 LIMIT 1`, seriNo
	case "ekipman":
		if seriNo != nil && *seriNo != "" {
			query, arg = `SELECT id, current_project_id FROM company_equipment WHERE tip='ekipman' AND seri_no=$1 LIMIT 1`, seriNo
		} else if demirbasNo != nil && *demirbasNo != "" {
			query, arg = `SELECT id, current_project_id FROM company_equipment WHERE tip='ekipman' AND demirbas_no=$1 LIMIT 1`, demirbasNo
		} else {
			return
		}
	default:
		return
	}
	if err := tx.QueryRow(ctx, query, arg).Scan(&equipmentID, &currentProjectID); err != nil {
		return uuid.Nil, nil, false
	}
	return equipmentID, currentProjectID, true
}

// CreateMachine — "Proje" modunda (varsayılan) hem company_equipment hem
// project_machines ataması tek transaction'da oluşturulur. "Firma
// Envanteri" modunda sadece company_equipment satırı oluşturulur; bu
// durumda oluşan kayıt bu projenin listesinde GÖRÜNMEZ (hiçbir projeye
// atanmadı) — çağıran taraf CreatedMachine yerine boş bir onay bekler.
//
// Her iki modda da önce envanterde eşleşme aranır (bkz.
// findMatchingEquipment): eşleşme yoksa ya da eşleşen kayıt henüz
// hiçbir projeye atanmamışsa/zaten bu projedeyse, mevcut kayıt
// kullanılır/güncellenir (yeni company_equipment açılmaz). Eşleşen
// kayıt BAŞKA bir projede aktifse 409 döner — istemci bunu "transfer
// talebi oluştur" uyarısına çevirir (bkz. equipmenttransfers paketi).
func (h *Handler) CreateMachine(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var b machineBody
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Ad == "" {
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "ad zorunludur.", nil)
		return
	}
	if b.Tip == "" {
		b.Tip = "arac"
	}
	if b.Sahiplik == "" {
		b.Sahiplik = "ozmal"
	}
	if b.Durum == "" {
		b.Durum = "aktif"
	}
	if b.KayitYeri == "" {
		b.KayitYeri = "proje"
	}

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(ctx)

	matchID, matchProjectID, matched := h.findMatchingEquipment(ctx, tx, b.Tip, b.Plaka, b.SeriNo, b.DemirbasNo)

	if matched && matchProjectID != nil && *matchProjectID != pid {
		var projectName string
		_ = tx.QueryRow(ctx, `SELECT name FROM projects WHERE id=$1`, *matchProjectID).Scan(&projectName)
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"İlgili iş makinesi/araç/ekipman başka bir projede kullanılmaktadır. Lütfen projenize transferini talep edin.",
			map[string]string{"company_equipment_id": matchID.String(), "current_project_name": projectName})
		return
	}

	if b.KayitYeri == "firma_envanteri" {
		if matched {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Bu makine/ekipman/araç zaten firma envanterinde kayıtlı.", nil)
			return
		}
		var equipmentID uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO company_equipment
			    (tip, ad, plaka, marka, model, seri_no, demirbas_no, uretim_yili,
			     sahiplik, tedarikci, gunluk_ucret, durum,
			     son_bakim_tarihi, sonraki_bakim_tarihi, aciklama, current_project_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NULL)
			 RETURNING id`,
			b.Tip, b.Ad, b.Plaka, b.Marka, b.Model, b.SeriNo, b.DemirbasNo, b.UretimYili,
			b.Sahiplik, b.Tedarikci, b.GunlukUcret, b.Durum,
			b.SonBakimTarihi, b.SonrakiBakimTarihi, b.Aciklama,
		).Scan(&equipmentID); err != nil {
			httpx.Internal(w, r)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			httpx.Internal(w, r)
			return
		}
		httpx.JSON(w, http.StatusCreated, map[string]any{"company_equipment_id": equipmentID})
		return
	}

	// "Proje" modu: eşleşme varsa (envanterde atanmamış ya da zaten bu
	// projedeyse) mevcut company_equipment kaydını kullan; yoksa yeni aç.
	var equipmentID uuid.UUID
	if matched {
		equipmentID = matchID
		if _, err := tx.Exec(ctx,
			`UPDATE company_equipment SET current_project_id=$2, updated_at=now() WHERE id=$1`,
			equipmentID, pid); err != nil {
			httpx.Internal(w, r)
			return
		}
	} else {
		if err := tx.QueryRow(ctx,
			`INSERT INTO company_equipment
			    (tip, ad, plaka, marka, model, seri_no, demirbas_no, uretim_yili,
			     sahiplik, tedarikci, gunluk_ucret, durum,
			     son_bakim_tarihi, sonraki_bakim_tarihi, aciklama, current_project_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
			 RETURNING id`,
			b.Tip, b.Ad, b.Plaka, b.Marka, b.Model, b.SeriNo, b.DemirbasNo, b.UretimYili,
			b.Sahiplik, b.Tedarikci, b.GunlukUcret, b.Durum,
			b.SonBakimTarihi, b.SonrakiBakimTarihi, b.Aciklama, pid,
		).Scan(&equipmentID); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	var m Machine
	var assignmentID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO project_machines (project_id, company_equipment_id) VALUES ($1,$2) RETURNING id`,
		pid, equipmentID).Scan(&assignmentID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := scanMachine(tx.QueryRow(ctx,
		`SELECT `+machineSelectCols+`
		 FROM project_machines pm JOIN company_equipment ce ON ce.id = pm.company_equipment_id
		 WHERE pm.id=$1`, assignmentID), &m); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"machine": m})
}

func (h *Handler) UpdateMachine(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var b machineBody
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	ctx := r.Context()
	var m Machine
	err := scanMachine(h.pool.QueryRow(ctx,
		`WITH upd AS (
		   UPDATE company_equipment SET
		     ad                   = COALESCE(NULLIF($3,''), ad),
		     plaka                = COALESCE($4, plaka),
		     marka                = COALESCE($5, marka),
		     model                = COALESCE($6, model),
		     seri_no              = COALESCE($7, seri_no),
		     demirbas_no          = COALESCE($8, demirbas_no),
		     uretim_yili          = COALESCE($9, uretim_yili),
		     sahiplik             = COALESCE(NULLIF($10,''), sahiplik),
		     tedarikci            = COALESCE($11, tedarikci),
		     gunluk_ucret         = COALESCE($12, gunluk_ucret),
		     durum                = COALESCE(NULLIF($13,''), durum),
		     son_bakim_tarihi     = COALESCE($14::date, son_bakim_tarihi),
		     sonraki_bakim_tarihi = COALESCE($15::date, sonraki_bakim_tarihi),
		     aciklama             = COALESCE($16, aciklama),
		     updated_at           = now()
		   WHERE id = (SELECT company_equipment_id FROM project_machines WHERE id=$1 AND project_id=$2)
		   RETURNING id
		 )
		 SELECT `+machineSelectCols+`
		 FROM project_machines pm JOIN company_equipment ce ON ce.id = pm.company_equipment_id
		 WHERE pm.id=$1 AND EXISTS (SELECT 1 FROM upd WHERE upd.id = ce.id)`,
		id, pid, b.Ad, b.Plaka, b.Marka, b.Model, b.SeriNo, b.DemirbasNo, b.UretimYili,
		b.Sahiplik, b.Tedarikci, b.GunlukUcret, b.Durum,
		b.SonBakimTarihi, b.SonrakiBakimTarihi, b.Aciklama), &m)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"machine": m})
}

// DeleteMachine — atamayı (project_machines satırını) siler. Bu
// envanterdeki TEK atamaysa (başka hiçbir projede geçmiş/aktif ataması
// yoksa) company_equipment kaydı da birlikte silinir — Faz B'den önce
// bir makinenin birden fazla ataması olamayacağı için bu her zaman
// doğrudur; Faz B transfer geçmişi eklediğinde bu kontrol geçmişi
// koruyacak şekilde davranmaya devam eder.
func (h *Handler) DeleteMachine(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(ctx)

	var equipmentID uuid.UUID
	if err := tx.QueryRow(ctx,
		`DELETE FROM project_machines WHERE id=$1 AND project_id=$2 RETURNING company_equipment_id`,
		id, pid).Scan(&equipmentID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM company_equipment
		 WHERE id=$1 AND NOT EXISTS (SELECT 1 FROM project_machines WHERE company_equipment_id=$1)`,
		equipmentID); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkDelivered — hedef proje kullanıcısı, onaylı bir transferle gelen
// atamanın fiziksel teslimini onaylar. Sadece from_transfer_id dolu ve
// henüz teslim_alindi_tarihi boş atamalarda geçerlidir (bkz. Faz C).
// equipment.approve_transfer yetkisi olan hedef proje üyelerine
// (üst onaycılar) bildirim gider.
func (h *Handler) MarkDelivered(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	var equipmentAd string
	if err := h.pool.QueryRow(ctx,
		`UPDATE project_machines pm SET teslim_alindi_tarihi=CURRENT_DATE
		 FROM company_equipment ce
		 WHERE pm.id=$1 AND pm.project_id=$2 AND pm.company_equipment_id=ce.id
		   AND pm.from_transfer_id IS NOT NULL AND pm.teslim_alindi_tarihi IS NULL
		 RETURNING ce.ad`,
		id, pid).Scan(&equipmentAd); err != nil {
		if err == pgx.ErrNoRows {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound,
				"Teslim alınacak bir transfer ataması bulunamadı ya da zaten teslim alındı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	uid, _ := auth.UserIDFrom(ctx)
	var projectName string
	_ = h.pool.QueryRow(ctx, `SELECT name FROM projects WHERE id=$1`, pid).Scan(&projectName)
	equipmenttransfers.NotifyApprovers(ctx, h.pool, h.nt, pid, uid, notify.Input{
		Type:       notify.TypeEquipmentDelivered,
		Title:      equipmentAd + " — " + projectName + " projesinde teslim alındı",
		EntityType: "project_machines", EntityID: &id, ProjectID: &pid,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "delivered"})
}

// ── Machine Logs ──────────────────────────────────────────────────────────────

func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	machineID, ok := parseID(w, r, "machineID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT l.id, l.project_id, l.machine_id, ce.ad,
		        TO_CHAR(l.tarih,'YYYY-MM-DD'), l.calisma_miktari::float8,
		        l.calisma_birimi, l.operator, l.aciklama, l.created_at
		 FROM project_machine_logs l
		 JOIN project_machines pm ON pm.id = l.machine_id
		 JOIN company_equipment ce ON ce.id = pm.company_equipment_id
		 WHERE l.project_id=$1 AND l.machine_id=$2
		 ORDER BY l.tarih DESC, l.created_at DESC
		 LIMIT 365`, pid, machineID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []MachineLog{}
	for rows.Next() {
		var lg MachineLog
		if err := rows.Scan(&lg.ID, &lg.ProjectID, &lg.MachineID, &lg.MachineAd,
			&lg.Tarih, &lg.CalismaMiktari, &lg.CalismaBirimi, &lg.Operator,
			&lg.Aciklama, &lg.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, lg)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"logs": out})
}

func (h *Handler) CreateLog(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	machineID, ok := parseID(w, r, "machineID")
	if !ok {
		return
	}
	var b struct {
		Tarih          string  `json:"tarih"`
		CalismaMiktari float64 `json:"calisma_miktari"`
		CalismaBirimi  string  `json:"calisma_birimi"`
		Operator       *string `json:"operator"`
		Aciklama       *string `json:"aciklama"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Tarih == "" {
		b.Tarih = time.Now().Format("2006-01-02")
	}
	if b.CalismaBirimi == "" {
		b.CalismaBirimi = "saat"
	}
	var lg MachineLog
	err := h.pool.QueryRow(r.Context(),
		`WITH ins AS (
		   INSERT INTO project_machine_logs
		       (project_id, machine_id, tarih, calisma_miktari, calisma_birimi, operator, aciklama)
		   VALUES ($1,$2,$3,$4,$5,$6,$7)
		   RETURNING id, project_id, machine_id, tarih, calisma_miktari,
		             calisma_birimi, operator, aciklama, created_at
		 )
		 SELECT ins.id, ins.project_id, ins.machine_id, ce.ad,
		        TO_CHAR(ins.tarih,'YYYY-MM-DD'), ins.calisma_miktari::float8,
		        ins.calisma_birimi, ins.operator, ins.aciklama, ins.created_at
		 FROM ins
		 JOIN project_machines pm ON pm.id = ins.machine_id
		 JOIN company_equipment ce ON ce.id = pm.company_equipment_id`,
		pid, machineID, b.Tarih, b.CalismaMiktari, b.CalismaBirimi, b.Operator, b.Aciklama,
	).Scan(&lg.ID, &lg.ProjectID, &lg.MachineID, &lg.MachineAd,
		&lg.Tarih, &lg.CalismaMiktari, &lg.CalismaBirimi, &lg.Operator,
		&lg.Aciklama, &lg.CreatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"log": lg})
}

func (h *Handler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM project_machine_logs WHERE id=$1 AND project_id=$2`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
