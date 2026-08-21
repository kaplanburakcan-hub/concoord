package warehouse

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/validate"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

// ── DTOs ─────────────────────────────────────────────────────────────────────

type Item struct {
	ID           uuid.UUID `json:"id"`
	ProjectID    uuid.UUID `json:"project_id"`
	MalzemeAdi   string    `json:"malzeme_adi"`
	Kategori     string    `json:"kategori"`
	Birim        string    `json:"birim"`
	MevcutMiktar float64   `json:"mevcut_miktar"`
	MinStok      float64   `json:"min_stok"`
	Aciklama     *string   `json:"aciklama,omitempty"`
	Sira         int       `json:"sira"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Movement struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"project_id"`
	ItemID      uuid.UUID `json:"item_id"`
	MalzemeAdi  string    `json:"malzeme_adi"`
	HareketTuru string    `json:"hareket_turu"`
	Miktar      float64   `json:"miktar"`
	Tarih       string    `json:"tarih"`
	BelgeNo     *string   `json:"belge_no,omitempty"`
	Aciklama    *string   `json:"aciklama,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

// ── Items ─────────────────────────────────────────────────────────────────────

func (h *Handler) ListItems(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, project_id, malzeme_adi, kategori, birim,
		        mevcut_miktar::float8, min_stok::float8, aciklama, sira, created_at, updated_at
		 FROM site_warehouse_items WHERE project_id=$1 ORDER BY kategori, sira, malzeme_adi`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.ProjectID, &it.MalzemeAdi, &it.Kategori, &it.Birim,
			&it.MevcutMiktar, &it.MinStok, &it.Aciklama, &it.Sira, &it.CreatedAt, &it.UpdatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, it)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var b struct {
		MalzemeAdi   string  `json:"malzeme_adi"`
		Kategori     string  `json:"kategori"`
		Birim        string  `json:"birim"`
		MevcutMiktar float64 `json:"mevcut_miktar"`
		MinStok      float64 `json:"min_stok"`
		Aciklama     *string `json:"aciklama"`
		Sira         *int    `json:"sira"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.MalzemeAdi == "" {
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "malzeme_adi zorunludur.", nil)
		return
	}
	if b.Kategori == "" {
		b.Kategori = "Genel"
	}
	if b.Birim == "" {
		b.Birim = "adet"
	}
	sira := 0
	if b.Sira != nil {
		sira = *b.Sira
	}
	var it Item
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO site_warehouse_items
		    (project_id, malzeme_adi, kategori, birim, mevcut_miktar, min_stok, aciklama, sira)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, project_id, malzeme_adi, kategori, birim,
		           mevcut_miktar::float8, min_stok::float8, aciklama, sira, created_at, updated_at`,
		pid, b.MalzemeAdi, b.Kategori, b.Birim, b.MevcutMiktar, b.MinStok, b.Aciklama, sira,
	).Scan(&it.ID, &it.ProjectID, &it.MalzemeAdi, &it.Kategori, &it.Birim,
		&it.MevcutMiktar, &it.MinStok, &it.Aciklama, &it.Sira, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"item": it})
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var b struct {
		MalzemeAdi   *string  `json:"malzeme_adi"`
		Kategori     *string  `json:"kategori"`
		Birim        *string  `json:"birim"`
		MevcutMiktar *float64 `json:"mevcut_miktar"`
		MinStok      *float64 `json:"min_stok"`
		Aciklama     *string  `json:"aciklama"`
		Sira         *int     `json:"sira"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	var it Item
	err := h.pool.QueryRow(r.Context(),
		`UPDATE site_warehouse_items SET
		   malzeme_adi   = COALESCE($3, malzeme_adi),
		   kategori      = COALESCE($4, kategori),
		   birim         = COALESCE($5, birim),
		   mevcut_miktar = COALESCE($6, mevcut_miktar),
		   min_stok      = COALESCE($7, min_stok),
		   aciklama      = COALESCE($8, aciklama),
		   sira          = COALESCE($9, sira),
		   updated_at    = NOW()
		 WHERE id=$1 AND project_id=$2
		 RETURNING id, project_id, malzeme_adi, kategori, birim,
		           mevcut_miktar::float8, min_stok::float8, aciklama, sira, created_at, updated_at`,
		id, pid, b.MalzemeAdi, b.Kategori, b.Birim, b.MevcutMiktar, b.MinStok, b.Aciklama, b.Sira,
	).Scan(&it.ID, &it.ProjectID, &it.MalzemeAdi, &it.Kategori, &it.Birim,
		&it.MevcutMiktar, &it.MinStok, &it.Aciklama, &it.Sira, &it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"item": it})
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM site_warehouse_items WHERE id=$1 AND project_id=$2`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Movements ─────────────────────────────────────────────────────────────────

func (h *Handler) ListMovements(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT m.id, m.project_id, m.item_id, i.malzeme_adi,
		        m.hareket_turu, m.miktar::float8,
		        TO_CHAR(m.tarih,'YYYY-MM-DD'), m.belge_no, m.aciklama, m.created_at
		 FROM site_warehouse_movements m
		 JOIN site_warehouse_items i ON i.id = m.item_id
		 WHERE m.project_id=$1
		 ORDER BY m.tarih DESC, m.created_at DESC
		 LIMIT 200`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Movement{}
	for rows.Next() {
		var mv Movement
		if err := rows.Scan(&mv.ID, &mv.ProjectID, &mv.ItemID, &mv.MalzemeAdi,
			&mv.HareketTuru, &mv.Miktar, &mv.Tarih, &mv.BelgeNo, &mv.Aciklama, &mv.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, mv)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"movements": out})
}

func (h *Handler) CreateMovement(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var b struct {
		ItemID      string  `json:"item_id"`
		HareketTuru string  `json:"hareket_turu"`
		Miktar      float64 `json:"miktar"`
		Tarih       string  `json:"tarih"`
		BelgeNo     *string `json:"belge_no"`
		Aciklama    *string `json:"aciklama"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	itemID, err := uuid.Parse(b.ItemID)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz item_id.", nil)
		return
	}
	if b.Miktar <= 0 {
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "miktar > 0 olmalıdır.", nil)
		return
	}
	if b.Tarih == "" {
		b.Tarih = time.Now().Format("2006-01-02")
	}
	if b.HareketTuru == "" {
		b.HareketTuru = "giris"
	}
	if t, perr := time.Parse("2006-01-02", b.Tarih); perr != nil {
		httpx.ValidationFailed(w, r, map[string]string{"tarih": "geçersiz tarih biçimi"})
		return
	} else if errs, kerr := validate.NotAfterKesinKabul(r.Context(), h.pool, pid, t, "tarih"); kerr != nil {
		httpx.Internal(w, r)
		return
	} else if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var mv Movement
	err = tx.QueryRow(r.Context(),
		`INSERT INTO site_warehouse_movements
		    (project_id, item_id, hareket_turu, miktar, tarih, belge_no, aciklama)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, project_id, item_id, hareket_turu, miktar::float8,
		           TO_CHAR(tarih,'YYYY-MM-DD'), belge_no, aciklama, created_at`,
		pid, itemID, b.HareketTuru, b.Miktar, b.Tarih, b.BelgeNo, b.Aciklama,
	).Scan(&mv.ID, &mv.ProjectID, &mv.ItemID, &mv.HareketTuru, &mv.Miktar,
		&mv.Tarih, &mv.BelgeNo, &mv.Aciklama, &mv.CreatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	mv.ItemID = itemID

	if b.HareketTuru == "sayim" {
		_, err = tx.Exec(r.Context(),
			`UPDATE site_warehouse_items SET mevcut_miktar=$1, updated_at=NOW() WHERE id=$2 AND project_id=$3`,
			b.Miktar, itemID, pid)
	} else if b.HareketTuru == "cikis" {
		_, err = tx.Exec(r.Context(),
			`UPDATE site_warehouse_items SET mevcut_miktar=mevcut_miktar-$1, updated_at=NOW()
			 WHERE id=$2 AND project_id=$3`, b.Miktar, itemID, pid)
	} else {
		_, err = tx.Exec(r.Context(),
			`UPDATE site_warehouse_items SET mevcut_miktar=mevcut_miktar+$1, updated_at=NOW()
			 WHERE id=$2 AND project_id=$3`, b.Miktar, itemID, pid)
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"movement": mv})
}

func (h *Handler) DeleteMovement(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM site_warehouse_movements WHERE id=$1 AND project_id=$2`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
