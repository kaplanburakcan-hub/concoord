package schedule

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
)

// Edge — bir schedule_dependencies satırının döngü kontrolü için yeterli kısmı.
type Edge struct {
	Predecessor uuid.UUID
	Successor   uuid.UUID
}

var ErrCycle = errors.New("döngüsel bağımlılık")

// WouldCreateCycle — saf fonksiyon (DB'siz). Mevcut kenarlara
// (predecessor→successor) newPred→newSucc eklenirse döngü oluşur mu?
// newSucc'tan başlayıp mevcut kenarlar üzerinden newPred'e ulaşılabiliyorsa
// (yani newSucc zaten newPred'e giden bir zincirin başındaysa), yeni kenar
// bu zinciri kapatıp bir döngü oluşturur.
func WouldCreateCycle(existing []Edge, newPred, newSucc uuid.UUID) bool {
	if newPred == newSucc {
		return true
	}
	adj := make(map[uuid.UUID][]uuid.UUID, len(existing))
	for _, e := range existing {
		adj[e.Predecessor] = append(adj[e.Predecessor], e.Successor)
	}
	visited := make(map[uuid.UUID]bool)
	stack := []uuid.UUID{newSucc}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == newPred {
			return true
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, next := range adj[cur] {
			if !visited[next] {
				stack = append(stack, next)
			}
		}
	}
	return false
}

// loadEdges — bir projenin tüm schedule_dependencies kenarlarını yükler.
func (h *Handler) loadEdges(ctx context.Context, projectID uuid.UUID) ([]Edge, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT sd.predecessor_id, sd.successor_id
		FROM schedule_dependencies sd
		JOIN schedule_items si ON si.id = sd.predecessor_id
		WHERE si.project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Edge{}
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.Predecessor, &e.Successor); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListDependencies — proje çapındaki tüm bağımlılıkları döner (DÜZELTME:
// spec'in 1.4 API listesinde bir GET yoktu, ama Gantt'ta ok çizmek ve
// bağımlılık yönetim arayüzü için gerekli).
func (h *Handler) ListDependencies(ctx context.Context, projectID uuid.UUID) ([]DependencyDTO, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT sd.predecessor_id, sd.successor_id, sd.dep_type, sd.lag_days
		FROM schedule_dependencies sd
		JOIN schedule_items si ON si.id = sd.predecessor_id
		WHERE si.project_id = $1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DependencyDTO{}
	for rows.Next() {
		var d DependencyDTO
		if err := rows.Scan(&d.PredecessorID, &d.SuccessorID, &d.DepType, &d.LagDays); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type DependencyDTO struct {
	PredecessorID uuid.UUID `json:"predecessor_id"`
	SuccessorID   uuid.UUID `json:"successor_id"`
	DepType       string    `json:"dep_type"`
	LagDays       int       `json:"lag_days"`
}

// CreateDependency — yeni bir bağımlılık kenarı ekler. Eklemeden önce
// projenin TÜM mevcut kenarlarını yükleyip WouldCreateCycle ile kontrol
// eder; döngü oluşacaksa ErrCycle döner ve HİÇBİR ŞEY yazılmaz.
func (h *Handler) CreateDependency(ctx context.Context, projectID, actorID uuid.UUID, predID, succID uuid.UUID, depType string, lagDays int) (*DependencyDTO, error) {
	if depType == "" {
		depType = "FS"
	}
	switch depType {
	case "FS", "SS", "FF", "SF":
	default:
		return nil, errors.New("dep_type 'FS','SS','FF','SF' olmalı")
	}

	var predProject, succProject uuid.UUID
	if err := h.pool.QueryRow(ctx,
		`SELECT project_id FROM schedule_items WHERE id=$1 AND deleted_at IS NULL`, predID,
	).Scan(&predProject); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := h.pool.QueryRow(ctx,
		`SELECT project_id FROM schedule_items WHERE id=$1 AND deleted_at IS NULL`, succID,
	).Scan(&succProject); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if predProject != projectID || succProject != projectID {
		return nil, ErrCrossProject
	}

	edges, err := h.loadEdges(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if WouldCreateCycle(edges, predID, succID) {
		return nil, ErrCycle
	}

	var d DependencyDTO
	if err := h.pool.QueryRow(ctx, `
		INSERT INTO schedule_dependencies (predecessor_id, successor_id, dep_type, lag_days)
		VALUES ($1,$2,$3,$4)
		RETURNING predecessor_id, successor_id, dep_type, lag_days`,
		predID, succID, depType, lagDays,
	).Scan(&d.PredecessorID, &d.SuccessorID, &d.DepType, &d.LagDays); err != nil {
		return nil, err
	}

	meta := audit.MetaFrom(ctx)
	// entity_id sütunu uuid tipinde — kenar tek kolona sığmadığı için
	// predecessor'ı EntityID yapıp her iki ucu da After'a yazıyoruz.
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_dependencies",
		EntityID: predID.String(), Action: audit.ActionInsert,
		After: map[string]any{"predecessor_id": predID, "successor_id": succID, "dep_type": depType, "lag_days": lagDays},
		IP:    meta.IP, ReqID: meta.ReqID,
	})
	return &d, nil
}

func (h *Handler) DeleteDependency(ctx context.Context, actorID, predID, succID uuid.UUID) error {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM schedule_dependencies WHERE predecessor_id=$1 AND successor_id=$2`, predID, succID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_dependencies",
		EntityID: predID.String(), Action: audit.ActionDelete,
		After: map[string]any{"successor_id": succID}, IP: meta.IP, ReqID: meta.ReqID,
	})
	return nil
}
