// Package ncr — uygunsuzluk (NCR) kaydı, ilk sürüm (Plan E4).
//
// Kayıt silinmez. Kapatma, durumu 'Kapandı' yapar ve kapanış bilgisini tutar;
// kapalı bir kayıt yeniden açılamaz (yeni bir kayıt açılır).
package ncr

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// NCRDTO — bir uygunsuzluk kaydı.
type NCRDTO struct {
	ID            uuid.UUID `json:"id"`
	No            int       `json:"no"`
	Title         string    `json:"title"`
	PozNo         *string   `json:"poz_no,omitempty"`
	Description   *string   `json:"description,omitempty"`
	OwnerName     *string   `json:"owner_name,omitempty"`
	Status        string    `json:"status"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     string    `json:"created_at"`
	ClosedByName  *string   `json:"closed_by_name,omitempty"`
	ClosedAt      *string   `json:"closed_at,omitempty"`
}

const ncrSelect = `
	SELECT n.id, n.ncr_no, n.title, n.poz_no, n.description, ou.full_name, n.status,
	       cu.full_name, to_char(n.created_at,'YYYY-MM-DD HH24:MI'),
	       clu.full_name, to_char(n.closed_at,'YYYY-MM-DD HH24:MI')
	FROM ncrs n
	JOIN users cu ON cu.id = n.created_by
	LEFT JOIN users ou ON ou.id = n.owner_id
	LEFT JOIN users clu ON clu.id = n.closed_by`

func scanNCR(row pgx.Row, d *NCRDTO) error {
	return row.Scan(&d.ID, &d.No, &d.Title, &d.PozNo, &d.Description, &d.OwnerName, &d.Status,
		&d.CreatedByName, &d.CreatedAt, &d.ClosedByName, &d.ClosedAt)
}

type ncrInput struct {
	Title       string     `json:"title"`
	PozNo       *string    `json:"poz_no"`
	Description *string    `json:"description"`
	OwnerID     *uuid.UUID `json:"owner_id"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), ncrSelect+`
		WHERE n.project_id=$1
		ORDER BY (n.status='Kapandı'), n.ncr_no DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []NCRDTO{}
	for rows.Next() {
		var d NCRDTO
		if err := scanNCR(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"ncrs": out})
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
	var in ncrInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	t := strings.TrimSpace(in.Title)
	errs := map[string]string{}
	if t == "" {
		errs["title"] = "başlık zorunlu"
	} else if len([]rune(t)) > 200 {
		errs["title"] = "en fazla 200 karakter"
	}
	if len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO ncrs (project_id, ncr_no, title, poz_no, description, owner_id, created_by)
		VALUES ($1,
		        (SELECT COALESCE(max(ncr_no),0)+1 FROM ncrs WHERE project_id=$1),
		        $2, NULLIF(btrim(coalesce($3,'')),''), NULLIF(btrim(coalesce($4,'')),''), $5, $6)
		RETURNING id`,
		pid, t, in.PozNo, in.Description, in.OwnerID, uid).Scan(&id); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "ncrs", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"title": t, "status": "Açık"}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	h.respond(w, r, pid, id, http.StatusCreated)
}

// Close — açık kaydı kapatır. Kapalı kayıt tekrar kapatılamaz ve açılamaz.
func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE ncrs SET status='Kapandı', closed_by=$3, closed_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND status='Açık'`, id, pid, uid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		var one int
		err := h.pool.QueryRow(r.Context(), `SELECT 1 FROM ncrs WHERE id=$1 AND project_id=$2`, id, pid).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Uygunsuzluk kaydı bulunamadı.", nil)
			return
		}
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Kayıt zaten kapalı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "ncrs", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": "Açık"}, After: map[string]interface{}{"status": "Kapandı"},
		IP: m.IP, ReqID: m.ReqID,
	})
	h.respond(w, r, pid, id, http.StatusOK)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, pid, id uuid.UUID, code int) {
	var d NCRDTO
	if err := scanNCR(h.pool.QueryRow(r.Context(), ncrSelect+` WHERE n.id=$1 AND n.project_id=$2`, id, pid), &d); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, code, map[string]interface{}{"ncr": d})
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}
