// Package approvals — genel/tekrar kullanılabilir çok kademeli onay
// hiyerarşisi motoru. Herhangi bir varlık (entity_type/entity_id ile)
// approval_chain_steps'te tanımlı sıralı kademelerden geçer; her kademenin
// onaycısı o kademenin role_code'una sahip proje üyeleridir. Herhangi bir
// kademe reddederse zincir orada durur (escalate yok). İlk kullanım:
// equipment_transfer_requests (bkz. internal/equipmenttransfers). Yeni bir
// modül eklemek için sadece migration'a approval_chain_steps satırı eklemek
// yeterli — bu paket değişmez.
package approvals

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Request struct {
	ID          uuid.UUID
	EntityType  string
	EntityID    uuid.UUID
	ProjectID   uuid.UUID
	CurrentStep int
	TotalSteps  int
	Status      string
}

// StepRole — entityType için step_order kademesinin role_code'u.
func StepRole(ctx context.Context, q Querier, entityType string, step int) (string, error) {
	var role string
	err := q.QueryRow(ctx,
		`SELECT role_code FROM approval_chain_steps WHERE entity_type=$1 AND step_order=$2`,
		entityType, step).Scan(&role)
	return role, err
}

// Start — entityType/entityID için yeni bir onay süreci açar (kademe 1'den
// başlar). Aynı transaction içinde çağrılmalı (varlığın kendi INSERT'iyle
// birlikte). İlk kademenin role_code'unu döner ki çağıran bildirim atabilsin.
func Start(ctx context.Context, tx pgx.Tx, entityType string, entityID, projectID, createdBy uuid.UUID) (*Request, string, error) {
	var totalSteps int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM approval_chain_steps WHERE entity_type=$1`, entityType).Scan(&totalSteps); err != nil {
		return nil, "", err
	}
	if totalSteps == 0 {
		return nil, "", fmt.Errorf("approvals: %q için tanımlı onay zinciri yok", entityType)
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO approval_requests (entity_type, entity_id, project_id, total_steps, created_by)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		entityType, entityID, projectID, totalSteps, createdBy).Scan(&id); err != nil {
		return nil, "", err
	}
	role, err := StepRole(ctx, tx, entityType, 1)
	if err != nil {
		return nil, "", err
	}
	return &Request{
		ID: id, EntityType: entityType, EntityID: entityID, ProjectID: projectID,
		CurrentStep: 1, TotalSteps: totalSteps, Status: "pending",
	}, role, nil
}

// Get — varlığın onay sürecini kilitleyerek okur (FOR UPDATE — decide()'dan
// önce aynı transaction'da çağrılmalı, yarış durumunu önler).
func Get(ctx context.Context, tx pgx.Tx, entityType string, entityID uuid.UUID) (*Request, error) {
	var req Request
	req.EntityType, req.EntityID = entityType, entityID
	err := tx.QueryRow(ctx,
		`SELECT id, project_id, current_step, total_steps, status
		 FROM approval_requests WHERE entity_type=$1 AND entity_id=$2 FOR UPDATE`,
		entityType, entityID).Scan(&req.ID, &req.ProjectID, &req.CurrentStep, &req.TotalSteps, &req.Status)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// ActorRoleAllowed — userID, projectID'de requiredRole rolüne sahip mi?
func ActorRoleAllowed(ctx context.Context, q Querier, projectID, userID uuid.UUID, requiredRole string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM project_members pm JOIN roles r ON r.id = pm.role_id
			WHERE pm.project_id=$1 AND pm.user_id=$2 AND pm.deleted_at IS NULL AND r.code=$3
		)`, projectID, userID, requiredRole).Scan(&exists)
	return exists, err
}

// Decide — req'in GEÇERLİ kademesi için karar kaydeder ve zinciri ilerletir.
// status: "pending" (sıradaki kademeye geçti) | "approved" (tüm kademeler
// tamamlandı) | "rejected" (bu kademe reddetti, zincir durdu). isFinal true
// ise çağıran onay/red sonucu iş mantığını (ör. ekipmanı taşı) uygulamalı.
// nextRole yalnızca status="pending" iken doludur (bir sonraki kademeye
// bildirim atmak için).
func Decide(ctx context.Context, tx pgx.Tx, req *Request, actorID uuid.UUID, actorRoleCode string, approve bool, note string) (status string, isFinal bool, nextRole string, err error) {
	decision := "rejected"
	if approve {
		decision = "approved"
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO approval_decisions (approval_request_id, step_order, role_code, decided_by, decision, note)
		 VALUES ($1,$2,$3,$4,$5,NULLIF(btrim($6),''))`,
		req.ID, req.CurrentStep, actorRoleCode, actorID, decision, note); err != nil {
		return "", false, "", err
	}

	if !approve {
		if _, err = tx.Exec(ctx,
			`UPDATE approval_requests SET status='rejected', decided_at=now() WHERE id=$1`, req.ID); err != nil {
			return "", false, "", err
		}
		return "rejected", true, "", nil
	}

	if req.CurrentStep >= req.TotalSteps {
		if _, err = tx.Exec(ctx,
			`UPDATE approval_requests SET status='approved', decided_at=now() WHERE id=$1`, req.ID); err != nil {
			return "", false, "", err
		}
		return "approved", true, "", nil
	}

	next := req.CurrentStep + 1
	if _, err = tx.Exec(ctx,
		`UPDATE approval_requests SET current_step=$2 WHERE id=$1`, req.ID, next); err != nil {
		return "", false, "", err
	}
	nextRole, err = StepRole(ctx, tx, req.EntityType, next)
	if err != nil {
		return "", false, "", err
	}
	return "pending", false, nextRole, nil
}
