package meetings

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

type Meeting struct {
	ID               uuid.UUID `json:"id"`
	ProjectID        uuid.UUID `json:"project_id"`
	TopantiNo        *string   `json:"toplanti_no,omitempty"`
	Baslik           string    `json:"baslik"`
	TopantiTuru      string    `json:"toplanti_turu"`
	Tarih            string    `json:"tarih"`
	BaslangicSaati   *string   `json:"baslangic_saati,omitempty"`
	BitisSaati       *string   `json:"bitis_saati,omitempty"`
	Lokasyon         *string   `json:"lokasyon,omitempty"`
	Katilimcilar     []string  `json:"katilimcilar"`
	Gundem           *string   `json:"gundem,omitempty"`
	Kararlar         *string   `json:"kararlar,omitempty"`
	AksiyonMaddeleri []byte    `json:"aksiyon_maddeleri"`
	SonrakiToplanti  *string   `json:"sonraki_toplanti_tarihi,omitempty"`
	Durum            string    `json:"durum"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz ID.", nil)
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, project_id, toplanti_no, baslik, toplanti_turu,
		        TO_CHAR(tarih,'YYYY-MM-DD'),
		        TO_CHAR(baslangic_saati,'HH24:MI'), TO_CHAR(bitis_saati,'HH24:MI'),
		        lokasyon, katilimcilar, gundem, kararlar,
		        aksiyon_maddeleri::text,
		        TO_CHAR(sonraki_toplanti_tarihi,'YYYY-MM-DD'),
		        durum, created_at, updated_at
		 FROM project_meetings
		 WHERE project_id=$1
		 ORDER BY tarih DESC, created_at DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Meeting{}
	for rows.Next() {
		var m Meeting
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.TopantiNo, &m.Baslik, &m.TopantiTuru,
			&m.Tarih, &m.BaslangicSaati, &m.BitisSaati, &m.Lokasyon,
			&m.Katilimcilar, &m.Gundem, &m.Kararlar, &m.AksiyonMaddeleri,
			&m.SonrakiToplanti, &m.Durum, &m.CreatedAt, &m.UpdatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, m)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"meetings": out})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var b struct {
		TopantiNo        *string  `json:"toplanti_no"`
		Baslik           string   `json:"baslik"`
		TopantiTuru      string   `json:"toplanti_turu"`
		Tarih            string   `json:"tarih"`
		BaslangicSaati   *string  `json:"baslangic_saati"`
		BitisSaati       *string  `json:"bitis_saati"`
		Lokasyon         *string  `json:"lokasyon"`
		Katilimcilar     []string `json:"katilimcilar"`
		Gundem           *string  `json:"gundem"`
		Kararlar         *string  `json:"kararlar"`
		AksiyonMaddeleri *string  `json:"aksiyon_maddeleri"`
		SonrakiToplanti  *string  `json:"sonraki_toplanti_tarihi"`
		Durum            string   `json:"durum"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Baslik == "" {
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "baslik zorunludur.", nil)
		return
	}
	if b.Tarih == "" {
		b.Tarih = time.Now().Format("2006-01-02")
	}
	if b.TopantiTuru == "" {
		b.TopantiTuru = "ilerleme"
	}
	if b.Durum == "" {
		b.Durum = "planli"
	}
	if b.Katilimcilar == nil {
		b.Katilimcilar = []string{}
	}
	aksiyonJSON := "[]"
	if b.AksiyonMaddeleri != nil {
		aksiyonJSON = *b.AksiyonMaddeleri
	}
	if !checkKesinKabul(w, r, h.pool, pid, b.Tarih, "tarih") {
		return
	}
	if b.SonrakiToplanti != nil && !checkKesinKabul(w, r, h.pool, pid, *b.SonrakiToplanti, "sonraki_toplanti_tarihi") {
		return
	}

	var m Meeting
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO project_meetings
		    (project_id, toplanti_no, baslik, toplanti_turu, tarih,
		     baslangic_saati, bitis_saati, lokasyon, katilimcilar,
		     gundem, kararlar, aksiyon_maddeleri, sonraki_toplanti_tarihi, durum)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,
		         NULLIF($13,'')::date,$14)
		 RETURNING id, project_id, toplanti_no, baslik, toplanti_turu,
		        TO_CHAR(tarih,'YYYY-MM-DD'),
		        TO_CHAR(baslangic_saati,'HH24:MI'), TO_CHAR(bitis_saati,'HH24:MI'),
		        lokasyon, katilimcilar, gundem, kararlar,
		        aksiyon_maddeleri::text,
		        TO_CHAR(sonraki_toplanti_tarihi,'YYYY-MM-DD'),
		        durum, created_at, updated_at`,
		pid, b.TopantiNo, b.Baslik, b.TopantiTuru, b.Tarih,
		nilStr(b.BaslangicSaati), nilStr(b.BitisSaati), b.Lokasyon, b.Katilimcilar,
		b.Gundem, b.Kararlar, aksiyonJSON, nilStr(b.SonrakiToplanti), b.Durum,
	).Scan(&m.ID, &m.ProjectID, &m.TopantiNo, &m.Baslik, &m.TopantiTuru,
		&m.Tarih, &m.BaslangicSaati, &m.BitisSaati, &m.Lokasyon,
		&m.Katilimcilar, &m.Gundem, &m.Kararlar, &m.AksiyonMaddeleri,
		&m.SonrakiToplanti, &m.Durum, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"meeting": m})
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
	var b struct {
		TopantiNo        *string  `json:"toplanti_no"`
		Baslik           *string  `json:"baslik"`
		TopantiTuru      *string  `json:"toplanti_turu"`
		Tarih            *string  `json:"tarih"`
		BaslangicSaati   *string  `json:"baslangic_saati"`
		BitisSaati       *string  `json:"bitis_saati"`
		Lokasyon         *string  `json:"lokasyon"`
		Katilimcilar     []string `json:"katilimcilar"`
		Gundem           *string  `json:"gundem"`
		Kararlar         *string  `json:"kararlar"`
		AksiyonMaddeleri *string  `json:"aksiyon_maddeleri"`
		SonrakiToplanti  *string  `json:"sonraki_toplanti_tarihi"`
		Durum            *string  `json:"durum"`
	}
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if b.Tarih != nil && !checkKesinKabul(w, r, h.pool, pid, *b.Tarih, "tarih") {
		return
	}
	if b.SonrakiToplanti != nil && !checkKesinKabul(w, r, h.pool, pid, *b.SonrakiToplanti, "sonraki_toplanti_tarihi") {
		return
	}
	var m Meeting
	err := h.pool.QueryRow(r.Context(),
		`UPDATE project_meetings SET
		   toplanti_no             = COALESCE($3, toplanti_no),
		   baslik                  = COALESCE($4, baslik),
		   toplanti_turu           = COALESCE($5, toplanti_turu),
		   tarih                   = COALESCE(NULLIF($6,'')::date, tarih),
		   baslangic_saati         = COALESCE(NULLIF($7,'')::time, baslangic_saati),
		   bitis_saati             = COALESCE(NULLIF($8,'')::time, bitis_saati),
		   lokasyon                = COALESCE($9, lokasyon),
		   katilimcilar            = COALESCE($10, katilimcilar),
		   gundem                  = COALESCE($11, gundem),
		   kararlar                = COALESCE($12, kararlar),
		   aksiyon_maddeleri       = COALESCE(NULLIF($13,'')::jsonb, aksiyon_maddeleri),
		   sonraki_toplanti_tarihi = COALESCE(NULLIF($14,'')::date, sonraki_toplanti_tarihi),
		   durum                   = COALESCE($15, durum),
		   updated_at              = NOW()
		 WHERE id=$1 AND project_id=$2
		 RETURNING id, project_id, toplanti_no, baslik, toplanti_turu,
		        TO_CHAR(tarih,'YYYY-MM-DD'),
		        TO_CHAR(baslangic_saati,'HH24:MI'), TO_CHAR(bitis_saati,'HH24:MI'),
		        lokasyon, katilimcilar, gundem, kararlar,
		        aksiyon_maddeleri::text,
		        TO_CHAR(sonraki_toplanti_tarihi,'YYYY-MM-DD'),
		        durum, created_at, updated_at`,
		id, pid, b.TopantiNo, b.Baslik, b.TopantiTuru, nilStr(b.Tarih),
		nilStr(b.BaslangicSaati), nilStr(b.BitisSaati), b.Lokasyon, b.Katilimcilar,
		b.Gundem, b.Kararlar, nilStr(b.AksiyonMaddeleri), nilStr(b.SonrakiToplanti), b.Durum,
	).Scan(&m.ID, &m.ProjectID, &m.TopantiNo, &m.Baslik, &m.TopantiTuru,
		&m.Tarih, &m.BaslangicSaati, &m.BitisSaati, &m.Lokasyon,
		&m.Katilimcilar, &m.Gundem, &m.Kararlar, &m.AksiyonMaddeleri,
		&m.SonrakiToplanti, &m.Durum, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"meeting": m})
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
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM project_meetings WHERE id=$1 AND project_id=$2`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// checkKesinKabul — toplantı tarihi ve sonraki toplantı tarihi projenin
// Kesin Kabul tarihini aşamaz (proje bittikten sonra toplantı planlamak
// anlamsız). dateStr boşsa (alan gönderilmemiş/temizlenmiş) kontrol atlanır.
func checkKesinKabul(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, pid uuid.UUID, dateStr, fieldName string) bool {
	if dateStr == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{fieldName: "geçersiz tarih biçimi"})
		return false
	}
	if errs, err := validate.NotAfterKesinKabul(r.Context(), pool, pid, t, fieldName); err != nil {
		httpx.Internal(w, r)
		return false
	} else if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return false
	}
	return true
}

func nilStr(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}
