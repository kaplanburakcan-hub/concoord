// Package risks — proje risk kaydı (Plan E3, ilk sürüm).
//
// Kurallar:
//   - Risk silinmez; yalnızca durumu değişir (Açık → İzleniyor → Kapandı).
//   - Skor = olasılık × etki; liste skora göre sıralanır.
//   - Her yazma işlemi audit kaydına yazılır.
package risks

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

// RiskDTO — bir risk kaydı. Score, sorgu içinde olasılık × etki olarak hesaplanır.
type RiskDTO struct {
	ID          uuid.UUID  `json:"id"`
	No          int        `json:"no"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Probability int        `json:"probability"`
	Impact      int        `json:"impact"`
	Score       int        `json:"score"`
	OwnerID     *uuid.UUID `json:"owner_id,omitempty"`
	OwnerName   *string    `json:"owner_name,omitempty"`
	Status      string     `json:"status"`
	RowVersion  int        `json:"row_version"`
}

const riskSelect = `
	SELECT r.id, r.risk_no, r.title, r.description, r.probability, r.impact,
	       r.probability * r.impact, r.owner_id, u.full_name, r.status, r.row_version
	FROM project_risks r
	LEFT JOIN users u ON u.id = r.owner_id`

var statuses = map[string]bool{"Açık": true, "İzleniyor": true, "Kapandı": true}

func scanRisk(row pgx.Row, d *RiskDTO) error {
	return row.Scan(&d.ID, &d.No, &d.Title, &d.Description, &d.Probability, &d.Impact,
		&d.Score, &d.OwnerID, &d.OwnerName, &d.Status, &d.RowVersion)
}

type riskInput struct {
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Probability *int       `json:"probability"`
	Impact      *int       `json:"impact"`
	OwnerID     *uuid.UUID `json:"owner_id"`
	Status      *string    `json:"status"`
}

// validate — oluşturma için zorunlu alanları, güncellemede verilen alanları denetler.
func validate(in riskInput, create bool) map[string]string {
	errs := map[string]string{}
	if create || in.Title != nil {
		t := ""
		if in.Title != nil {
			t = strings.TrimSpace(*in.Title)
		}
		if t == "" {
			errs["title"] = "başlık zorunlu"
		} else if len([]rune(t)) > 200 {
			errs["title"] = "en fazla 200 karakter"
		}
	}
	if create && in.Probability == nil {
		errs["probability"] = "olasılık zorunlu (1-5)"
	}
	if in.Probability != nil && (*in.Probability < 1 || *in.Probability > 5) {
		errs["probability"] = "1 ile 5 arasında olmalı"
	}
	if create && in.Impact == nil {
		errs["impact"] = "etki zorunlu (1-5)"
	}
	if in.Impact != nil && (*in.Impact < 1 || *in.Impact > 5) {
		errs["impact"] = "1 ile 5 arasında olmalı"
	}
	if in.Status != nil && !statuses[*in.Status] {
		errs["status"] = "geçersiz durum"
	}
	return errs
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), riskSelect+`
		WHERE r.project_id=$1 AND r.deleted_at IS NULL
		ORDER BY (r.status='Kapandı'), r.probability*r.impact DESC, r.risk_no`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []RiskDTO{}
	for rows.Next() {
		var d RiskDTO
		if err := scanRisk(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"risks": out})
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
	var in riskInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := validate(in, true); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	status := "Açık"
	if in.Status != nil {
		status = *in.Status
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO project_risks (project_id, risk_no, title, description, probability, impact,
		                           owner_id, status, created_by)
		VALUES ($1,
		        (SELECT COALESCE(max(risk_no),0)+1 FROM project_risks WHERE project_id=$1),
		        $2, NULLIF(btrim(coalesce($3,'')),''), $4, $5, $6, $7, $8)
		RETURNING id`,
		pid, strings.TrimSpace(*in.Title), in.Description, *in.Probability, *in.Impact,
		in.OwnerID, status, uid).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "project_risks", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"title": in.Title, "probability": in.Probability, "impact": in.Impact, "status": status},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	h.respond(w, r, pid, id, http.StatusCreated)
}

// Update — kısmi güncelleme. Silme yoktur; durum geçişi de bu uçtan yapılır.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var in riskInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := validate(in, false); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	var before string
	if err := h.pool.QueryRow(r.Context(), `
		SELECT status FROM project_risks WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		id, pid).Scan(&before); errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Risk bulunamadı.", nil)
		return
	} else if err != nil {
		httpx.Internal(w, r)
		return
	}
	// Boş/eksik alanlar mevcut değeri korur (COALESCE). Açıklama boşaltılabilsin
	// diye description için ayrı bir bayrak kullanılmaz: boş metin NULL'a döner.
	_, err := h.pool.Exec(r.Context(), `
		UPDATE project_risks SET
		  title       = COALESCE(NULLIF(btrim($3),''), title),
		  description = CASE WHEN $4::text IS NULL THEN description ELSE NULLIF(btrim($4),'') END,
		  probability = COALESCE($5, probability),
		  impact      = COALESCE($6, impact),
		  owner_id    = COALESCE($7, owner_id),
		  status      = COALESCE($8, status),
		  row_version = row_version + 1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		id, pid, ptrStr(in.Title), in.Description, in.Probability, in.Impact, in.OwnerID, in.Status)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	after := map[string]interface{}{}
	if in.Status != nil {
		after["status"] = *in.Status
	}
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "project_risks", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": before}, After: after,
		IP: m.IP, ReqID: m.ReqID,
	})
	h.respond(w, r, pid, id, http.StatusOK)
}

func ptrStr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, pid, id uuid.UUID, code int) {
	var d RiskDTO
	if err := scanRisk(h.pool.QueryRow(r.Context(), riskSelect+`
		WHERE r.id=$1 AND r.project_id=$2`, id, pid), &d); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, code, map[string]interface{}{"risk": d})
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", nil)
		return uuid.Nil, false
	}
	return id, true
}
