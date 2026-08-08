package machines

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

// ── DTOs ──────────────────────────────────────────────────────────────────────

type Machine struct {
	ID                  uuid.UUID  `json:"id"`
	ProjectID           uuid.UUID  `json:"project_id"`
	Tip                 string     `json:"tip"`
	Ad                  string     `json:"ad"`
	Plaka               *string    `json:"plaka,omitempty"`
	Marka               *string    `json:"marka,omitempty"`
	Model               *string    `json:"model,omitempty"`
	SeriNo              *string    `json:"seri_no,omitempty"`
	UretimYili          *int       `json:"uretim_yili,omitempty"`
	Sahiplik            string     `json:"sahiplik"`
	Tedarikci           *string    `json:"tedarikci,omitempty"`
	GunlukUcret         *float64   `json:"gunluk_ucret,omitempty"`
	Durum               string     `json:"durum"`
	SonBakimTarihi      *string    `json:"son_bakim_tarihi,omitempty"`
	SonrakiBakimTarihi  *string    `json:"sonraki_bakim_tarihi,omitempty"`
	Aciklama            *string    `json:"aciklama,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type MachineLog struct {
	ID              uuid.UUID `json:"id"`
	ProjectID       uuid.UUID `json:"project_id"`
	MachineID       uuid.UUID `json:"machine_id"`
	MachineAd       string    `json:"machine_ad"`
	Tarih           string    `json:"tarih"`
	CalismaMiktari  float64   `json:"calisma_miktari"`
	CalismaBirimi   string    `json:"calisma_birimi"`
	Operator        *string   `json:"operator,omitempty"`
	Aciklama        *string   `json:"aciklama,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
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
	var rows interface {
		Next() bool
		Close()
		Scan(...any) error
	}
	var err error
	if tip != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT id, project_id, tip, ad, plaka, marka, model, seri_no, uretim_yili,
			        sahiplik, tedarikci, gunluk_ucret::float8, durum,
			        TO_CHAR(son_bakim_tarihi,'YYYY-MM-DD'),
			        TO_CHAR(sonraki_bakim_tarihi,'YYYY-MM-DD'),
			        aciklama, created_at, updated_at
			 FROM project_machines WHERE project_id=$1 AND tip=$2
			 ORDER BY ad`, pid, tip)
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT id, project_id, tip, ad, plaka, marka, model, seri_no, uretim_yili,
			        sahiplik, tedarikci, gunluk_ucret::float8, durum,
			        TO_CHAR(son_bakim_tarihi,'YYYY-MM-DD'),
			        TO_CHAR(sonraki_bakim_tarihi,'YYYY-MM-DD'),
			        aciklama, created_at, updated_at
			 FROM project_machines WHERE project_id=$1
			 ORDER BY tip, ad`, pid)
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Machine{}
	for rows.Next() {
		var m Machine
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Tip, &m.Ad, &m.Plaka, &m.Marka, &m.Model,
			&m.SeriNo, &m.UretimYili, &m.Sahiplik, &m.Tedarikci, &m.GunlukUcret, &m.Durum,
			&m.SonBakimTarihi, &m.SonrakiBakimTarihi, &m.Aciklama, &m.CreatedAt, &m.UpdatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, m)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"machines": out})
}

