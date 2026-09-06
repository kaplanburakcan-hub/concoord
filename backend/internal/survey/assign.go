package survey

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

// Bir keşif kalemi bir taşerona atandığında, o an ki poz_no/tanım/birim/
// miktar/birim_fiyat değerleriyle GERÇEK bir work_items (hakediş poz) satırı
// oluşturulur ve work_item_id ile buraya bağlanır. Keşif master BOQ listesi
// olarak kalır; work_items ondan bağımsız olarak hakediş akışında revize
// edilebilir.

type assignReq struct {
	SubcontractorID string  `json:"subcontractor_id"`
	ContractID      *string `json:"contract_id"`
}

type assignResp struct {
	WorkItemID string `json:"work_item_id"`
}

func (h *Handler) AssignSubcontractor(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kalem ID.", nil)
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}

	var body assignReq
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	subID, err := uuid.Parse(body.SubcontractorID)
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "Geçerli bir taşeron seçin."})
		return
	}
	var contractID *uuid.UUID
	if body.ContractID != nil && strings.TrimSpace(*body.ContractID) != "" {
		cid, err := uuid.Parse(*body.ContractID)
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"contract_id": "Geçersiz sözleşme."})
			return
		}
		contractID = &cid
	}

	ctx := r.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(ctx)

	var item Item
	var workItemID *string
	if err := tx.QueryRow(ctx, `
		SELECT kategori, COALESCE(poz_no,''), tanim, birim, miktar, birim_fiyat, work_item_id
		FROM project_survey_items WHERE id=$1 AND project_id=$2 FOR UPDATE`,
		itemID, pid,
	).Scan(&item.Kategori, &item.PozNo, &item.Tanim, &item.Birim, &item.Miktar, &item.BirimFiyat, &workItemID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Keşif kalemi bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if workItemID != nil {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu kalem zaten bir taşerona atanmış. Önce kaldırın.", nil)
		return
	}
	if strings.TrimSpace(item.PozNo) == "" {
		httpx.ValidationFailed(w, r, map[string]string{"poz_no": "Atama yapabilmek için önce keşif kaleminde poz numarası girilmeli."})
		return
	}

	var subExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM subcontractors WHERE id=$1 AND project_id=$2)`, subID, pid,
	).Scan(&subExists); err != nil || !subExists {
		httpx.ValidationFailed(w, r, map[string]string{"subcontractor_id": "Bu taşeron bu projede bulunamadı."})
		return
	}

	var newWorkItemID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO work_items (project_id, subcontractor_id, contract_id, poz_no, description, unit, contract_qty, unit_price)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id`,
		pid, subID, contractID, item.PozNo, item.Tanim, item.Birim, item.Miktar, item.BirimFiyat,
	).Scan(&newWorkItemID); err != nil {
		if strings.Contains(err.Error(), "uq_work_items_poz") {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Bu taşeron için bu poz numarası zaten tanımlı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	if _, err := tx.Exec(ctx,
		`UPDATE project_survey_items SET work_item_id=$1, updated_at=now() WHERE id=$2`, newWorkItemID, itemID,
	); err != nil {
		httpx.Internal(w, r)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		httpx.Internal(w, r)
		return
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: uid.String(), Entity: "work_items", EntityID: newWorkItemID, Action: audit.ActionInsert,
		After: map[string]any{"poz_no": item.PozNo, "kaynak": "proje_kesfi", "survey_item_id": itemID},
		IP:    meta.IP, ReqID: meta.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, assignResp{WorkItemID: newWorkItemID})
}

// Unassign — keşif kalemindeki taşeron bağını çözer. work_items satırı
// SİLİNMEZ (hakediş zaten referans veriyor olabilir), yalnızca bağ kaldırılır.
func (h *Handler) Unassign(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz proje ID.", nil)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kalem ID.", nil)
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`UPDATE project_survey_items SET work_item_id=NULL, updated_at=now() WHERE id=$1 AND project_id=$2`,
		itemID, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kalem bulunamadı.", nil)
		return
	}

	meta := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: uid.String(), Entity: "project_survey_items", EntityID: itemID.String(), Action: audit.ActionUpdate,
		After: map[string]any{"work_item_id": nil}, IP: meta.IP, ReqID: meta.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
