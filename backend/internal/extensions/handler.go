// Package extensions — süre uzatımı (EOT) talep taslakları (Plan E2, ilk adım).
//
// Bu aşamada yalnızca taslak kayıt vardır: onay, sözleşme bitiş tarihi ve iş
// programı etkisi sonraki adımlarda eklenecektir. Kayıt silinmez.
package extensions

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

type DraftDTO struct {
	ID            uuid.UUID `json:"id"`
	No            int       `json:"no"`
	Title         string    `json:"title"`
	ReasonCode    string    `json:"reason_code"`
	RequestedDays int       `json:"requested_days"`
	Basis         *string   `json:"basis,omitempty"`
	Status        string    `json:"status"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     string    `json:"created_at"`
}

var reasons = map[string]bool{"Sahip": true, "Hava": true, "Tasarim": true, "MucbirSebep": true, "Diger": true}

type draftInput struct {
	Title         string  `json:"title"`
	ReasonCode    string  `json:"reason_code"`
	RequestedDays int     `json:"requested_days"`
	Basis         *string `json:"basis"`
}

const draftSelect = `
	SELECT t.id, t.draft_no, t.title, t.reason_code, t.requested_days, t.basis, t.status,
	       u.full_name, to_char(t.created_at,'YYYY-MM-DD')
	FROM time_extension_drafts t
	JOIN users u ON u.id = t.created_by`

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), draftSelect+`
		WHERE t.project_id=$1 ORDER BY t.draft_no DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []DraftDTO{}
	for rows.Next() {
		var d DraftDTO
		if err := rows.Scan(&d.ID, &d.No, &d.Title, &d.ReasonCode, &d.RequestedDays, &d.Basis, &d.Status,
			&d.CreatedByName, &d.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"drafts": out})
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
	var in draftInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	errs := map[string]string{}
	t := strings.TrimSpace(in.Title)
	if t == "" {
		errs["title"] = "başlık zorunlu"
	} else if len([]rune(t)) > 200 {
		errs["title"] = "en fazla 200 karakter"
	}
	if !reasons[in.ReasonCode] {
		errs["reason_code"] = "geçerli bir neden seçin"
	}
	if in.RequestedDays < 1 || in.RequestedDays > 3650 {
		errs["requested_days"] = "1 ile 3650 gün arasında olmalı"
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	var id uuid.UUID
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO time_extension_drafts (project_id, draft_no, title, reason_code, requested_days, basis, created_by)
		VALUES ($1,
		        (SELECT COALESCE(max(draft_no),0)+1 FROM time_extension_drafts WHERE project_id=$1),
		        $2,$3,$4,NULLIF(btrim(coalesce($5,'')),''),$6)
		RETURNING id`,
		pid, t, in.ReasonCode, in.RequestedDays, in.Basis, uid).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "time_extension_drafts", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"title": t, "requested_days": in.RequestedDays, "status": "Taslak"},
		IP:    m.IP, ReqID: m.ReqID,
	})
	var d DraftDTO
	if err := rowScan(h, r, pid, id, &d); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"draft": d})
}

func rowScan(h *Handler, r *http.Request, pid, id uuid.UUID, d *DraftDTO) error {
	return h.pool.QueryRow(r.Context(), draftSelect+` WHERE t.id=$1 AND t.project_id=$2`, id, pid).
		Scan(&d.ID, &d.No, &d.Title, &d.ReasonCode, &d.RequestedDays, &d.Basis, &d.Status,
			&d.CreatedByName, &d.CreatedAt)
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}
