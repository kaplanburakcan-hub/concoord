package schedule

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
)

type BaselineDTO struct {
	ID         uuid.UUID `json:"id"`
	ProjectID  uuid.UUID `json:"project_id"`
	RevisionNo int       `json:"revision_no"`
	FrozenAt   string    `json:"frozen_at"`
	FrozenBy   uuid.UUID `json:"frozen_by"`
	Note       *string   `json:"note"`
}

type BaselineDetailDTO struct {
	BaselineDTO
	Snapshot json.RawMessage `json:"snapshot"`
}

// baselineSnapshotItem — dondurma anındaki bir WBS kaleminin durumu.
// Kasıtlı olarak schedule_items'tan BAĞIMSIZ bir kopyadır: kalem daha
// sonra değişse (tarih, ilerleme, hatta silinse) bile bu JSON değişmez —
// "Baseline dondurulduktan sonra tarih değişse bile dondurulmuş revizyon
// değişmiyor" kabul kriteri budur.
type baselineSnapshotItem struct {
	ID             uuid.UUID  `json:"id"`
	ParentID       *uuid.UUID `json:"parent_id"`
	WbsCode        string     `json:"wbs_code"`
	Name           string     `json:"name"`
	IsMilestone    bool       `json:"is_milestone"`
	BaselineStart  *string    `json:"baseline_start"`
	BaselineFinish *string    `json:"baseline_finish"`
	Weight         float64    `json:"weight"`
	Progress       float64    `json:"progress"`
}

// buildBaselineSnapshot — saf (DB'siz) fonksiyon: ItemDTO listesinden
// bağımsız bir JSON anlık görüntüsü üretir. json.Marshal değer bazlı bir
// kopya çıkardığından, döndürülen []byte ÜZERİNDE items'a yapılacak sonraki
// hiçbir mutasyon (aynı slice/struct'lar başka yerde değiştirilse bile) etki
// etmez — dondurma sonrası revizyonun değişmezliğinin temeli budur.
func buildBaselineSnapshot(items []ItemDTO) ([]byte, error) {
	snap := make([]baselineSnapshotItem, 0, len(items))
	for _, it := range items {
		snap = append(snap, baselineSnapshotItem{
			ID: it.ID, ParentID: it.ParentID, WbsCode: it.WbsCode, Name: it.Name,
			IsMilestone: it.IsMilestone, BaselineStart: it.BaselineStart, BaselineFinish: it.BaselineFinish,
			Weight: it.Weight, Progress: it.Progress,
		})
	}
	return json.Marshal(snap)
}

// FreezeBaseline — projenin o anki WBS ağacını (ilerlemesiyle birlikte)
// değişmez bir JSON anlık görüntüsü olarak schedule_baselines'a yazar.
// Sonraki schedule_items değişiklikleri bu satırı ETKİLEMEZ.
func (h *Handler) FreezeBaseline(ctx context.Context, projectID, actorID uuid.UUID, note *string) (*BaselineDTO, error) {
	items, err := h.ListItems(ctx, projectID) // istek anında hesaplanmış Progress dahil
	if err != nil {
		return nil, err
	}
	snapJSON, err := buildBaselineSnapshot(items)
	if err != nil {
		return nil, err
	}

	var b BaselineDTO
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO schedule_baselines (project_id, revision_no, frozen_by, note, snapshot)
		VALUES ($1, COALESCE((SELECT max(revision_no)+1 FROM schedule_baselines WHERE project_id=$1), 1), $2, $3, $4)
		RETURNING id, project_id, revision_no, to_char(frozen_at,'YYYY-MM-DD"T"HH24:MI:SSOF'), frozen_by, note`,
		projectID, actorID, note, snapJSON,
	).Scan(&b.ID, &b.ProjectID, &b.RevisionNo, &b.FrozenAt, &b.FrozenBy, &b.Note); err != nil {
		return nil, err
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_baselines", EntityID: b.ID.String(), Action: audit.ActionInsert,
		After: map[string]any{"revision_no": b.RevisionNo, "item_count": len(items)}, IP: meta.IP, ReqID: meta.ReqID,
	})
	return &b, nil
}

func (h *Handler) ListBaselines(ctx context.Context, projectID uuid.UUID) ([]BaselineDTO, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id, project_id, revision_no, to_char(frozen_at,'YYYY-MM-DD"T"HH24:MI:SSOF'), frozen_by, note
		FROM schedule_baselines WHERE project_id=$1 ORDER BY revision_no DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BaselineDTO{}
	for rows.Next() {
		var b BaselineDTO
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.RevisionNo, &b.FrozenAt, &b.FrozenBy, &b.Note); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (h *Handler) GetBaseline(ctx context.Context, baselineID uuid.UUID) (*BaselineDetailDTO, error) {
	var d BaselineDetailDTO
	if err := h.pool.QueryRow(ctx, `
		SELECT id, project_id, revision_no, to_char(frozen_at,'YYYY-MM-DD"T"HH24:MI:SSOF'), frozen_by, note, snapshot
		FROM schedule_baselines WHERE id=$1`, baselineID,
	).Scan(&d.ID, &d.ProjectID, &d.RevisionNo, &d.FrozenAt, &d.FrozenBy, &d.Note, &d.Snapshot); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}
