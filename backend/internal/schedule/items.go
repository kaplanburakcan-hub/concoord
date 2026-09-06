package schedule

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
)

var (
	ErrConflict     = errors.New("satır değişmiş (row_version uyuşmuyor)")
	ErrHasChildren  = errors.New("alt kalemleri olan bir kalem silinemez")
	ErrNotFound     = errors.New("kayıt bulunamadı")
	ErrCrossProject = errors.New("poz başka bir projeye ait")
)

type ItemDTO struct {
	ID             uuid.UUID  `json:"id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	ParentID       *uuid.UUID `json:"parent_id"`
	WbsCode        string     `json:"wbs_code"`
	Name           string     `json:"name"`
	SortOrder      int        `json:"sort_order"`
	IsMilestone    bool       `json:"is_milestone"`
	BaselineStart  *string    `json:"baseline_start"`
	BaselineFinish *string    `json:"baseline_finish"`
	ActualStart    *string    `json:"actual_start"`
	ActualFinish   *string    `json:"actual_finish"`
	ProgressSource string     `json:"progress_source"`
	ManualProgress *float64   `json:"manual_progress"`
	Weight         float64    `json:"weight"`
	Progress       float64    `json:"progress"` // ComputeProgress'ten — HER ZAMAN istek anında hesaplanır
	RowVersion     int        `json:"row_version"`
}

const itemCols = `
	id, project_id, parent_id, wbs_code, name, sort_order, is_milestone,
	to_char(baseline_start,'YYYY-MM-DD'), to_char(baseline_finish,'YYYY-MM-DD'),
	to_char(actual_start,'YYYY-MM-DD'), to_char(actual_finish,'YYYY-MM-DD'),
	progress_source, manual_progress, weight::float8, row_version`

func scanItem(row pgx.Row, it *ItemDTO) error {
	return row.Scan(&it.ID, &it.ProjectID, &it.ParentID, &it.WbsCode, &it.Name, &it.SortOrder, &it.IsMilestone,
		&it.BaselineStart, &it.BaselineFinish, &it.ActualStart, &it.ActualFinish,
		&it.ProgressSource, &it.ManualProgress, &it.Weight, &it.RowVersion)
}

// ListItems — projenin tüm WBS ağacını, HER kalem için istek anında
// hesaplanmış ilerleme yüzdesiyle döner (bkz. progress.go — cache/materialized
// view yok).
func (h *Handler) ListItems(ctx context.Context, projectID uuid.UUID) ([]ItemDTO, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT `+itemCols+`
		FROM schedule_items WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY wbs_code`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ItemDTO{}
	for rows.Next() {
		var it ItemDTO
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	agg, err := h.LoadItemAggregates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	progress := ComputeProgress(agg)
	for i := range out {
		out[i].Progress = progress[out[i].ID]
	}
	return out, nil
}

type ItemReq struct {
	ParentID       *string  `json:"parent_id"`
	WbsCode        string   `json:"wbs_code"`
	Name           string   `json:"name"`
	SortOrder      int      `json:"sort_order"`
	IsMilestone    bool     `json:"is_milestone"`
	BaselineStart  *string  `json:"baseline_start"`
	BaselineFinish *string  `json:"baseline_finish"`
	ActualStart    *string  `json:"actual_start"`
	ActualFinish   *string  `json:"actual_finish"`
	ProgressSource string   `json:"progress_source"`
	ManualProgress *float64 `json:"manual_progress"`
	Weight         *float64 `json:"weight"`
}

// Validate — alan bazlı doğrulama hataları (boşsa geçerli).
func (req ItemReq) Validate() map[string]string {
	f := map[string]string{}
	if strings.TrimSpace(req.WbsCode) == "" {
		f["wbs_code"] = "Zorunlu."
	}
	if strings.TrimSpace(req.Name) == "" {
		f["name"] = "Zorunlu."
	}
	src := req.ProgressSource
	if src == "" {
		src = "manual"
	}
	if src != "manual" && src != "derived" {
		f["progress_source"] = "'manual' veya 'derived' olmalı."
	}
	if req.ManualProgress != nil && (*req.ManualProgress < 0 || *req.ManualProgress > 100) {
		f["manual_progress"] = "0 ile 100 arasında olmalı."
	}
	return f
}

func (h *Handler) CreateItem(ctx context.Context, projectID, actorID uuid.UUID, req ItemReq) (*ItemDTO, error) {
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		pid, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, errors.New("geçersiz parent_id")
		}
		parentID = &pid
	}
	progressSource := req.ProgressSource
	if progressSource == "" {
		progressSource = "manual"
	}
	weight := 0.0
	if req.Weight != nil {
		weight = *req.Weight
	}

	var it ItemDTO
	err := scanItem(h.pool.QueryRow(ctx, `
		INSERT INTO schedule_items
			(project_id, parent_id, wbs_code, name, sort_order, is_milestone,
			 baseline_start, baseline_finish, actual_start, actual_finish,
			 progress_source, manual_progress, weight)
		VALUES ($1,$2,$3,$4,$5,$6,$7::date,$8::date,$9::date,$10::date,$11,$12,$13)
		RETURNING `+itemCols,
		projectID, parentID, strings.TrimSpace(req.WbsCode), strings.TrimSpace(req.Name),
		req.SortOrder, req.IsMilestone, req.BaselineStart, req.BaselineFinish,
		req.ActualStart, req.ActualFinish, progressSource, req.ManualProgress, weight,
	), &it)
	if err != nil {
		return nil, err
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_items", EntityID: it.ID.String(), Action: audit.ActionInsert,
		After: map[string]any{"wbs_code": it.WbsCode, "name": it.Name}, IP: meta.IP, ReqID: meta.ReqID,
	})
	return &it, nil
}

