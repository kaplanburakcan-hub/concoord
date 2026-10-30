// Package decisions — proje karar günlüğü (Plan E3).
//
// Kararlar değiştirilmez ve silinmez (veritabanı tetikleyicisi de bunu zorlar).
// Bir kararı değiştirmek için yeni bir kayıt, eskisini supersedes_id ile geçersiz
// kılar. Aktif kararlar, başka bir kayıtça geçersiz kılınmamış olanlardır.
package decisions

import (
	"errors"
	"net/http"
	"strings"
	"time"

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

// DecisionDTO — bir karar kaydı. Active=false ise sonraki bir kayıt tarafından geçersiz kılınmıştır.
type DecisionDTO struct {
	ID            uuid.UUID  `json:"id"`
	Text          string     `json:"text"`
	DecidedOn     string     `json:"decided_on"`
	DecidedBy     *uuid.UUID `json:"decided_by,omitempty"`
	DecidedByName *string    `json:"decided_by_name,omitempty"`
	MeetingID     *uuid.UUID `json:"meeting_id,omitempty"`
	SupersedesID  *uuid.UUID `json:"supersedes_id,omitempty"`
	SupersededBy  *uuid.UUID `json:"superseded_by,omitempty"`
	Active        bool       `json:"active"`
	CreatedByName string     `json:"created_by_name"`
	CreatedAt     string     `json:"created_at"`
}

const decisionSelect = `
	SELECT d.id, d.decision_text, to_char(d.decided_on,'YYYY-MM-DD'), d.decided_by, du.full_name,
	       d.meeting_id, d.supersedes_id,
	       (SELECT n.id FROM project_decisions n WHERE n.supersedes_id = d.id),
	       cu.full_name, to_char(d.created_at,'YYYY-MM-DD HH24:MI')
	FROM project_decisions d
	JOIN users cu ON cu.id = d.created_by
	LEFT JOIN users du ON du.id = d.decided_by`

func scanDecision(row pgx.Row, d *DecisionDTO) error {
	var superseded *uuid.UUID
	if err := row.Scan(&d.ID, &d.Text, &d.DecidedOn, &d.DecidedBy, &d.DecidedByName,
		&d.MeetingID, &d.SupersedesID, &superseded, &d.CreatedByName, &d.CreatedAt); err != nil {
		return err
	}
	d.SupersededBy = superseded
	d.Active = superseded == nil
	return nil
}

type decisionInput struct {
	Text         string     `json:"text"`
	DecidedOn    string     `json:"decided_on"`
	DecidedBy    *uuid.UUID `json:"decided_by"`
	MeetingID    *uuid.UUID `json:"meeting_id"`
	SupersedesID *uuid.UUID `json:"supersedes_id"`
}

func validate(in decisionInput) map[string]string {
	errs := map[string]string{}
	t := strings.TrimSpace(in.Text)
	if t == "" {
		errs["text"] = "karar metni zorunlu"
	} else if len([]rune(t)) > 2000 {
		errs["text"] = "en fazla 2000 karakter"
	}
	if _, err := time.Parse("2006-01-02", in.DecidedOn); err != nil {
		errs["decided_on"] = "geçerli bir tarih (YYYY-MM-DD) girin"
	}
	return errs
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), decisionSelect+`
		WHERE d.project_id=$1
		ORDER BY d.decided_on DESC, d.created_at DESC`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []DecisionDTO{}
	for rows.Next() {
		var d DecisionDTO
		if err := scanDecision(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"decisions": out})
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
	var in decisionInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := validate(in); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	// Geçersiz kılınan ve bağlanan toplantı aynı projeye ait olmalı; veritabanı
	// tetikleyicisi geçersiz kılmayı zaten denetler, toplantı için burada bakılır.
	if in.MeetingID != nil {
		var one int
		if err := h.pool.QueryRow(r.Context(), `
			SELECT 1 FROM project_meetings WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
			*in.MeetingID, pid).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
			httpx.ValidationFailed(w, r, map[string]string{"meeting_id": "bu projede kayıtlı toplantı değil"})
			return
		} else if err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO project_decisions (project_id, meeting_id, decision_text, decided_by, decided_on,
		                               supersedes_id, created_by)
		VALUES ($1,$2,$3,$4,$5::date,$6,$7) RETURNING id`,
		pid, in.MeetingID, strings.TrimSpace(in.Text), in.DecidedBy, in.DecidedOn, in.SupersedesID, uid).Scan(&id)
	if err != nil {
		// Aynı kararı ikinci kez geçersiz kılma (benzersiz indeks) ve başka projeye ait
		// geçersiz kılma (tetikleyici) durumları kullanıcıya açıklanır.
		msg := err.Error()
		switch {
		case strings.Contains(msg, "uq_project_decision_supersedes"):
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Bu karar zaten başka bir kararla geçersiz kılınmış.", nil)
		case strings.Contains(msg, "başka projeye ait"):
			httpx.ValidationFailed(w, r, map[string]string{"supersedes_id": "bu projede kayıtlı karar değil"})
		default:
			httpx.Internal(w, r)
		}
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "project_decisions", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"decided_on": in.DecidedOn, "supersedes_id": in.SupersedesID},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	var d DecisionDTO
	if err := scanDecision(h.pool.QueryRow(r.Context(), decisionSelect+` WHERE d.id=$1`, id), &d); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"decision": d})
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}
