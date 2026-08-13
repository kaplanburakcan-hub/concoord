// Package fixedexpenses — Nakit Akış Faz E: Sabit Giderler (araç kiraları,
// endirekt personel, mobilizasyon sarf vb. tekrarlayan aylık giderler).
//
// Bu paket yalnızca CRUD sağlar; kayıtlar cash_events'e YAZILMAZ (Render'da
// arka plan işçisi çalışmadığı için önceden satır satır üretilemez). Bunun
// yerine Expand, istenen [from,to] tarih aralığı için kayıtları ay ay
// SANAL olarak genişletir — nakit akış raporu (Faz F) bunu gerçek
// cash_events satırlarıyla birleştirir. Expand DB'ye dokunmaz, saf bir
// fonksiyondur.
package fixedexpenses

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

type FixedExpense struct {
	ID                uuid.UUID `json:"id"`
	ProjectID         uuid.UUID `json:"project_id"`
	Label             string    `json:"label"`
	Amount            float64   `json:"amount"`
	Category          string    `json:"category"`
	ExpenseDayOfMonth int       `json:"expense_day_of_month"`
	StartDate         string    `json:"start_date"`
	EndDate           *string   `json:"end_date,omitempty"`
	Active            bool      `json:"active"`
	CreatedByName     string    `json:"created_by_name"`
	CreatedAt         time.Time `json:"created_at"`
	RowVersion        int       `json:"row_version"`
}

const listCols = `
	f.id, f.project_id, f.label, f.amount::float8, f.category, f.expense_day_of_month,
	to_char(f.start_date,'YYYY-MM-DD'), to_char(f.end_date,'YYYY-MM-DD'),
	f.active, u.full_name, f.created_at, f.row_version`

func scanRow(row pgx.Row, f *FixedExpense) error {
	return row.Scan(&f.ID, &f.ProjectID, &f.Label, &f.Amount, &f.Category, &f.ExpenseDayOfMonth,
		&f.StartDate, &f.EndDate, &f.Active, &f.CreatedByName, &f.CreatedAt, &f.RowVersion)
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
		FROM fixed_expenses f
		JOIN users u ON u.id = f.created_by
		WHERE f.project_id=$1 AND f.deleted_at IS NULL
		ORDER BY f.active DESC, f.label`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []FixedExpense{}
	for rows.Next() {
		var f FixedExpense
		if err := scanRow(rows, &f); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, f)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"fixed_expenses": out})
}

// ── Create ───────────────────────────────────────────────────────────────

type upsertReq struct {
	Label             string  `json:"label"`
	Amount            float64 `json:"amount"`
	Category          string  `json:"category"`
	ExpenseDayOfMonth int     `json:"expense_day_of_month"`
	StartDate         string  `json:"start_date"`
	EndDate           *string `json:"end_date"`
	Active            *bool   `json:"active"`
	RowVersion        int     `json:"row_version"`
}

func validate(req upsertReq) map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(req.Label) == "" {
		f["label"] = "başlık zorunlu"
	}
	if req.Amount <= 0 {
		f["amount"] = "0'dan büyük olmalı"
	}
	if req.ExpenseDayOfMonth < 1 || req.ExpenseDayOfMonth > 28 {
		f["expense_day_of_month"] = "1-28 arası olmalı"
	}
	if _, err := time.Parse("2006-01-02", req.StartDate); err != nil {
		f["start_date"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	}
	if req.EndDate != nil && strings.TrimSpace(*req.EndDate) != "" {
		if _, err := time.Parse("2006-01-02", *req.EndDate); err != nil {
			f["end_date"] = "geçerli bir tarih (YYYY-MM-DD) girin"
		} else if req.StartDate != "" && *req.EndDate < req.StartDate {
			f["end_date"] = "başlangıç tarihinden önce olamaz"
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
	var req upsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "Diğer"
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO fixed_expenses
			(project_id, label, amount, category, expense_day_of_month, start_date, end_date, active, created_by)
		VALUES ($1,$2,$3,$4,$5,$6::date,NULLIF($7,'')::date,$8,$9)
		RETURNING id`,
		pid, strings.TrimSpace(req.Label), req.Amount, category, req.ExpenseDayOfMonth,
		req.StartDate, strDeref(req.EndDate), active, uid,
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
	var req upsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := validate(req); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "Diğer"
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE fixed_expenses SET
			label=$3, amount=$4, category=$5, expense_day_of_month=$6,
			start_date=$7::date, end_date=NULLIF($8,'')::date, active=$9,
			updated_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL AND row_version=$10`,
		id, pid, strings.TrimSpace(req.Label), req.Amount, category, req.ExpenseDayOfMonth,
		req.StartDate, strDeref(req.EndDate), active, req.RowVersion)
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
		UPDATE fixed_expenses SET deleted_at=now()
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

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