func (h *Handler) CreateMachine(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var b struct {
		Tip                string   `json:"tip"`
		Ad                 string   `json:"ad"`
		Plaka              *string  `json:"plaka"`
		Marka              *string  `json:"marka"`
		Model              *string  `json:"model"`
		SeriNo             *string  `json:"seri_no"`
		UretimYili         *int     `json:"uretim_yili"`
		Sahiplik           string   `json:"sahiplik"`
		Tedarikci          *string  `json:"tedarikci"`
		GunlukUcret        *float64 `json:"gunluk_ucret"`
		Durum              string   `json:"durum"`
		SonBakimTarihi     *string  `json:"son_bakim_tarihi"`
		SonrakiBakimTarihi *string  `json:"sonraki_bakim_tarihi"`
		Aciklama           *string  `json:"aciklama"`
	}
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
	var m Machine
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO project_machines
		    (project_id, tip, ad, plaka, marka, model, seri_no, uretim_yili,
		     sahiplik, tedarikci, gunluk_ucret, durum,
		     son_bakim_tarihi, sonraki_bakim_tarihi, aciklama)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 RETURNING id, project_id, tip, ad, plaka, marka, model, seri_no, uretim_yili,
		           sahiplik, tedarikci, gunluk_ucret::float8, durum,
		           TO_CHAR(son_bakim_tarihi,'YYYY-MM-DD'),
		           TO_CHAR(sonraki_bakim_tarihi,'YYYY-MM-DD'),
		           aciklama, created_at, updated_at`,
		pid, b.Tip, b.Ad, b.Plaka, b.Marka, b.Model, b.SeriNo, b.UretimYili,
		b.Sahiplik, b.Tedarikci, b.GunlukUcret, b.Durum,
		b.SonBakimTarihi, b.SonrakiBakimTarihi, b.Aciklama,
	).Scan(&m.ID, &m.ProjectID, &m.Tip, &m.Ad, &m.Plaka, &m.Marka, &m.Model,
		&m.SeriNo, &m.UretimYili, &m.Sahiplik, &m.Tedarikci, &m.GunlukUcret, &m.Durum,
		&m.SonBakimTarihi, &m.SonrakiBakimTarihi, &m.Aciklama, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
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
	var b struct {
		Ad                 *string  `json:"ad"`
		Plaka              *string  `json:"plaka"`
		Marka              *string  `json:"marka"`
		Model              *string  `json:"model"`
		SeriNo             *string  `json:"seri_no"`
		UretimYili         *int     `json:"uretim_yili"`
		Sahiplik           *string  `json:"sahiplik"`
		Tedarikci          *string  `json:"tedarikci"`
		GunlukUcret        *float64 `json:"gunluk_ucret"`
		Durum              *string  `json:"durum"`
		SonBakimTarihi     *string  `json:"son_bakim_tarihi"`
		SonrakiBakimTarihi *string  `json:"sonraki_bakim_tarihi"`
		Aciklama           *string  `json:"aciklama"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	var m Machine
	err := h.pool.QueryRow(r.Context(),
		`UPDATE project_machines SET
		   ad                  = COALESCE($3, ad),
		   plaka               = COALESCE($4, plaka),
		   marka               = COALESCE($5, marka),
		   model               = COALESCE($6, model),
		   seri_no             = COALESCE($7, seri_no),
		   uretim_yili         = COALESCE($8, uretim_yili),
		   sahiplik            = COALESCE($9, sahiplik),
		   tedarikci           = COALESCE($10, tedarikci),
		   gunluk_ucret        = COALESCE($11, gunluk_ucret),
		   durum               = COALESCE($12, durum),
		   son_bakim_tarihi    = COALESCE($13::date, son_bakim_tarihi),
		   sonraki_bakim_tarihi= COALESCE($14::date, sonraki_bakim_tarihi),
		   aciklama            = COALESCE($15, aciklama),
		   updated_at          = NOW()
		 WHERE id=$1 AND project_id=$2
		 RETURNING id, project_id, tip, ad, plaka, marka, model, seri_no, uretim_yili,
		           sahiplik, tedarikci, gunluk_ucret::float8, durum,
		           TO_CHAR(son_bakim_tarihi,'YYYY-MM-DD'),
		           TO_CHAR(sonraki_bakim_tarihi,'YYYY-MM-DD'),
		           aciklama, created_at, updated_at`,
		id, pid, b.Ad, b.Plaka, b.Marka, b.Model, b.SeriNo, b.UretimYili,
		b.Sahiplik, b.Tedarikci, b.GunlukUcret, b.Durum,
		b.SonBakimTarihi, b.SonrakiBakimTarihi, b.Aciklama,
	).Scan(&m.ID, &m.ProjectID, &m.Tip, &m.Ad, &m.Plaka, &m.Marka, &m.Model,
		&m.SeriNo, &m.UretimYili, &m.Sahiplik, &m.Tedarikci, &m.GunlukUcret, &m.Durum,
		&m.SonBakimTarihi, &m.SonrakiBakimTarihi, &m.Aciklama, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"machine": m})
}

func (h *Handler) DeleteMachine(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM project_machines WHERE id=$1 AND project_id=$2`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		`SELECT l.id, l.project_id, l.machine_id, m.ad,
		        TO_CHAR(l.tarih,'YYYY-MM-DD'), l.calisma_miktari::float8,
		        l.calisma_birimi, l.operator, l.aciklama, l.created_at
		 FROM project_machine_logs l
		 JOIN project_machines m ON m.id = l.machine_id
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
		Tarih          string   `json:"tarih"`
		CalismaMiktari float64  `json:"calisma_miktari"`
		CalismaBirimi  string   `json:"calisma_birimi"`
		Operator       *string  `json:"operator"`
		Aciklama       *string  `json:"aciklama"`
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
		 SELECT ins.id, ins.project_id, ins.machine_id, m.ad,
		        TO_CHAR(ins.tarih,'YYYY-MM-DD'), ins.calisma_miktari::float8,
		        ins.calisma_birimi, ins.operator, ins.aciklama, ins.created_at
		 FROM ins JOIN project_machines m ON m.id = ins.machine_id`,
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