func (h *Handler) UpdateItem(ctx context.Context, itemID, actorID uuid.UUID, rowVersion int, req ItemReq) (*ItemDTO, error) {
	var parentID *uuid.UUID
	if req.ParentID != nil && *req.ParentID != "" {
		pid, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, errors.New("geçersiz parent_id")
		}
		parentID = &pid
	}
	progressSource := req.ProgressSource
	if progressSource == "" {
		progressSource = "manual"
	}
	weight := 0.0
	if req.Weight != nil {
		weight = *req.Weight
	}

	var it ItemDTO
	row := h.pool.QueryRow(ctx, `
		UPDATE schedule_items SET
			parent_id=$1, wbs_code=$2, name=$3, sort_order=$4, is_milestone=$5,
			baseline_start=$6::date, baseline_finish=$7::date,
			actual_start=$8::date, actual_finish=$9::date,
			progress_source=$10, manual_progress=$11, weight=$12,
			row_version=row_version+1
		WHERE id=$13 AND row_version=$14 AND deleted_at IS NULL
		RETURNING `+itemCols,
		parentID, strings.TrimSpace(req.WbsCode), strings.TrimSpace(req.Name),
		req.SortOrder, req.IsMilestone, req.BaselineStart, req.BaselineFinish,
		req.ActualStart, req.ActualFinish, progressSource, req.ManualProgress, weight,
		itemID, rowVersion)
	if err := scanItem(row, &it); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrConflict
		}
		return nil, err
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_items", EntityID: it.ID.String(), Action: audit.ActionUpdate,
		After: map[string]any{"wbs_code": it.WbsCode, "name": it.Name}, IP: meta.IP, ReqID: meta.ReqID,
	})
	return &it, nil
}

// DeleteItem — soft delete. Alt kalemi olan bir kalem silinemez (kullanıcı
// önce alt kalemleri silmeli/taşımalı) — sessizce yetim bırakmaktansa
// açık bir hata daha güvenli.
func (h *Handler) DeleteItem(ctx context.Context, itemID, actorID uuid.UUID) error {
	var childCount int
	if err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM schedule_items WHERE parent_id=$1 AND deleted_at IS NULL`, itemID,
	).Scan(&childCount); err != nil {
		return err
	}
	if childCount > 0 {
		return ErrHasChildren
	}

	tag, err := h.pool.Exec(ctx,
		`UPDATE schedule_items SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, itemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_items", EntityID: itemID.String(), Action: audit.ActionDelete,
		IP: meta.IP, ReqID: meta.ReqID,
	})
	return nil
}

// LinkPoz — bir WBS kalemini bir sözleşme pozuna (work_items) bağlar.
// Poz, kalemle AYNI projeye ait olmalı.
func (h *Handler) LinkPoz(ctx context.Context, itemID, pozID, actorID uuid.UUID) error {
	var itemProject, pozProject uuid.UUID
	if err := h.pool.QueryRow(ctx,
		`SELECT project_id FROM schedule_items WHERE id=$1 AND deleted_at IS NULL`, itemID,
	).Scan(&itemProject); err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if err := h.pool.QueryRow(ctx,
		`SELECT project_id FROM work_items WHERE id=$1 AND deleted_at IS NULL`, pozID,
	).Scan(&pozProject); err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if itemProject != pozProject {
		return ErrCrossProject
	}

	if _, err := h.pool.Exec(ctx, `
		INSERT INTO schedule_item_pozlar (schedule_item_id, poz_id)
		VALUES ($1,$2) ON CONFLICT DO NOTHING`, itemID, pozID); err != nil {
		return err
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_item_pozlar", EntityID: itemID.String(), Action: audit.ActionInsert,
		After: map[string]any{"poz_id": pozID}, IP: meta.IP, ReqID: meta.ReqID,
	})
	return nil
}

type AvailablePozDTO struct {
	ID              uuid.UUID `json:"id"`
	PozNo           string    `json:"poz_no"`
	Description     string    `json:"description"`
	Unit            string    `json:"unit"`
	ContractQty     float64   `json:"contract_qty"`
	SubcontractorAd string    `json:"subcontractor_adi"`
}

// ListAvailablePozlar — DÜZELTME: spec'te yoktu, poz bağlama arayüzünün
// seçim listesi için gerekli. work_items normalde taşeron bazlı listelenir
// (internal/payments), burada proje çapında tek listede toplanır.
func (h *Handler) ListAvailablePozlar(ctx context.Context, projectID uuid.UUID) ([]AvailablePozDTO, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT wi.id, wi.poz_no, wi.description, wi.unit, wi.contract_qty::float8, s.company_name
		FROM work_items wi
		JOIN subcontractors s ON s.id = wi.subcontractor_id
		WHERE wi.project_id = $1 AND wi.deleted_at IS NULL
		ORDER BY wi.poz_no`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AvailablePozDTO{}
	for rows.Next() {
		var p AvailablePozDTO
		if err := rows.Scan(&p.ID, &p.PozNo, &p.Description, &p.Unit, &p.ContractQty, &p.SubcontractorAd); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (h *Handler) UnlinkPoz(ctx context.Context, itemID, pozID, actorID uuid.UUID) error {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM schedule_item_pozlar WHERE schedule_item_id=$1 AND poz_id=$2`, itemID, pozID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_item_pozlar", EntityID: itemID.String(), Action: audit.ActionDelete,
		After: map[string]any{"poz_id": pozID}, IP: meta.IP, ReqID: meta.ReqID,
	})
	return nil
}
