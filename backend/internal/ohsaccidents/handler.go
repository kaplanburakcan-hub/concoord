// Package ohsaccidents — Dashboard v2: iş kazası kaydı ve "kazasız gün"
// sayacı. ohs_findings (denetim bulguları) ile KARIŞTIRILMAMALI: findings
// saha denetiminde tespit edilen ihlal/tehlikelerdir, kaza olmadan da
// açılabilir; buradaki accidents ise fiilen gerçekleşmiş kaza olaylarının
// tarih kaydıdır — "kazasız gün" yalnızca bu tablodan hesaplanır.
package ohsaccidents

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct{ pool *pgxpool.Pool }

func NewHandler(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool} }

type Accident struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	AccidentDate  string     `json:"accident_date"`
	Description   string     `json:"description"`
	Status        string     `json:"status"`
	CreatedByName string     `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
	ClosedByName  *string    `json:"closed_by_name,omitempty"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	CloseNote     *string    `json:"close_note,omitempty"`
	RowVersion    int        `json:"row_version"`
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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT a.id, a.project_id, to_char(a.accident_date,'YYYY-MM-DD'), a.description,
		       a.status, u.full_name, a.created_at, cu.full_name, a.closed_at, a.close_note,
		       a.row_version
		FROM ohs_accidents a
		JOIN users u ON u.id = a.created_by
		LEFT JOIN users cu ON cu.id = a.closed_by
		WHERE a.project_id=$1 AND a.deleted_at IS NULL
		ORDER BY a.accident_date DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []Accident{}
	for rows.Next() {
		var a Accident
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.AccidentDate, &a.Description,
			&a.Status, &a.CreatedByName, &a.CreatedAt, &a.ClosedByName, &a.ClosedAt, &a.CloseNote,
			&a.RowVersion); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, a)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"accidents": out})
}

type createReq struct {
	AccidentDate string `json:"accident_date"`
	Description  string `json:"description"`
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
	if _, err := time.Parse("2006-01-02", req.AccidentDate); err != nil {
		f["accident_date"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	}
	if req.AccidentDate > time.Now().UTC().Format("2006-01-02") {
		f["accident_date"] = "gelecek bir tarih olamaz"
	}
	if strings.TrimSpace(req.Description) == "" {
		f["description"] = "açıklama zorunlu"
	}
	if len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO ohs_accidents (project_id, accident_date, description, created_by)
		VALUES ($1,$2::date,$3,$4) RETURNING id`,
		pid, req.AccidentDate, strings.TrimSpace(req.Description), uid,
	).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
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
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE ohs_accidents SET deleted_at=now()
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

type closeReq struct {
	Note       string `json:"note"`
	RowVersion int    `json:"row_version"`
}

// Close — Investigating→Closed. Kaza kaydının statik alanları (tarih/açıklama)
// hâlâ düzenlenebilir kalır; yalnızca inceleme durumu tek yönlü kapanır
// (ohs_findings'teki gibi ayrı bir kilit tetikleyicisi yok, WHERE koşulu
// status='Investigating' yeterli çünkü tekrar açma akışı yok).
func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
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
	var req closeReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		httpx.ValidationFailed(w, r, map[string]string{"note": "kapatma notu zorunludur"})
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE ohs_accidents
		SET status='Closed', closed_by=$3, closed_at=now(), close_note=$4, row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND status='Investigating'
		  AND ($5=0 OR row_version=$5)`,
		id, pid, uid, note, req.RowVersion)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt bulunamadı, zaten kapatılmış ya da başkası tarafından güncellendi.", nil)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "Closed"})
}

// FreeDays — Dashboard'daki kazasız gün widget'ının HTTP karşılığı,
// İSG Bulguları sayfasında dashboard'un tamamını çekmeden aynı sayacı
// göstermek için.
func (h *Handler) FreeDays(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	fd, err := LoadFreeDays(r.Context(), h.pool, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, fd)
}

// FreeDaysSummary — dashboard kartı için: bugün ile referans tarih
// arasındaki gün sayısı. Hiç kaza kaydı yoksa referans proje başlangıç
// tarihidir (o da yoksa gösterilecek bir şey yoktur — HasReference=false,
// proje başlangıcından ÖNCEYE asla inilmez).
type FreeDaysSummary struct {
	Days          int     `json:"days"`
	ReferenceDate *string `json:"reference_date"`
	SinceAccident bool    `json:"since_accident"` // true: son kazadan beri, false: proje başlangıcından beri
	HasReference  bool    `json:"has_reference"`
}

// LoadFreeDays — HTTP dışı, dashboard.Handler.ProjectDashboard tarafından
// çağrılır (Faz F'nin fixedexpenses.ListActive deseniyle aynı: küçük
// modüller birbirinin exported fonksiyonunu doğrudan çağırır, ayrı bir
// HTTP round-trip gerekmez).
func LoadFreeDays(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID) (FreeDaysSummary, error) {
	var lastAccident *string
	if err := pool.QueryRow(ctx, `
		SELECT to_char(max(accident_date),'YYYY-MM-DD')
		FROM ohs_accidents WHERE project_id=$1 AND deleted_at IS NULL`, projectID,
	).Scan(&lastAccident); err != nil {
		return FreeDaysSummary{}, err
	}

	var out FreeDaysSummary
	var ref *string
	if lastAccident != nil {
		ref = lastAccident
		out.SinceAccident = true
	} else {
		var startDate *string
		if err := pool.QueryRow(ctx, `
			SELECT to_char(start_date,'YYYY-MM-DD') FROM projects WHERE id=$1`, projectID,
		).Scan(&startDate); err != nil {
			return FreeDaysSummary{}, err
		}
		ref = startDate
		out.SinceAccident = false
	}
	if ref == nil {
		out.HasReference = false
		return out, nil
	}
	refDate, err := time.Parse("2006-01-02", *ref)
	if err != nil {
		return FreeDaysSummary{}, err
	}
	days := int(time.Now().UTC().Truncate(24*time.Hour).Sub(refDate).Hours() / 24)
	if days < 0 {
		days = 0
	}
	out.Days = days
	out.ReferenceDate = ref
	out.HasReference = true
	return out, nil
}
