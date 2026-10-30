// Package costcodes — proje maliyet kodu tanım listesi (Plan E6, ilk adım).
// Bütçe, PO ve hakediş ile bağlantı bu aşamada yoktur.
package costcodes

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
	rec  *audit.Recorder
}

func NewHandler(pool *pgxpool.Pool, rec *audit.Recorder) *Handler {
	return &Handler{pool: pool, rec: rec}
}

type CodeDTO struct {
	ID        uuid.UUID  `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Budget    *float64   `json:"budget,omitempty"`
	CreatedBy string     `json:"created_by_name"`
	CreatedAt string     `json:"created_at"`
}

type codeInput struct {
	Code     string     `json:"code"`
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, c.code, c.name, c.parent_id, u.full_name, to_char(c.created_at,'YYYY-MM-DD'),
		       b.amount::float8
		FROM cost_codes c
		JOIN users u ON u.id = c.created_by
		LEFT JOIN cost_code_budgets b ON b.cost_code_id = c.id
		WHERE c.project_id=$1 ORDER BY c.code`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []CodeDTO{}
	for rows.Next() {
		var d CodeDTO
		if err := rows.Scan(&d.ID, &d.Code, &d.Name, &d.ParentID, &d.CreatedBy, &d.CreatedAt, &d.Budget); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"cost_codes": out})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	var in codeInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	code := strings.TrimSpace(in.Code)
	name := strings.TrimSpace(in.Name)
	errs := map[string]string{}
	if code == "" || len([]rune(code)) > 40 {
		errs["code"] = "kod zorunlu (en fazla 40 karakter)"
	}
	if name == "" || len([]rune(name)) > 200 {
		errs["name"] = "ad zorunlu (en fazla 200 karakter)"
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	var id uuid.UUID
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO cost_codes (project_id, code, name, parent_id, created_by)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, pid, code, name, in.ParentID, uid).Scan(&id)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "uq_cost_code"):
			httpx.ValidationFailed(w, r, map[string]string{"code": "bu kod projede zaten var"})
		case strings.Contains(msg, "başka projeye ait"):
			httpx.ValidationFailed(w, r, map[string]string{"parent_id": "bu projede kayıtlı bir kod değil"})
		default:
			httpx.Internal(w, r)
		}
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "cost_codes", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"code": code, "name": name}, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}

type budgetInput struct {
	Amount *float64 `json:"amount"`
}

// SetBudget — maliyet kodunun planlanan tutarını yazar (yoksa oluşturur, varsa günceller).
// Yalnız giriş; EVM, sapma veya PO/hakediş hesabına bağlı değildir.
func (h *Handler) SetBudget(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	cid, ok := parseID(w, r, "codeID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	var in budgetInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.Amount == nil || *in.Amount < 0 || *in.Amount > 999999999999 {
		httpx.ValidationFailed(w, r, map[string]string{"amount": "0 veya daha büyük bir tutar girin"})
		return
	}
	var one int
	if err := h.pool.QueryRow(r.Context(), `
		SELECT 1 FROM cost_codes WHERE id=$1 AND project_id=$2`, cid, pid).Scan(&one); err != nil {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Maliyet kodu bulunamadı.", nil)
		return
	}
	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (cost_code_id) DO UPDATE
		  SET amount=EXCLUDED.amount, updated_by=EXCLUDED.updated_by, updated_at=now()`,
		cid, pid, *in.Amount, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "cost_code_budgets", EntityID: cid.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"amount": *in.Amount}, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"cost_code_id": cid, "amount": *in.Amount})
}
