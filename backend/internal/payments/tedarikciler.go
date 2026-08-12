package payments

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// ---------------------------------------------------------------------------
// Tedarikçiler — subcontractors ile birebir aynı şekil, ayrı tablo (bkz.
// migration 000037). Taşeron'dan (subcontractors) bilinçli olarak ayrı:
// subcontractors 9 modüle FK ile bağlı, Tedarikçi ise şimdilik yalnızca
// Taşeron-Tedarikçi Sözleşmeleri sayfasında/Taşeron Dashboard'da gösteriliyor.
// ---------------------------------------------------------------------------

type tedarikciDTO struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	CompanyName   string    `json:"company_name"`
	TaxNo         *string   `json:"tax_no,omitempty"`
	ContactPerson *string   `json:"contact_person,omitempty"`
	Phone         *string   `json:"phone,omitempty"`
	Email         *string   `json:"email,omitempty"`
	Trade         *string   `json:"trade,omitempty"`
	RowVersion    int       `json:"row_version"`
	CreatedAt     time.Time `json:"created_at"`
}

const tedCols = `id, project_id, company_name, tax_no, contact_person, phone, email, trade, row_version, created_at`

func scanTed(row pgx.Row, t *tedarikciDTO) error {
	return row.Scan(&t.ID, &t.ProjectID, &t.CompanyName, &t.TaxNo, &t.ContactPerson,
		&t.Phone, &t.Email, &t.Trade, &t.RowVersion, &t.CreatedAt)
}

func (h *Handler) ListTedarikciler(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT `+tedCols+` FROM tedarikciler
		WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY company_name`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []tedarikciDTO{}
	for rows.Next() {
		var t tedarikciDTO
		if err := scanTed(rows, &t); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, t)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"tedarikciler": out})
}

type tedReq struct {
	CompanyName   string  `json:"company_name"`
	TaxNo         *string `json:"tax_no"`
	ContactPerson *string `json:"contact_person"`
	Phone         *string `json:"phone"`
	Email         *string `json:"email"`
	Trade         *string `json:"trade"`
	RowVersion    int     `json:"row_version"`
}

func (h *Handler) CreateTedarikci(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req tedReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	if f := ValidateSubcontractor(req.CompanyName); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var t tedarikciDTO
	err = scanTed(tx.QueryRow(r.Context(), `
		INSERT INTO tedarikciler (project_id, company_name, tax_no, contact_person, phone, email, trade)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+tedCols,
		pid, req.CompanyName, req.TaxNo, req.ContactPerson, req.Phone, req.Email, req.Trade), &t)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Proje bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "tedarikciler", EntityID: t.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid, "company_name": t.CompanyName}, IP: m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"tedarikci": t})
}

func (h *Handler) GetTedarikci(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	tid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var t tedarikciDTO
	err := scanTed(h.pool.QueryRow(r.Context(),
		`SELECT `+tedCols+` FROM tedarikciler WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, tid, pid), &t)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Tedarikçi bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"tedarikci": t})
}

func (h *Handler) UpdateTedarikci(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	tid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req tedReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var before tedarikciDTO
	err = scanTed(tx.QueryRow(r.Context(),
		`SELECT `+tedCols+` FROM tedarikciler WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, tid, pid), &before)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Tedarikçi bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != before.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": before.RowVersion})
		return
	}
	name := before.CompanyName
	if strings.TrimSpace(req.CompanyName) != "" {
		name = strings.TrimSpace(req.CompanyName)
	}
	if f := ValidateSubcontractor(name); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	var t tedarikciDTO
	err = scanTed(tx.QueryRow(r.Context(), `
		UPDATE tedarikciler SET
			company_name=$3,
			tax_no=COALESCE($4, tax_no),
			contact_person=COALESCE($5, contact_person),
			phone=COALESCE($6, phone),
			email=COALESCE($7, email),
			trade=COALESCE($8, trade),
			row_version=row_version+1
		WHERE id=$1 AND project_id=$2
		RETURNING `+tedCols,
		tid, pid, name, req.TaxNo, req.ContactPerson, req.Phone, req.Email, req.Trade), &t)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "tedarikciler", EntityID: tid.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"company_name": before.CompanyName},
		After:  map[string]interface{}{"company_name": t.CompanyName}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"tedarikci": t})
}

func (h *Handler) DeleteTedarikci(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	tid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE tedarikciler SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, tid, pid)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Tedarikçiye bağlı kayıt var; önce onları arşivleyin.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Tedarikçi bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "tedarikciler", EntityID: tid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
