package payments

// Faz 11 — Çok adımlı hakediş onay zinciri.
//
// Saha uygulamasında hakediş tek bir "saha onayı" ile kesinleşmez; teknik ofis,
// kalite, İSG, şantiye müdürlüğü, merkez ofis, genel müdürlük, muhasebe ve
// finansal kontrol kademelerinden sırayla geçer.
//
// Zincir SABİT KODLANMAZ, veritabanında tutulur (payment_approval_steps):
// proje bazında tanımlanabilir, yoksa global varsayılan (project_id IS NULL)
// zincir uygulanır. Böylece her kurum kendi akışını sürüm çıkmadan kurar
// (Plan §3: "yetki koda gömülmez, veriye yazılır").
//
// Akış:
//   Draft → Submitted → [adım 1..N onayları] → Finalized
//   Herhangi bir adımda ret → Rejected (taslağa geri çekilebilir)
//
// Her onay payment_approvals'a yazılır ve SİLİNMEZ — kim neyi ne zaman onayladı
// sorusunun cevabı kalıcıdır (Plan §3: "hiçbir değişiklik izsiz kalmaz").

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type approvalStep struct {
	StepNo     int    `json:"step_no"`
	Code       string `json:"code"`
	Label      string `json:"label"`
	Permission string `json:"permission"`
}

type approvalRecord struct {
	StepNo    int       `json:"step_no"`
	StepCode  string    `json:"step_code"`
	Label     string    `json:"label,omitempty"`
	Decision  string    `json:"decision"`
	Note      *string   `json:"note,omitempty"`
	ActorName string    `json:"actor_name"`
	CreatedAt time.Time `json:"created_at"`
}

// loadChain — projeye özel zincir varsa onu, yoksa global varsayılanı döner.
func loadChain(ctx context.Context, q pgxQuerier, projectID uuid.UUID) ([]approvalStep, error) {
	rows, err := q.Query(ctx, `
		SELECT step_no, code, label, permission
		FROM payment_approval_steps
		WHERE is_active AND project_id = $1
		ORDER BY step_no`, projectID)
	if err != nil {
		return nil, err
	}
	steps := scanSteps(rows)
	if len(steps) > 0 {
		return steps, nil
	}
	rows, err = q.Query(ctx, `
		SELECT step_no, code, label, permission
		FROM payment_approval_steps
		WHERE is_active AND project_id IS NULL
		ORDER BY step_no`)
	if err != nil {
		return nil, err
	}
	return scanSteps(rows), nil
}

func scanSteps(rows pgx.Rows) []approvalStep {
	defer rows.Close()
	out := []approvalStep{}
	for rows.Next() {
		var s approvalStep
		if err := rows.Scan(&s.StepNo, &s.Code, &s.Label, &s.Permission); err != nil {
			return out
		}
		out = append(out, s)
	}
	return out
}

// ApprovalChain — GET /api/v1/payments/{id}/approvals
// Zinciri, tamamlanmış onayları ve sıradaki adımı döner.
func (h *Handler) ApprovalChain(w http.ResponseWriter, r *http.Request) {
	ppID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz hakediş kimliği.", nil)
		return
	}

	var projectID uuid.UUID
	var status string
	var currentStep int
	if err := h.pool.QueryRow(r.Context(),
		`SELECT project_id, status, current_step_no FROM progress_payments
		 WHERE id=$1 AND deleted_at IS NULL`, ppID).Scan(&projectID, &status, &currentStep); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}

	chain, err := loadChain(r.Context(), h.pool, projectID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT a.step_no, a.step_code, a.decision, a.note, u.full_name, a.created_at
		FROM payment_approvals a
		JOIN users u ON u.id = a.actor_id
		WHERE a.progress_payment_id=$1
		ORDER BY a.created_at`, ppID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	history := []approvalRecord{}
	for rows.Next() {
		var a approvalRecord
		if err := rows.Scan(&a.StepNo, &a.StepCode, &a.Decision, &a.Note, &a.ActorName, &a.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		for _, st := range chain {
			if st.StepNo == a.StepNo {
				a.Label = st.Label
			}
		}
		history = append(history, a)
	}

	var next *approvalStep
	if status == "Submitted" || status == "InApproval" {
		for i := range chain {
			if chain[i].StepNo == currentStep+1 {
				next = &chain[i]
				break
			}
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"chain":           chain,
		"history":         history,
		"current_step_no": currentStep,
		"next_step":       next,
		"status":          status,
	})
}

type approveReq struct {
	Decision   string `json:"decision"` // Approved | Rejected
	Note       string `json:"note"`
	RowVersion int    `json:"row_version"`
}

// ApproveStep — POST /api/v1/payments/{id}/approvals
// Sıradaki adımı onaylar veya reddeder. Son adım onaylandığında hakediş
// kesinleşir (Finalized) ve değiştirilemez hale gelir.
func (h *Handler) ApproveStep(w http.ResponseWriter, r *http.Request) {
	ppID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz hakediş kimliği.", nil)
		return
	}
	var req approveReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Decision != "Approved" && req.Decision != "Rejected" {
		httpx.ValidationFailed(w, r, map[string]string{"decision": "Approved veya Rejected olmalı."})
		return
	}
	if req.Decision == "Rejected" && len(req.Note) == 0 {
		httpx.ValidationFailed(w, r, map[string]string{"note": "Ret gerekçesi zorunludur."})
		return
	}

	m := audit.MetaFrom(r.Context())
	actorID, err := uuid.Parse(m.ActorID)
	if err != nil {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum bulunamadı.", nil)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var projectID uuid.UUID
	var status string
	var currentStep, rowVersion int
	if err := tx.QueryRow(r.Context(), `
		SELECT project_id, status, current_step_no, row_version
		FROM progress_payments WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, ppID).
		Scan(&projectID, &status, &currentStep, &rowVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Hakediş bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != rowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi, sayfayı yenileyin.", nil)
		return
	}
	if status != "Submitted" && status != "InApproval" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Bu hakediş onay sürecinde değil (durum: "+status+").", nil)
		return
	}

	chain, err := loadChain(r.Context(), tx, projectID)
	if err != nil || len(chain) == 0 {
		httpx.Internal(w, r)
		return
	}
	var step *approvalStep
	for i := range chain {
		if chain[i].StepNo == currentStep+1 {
			step = &chain[i]
			break
		}
	}
	if step == nil {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Onaylanacak adım kalmadı.", nil)
		return
	}

	// Adımın gerektirdiği izin, zincir verisinden gelir ve RBAC motoruyla
	// proje kapsamında değerlendirilir (kullanıcı override'ları dahil).
	allowed, err := h.eval.Can(r.Context(), actorID, &projectID, step.Permission)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if !allowed {
		httpx.Error(w, r, http.StatusForbidden, httpx.CodeForbidden,
			"Bu adımı onaylama yetkiniz yok: "+step.Label,
			map[string]string{"required": step.Permission})
		return
	}

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO payment_approvals (progress_payment_id, step_no, step_code, decision, note, actor_id)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6)`,
		ppID, step.StepNo, step.Code, req.Decision, req.Note, actorID); err != nil {
		httpx.Internal(w, r)
		return
	}

	newStatus := "InApproval"
	newStep := step.StepNo
	if req.Decision == "Rejected" {
		newStatus = "Rejected"
		newStep = 0
	} else if step.StepNo == chain[len(chain)-1].StepNo {
		// Son adım: hakediş kesinleşir ve kilitlenir.
		newStatus = "Finalized"
	}

	if newStatus == "Finalized" {
		_, err = tx.Exec(r.Context(), `
			UPDATE progress_payments
			SET status='Finalized', current_step_no=$2, finalized_at=now(), finalized_by=$3,
			    row_version=row_version+1
			WHERE id=$1`, ppID, newStep, actorID)
	} else {
		_, err = tx.Exec(r.Context(), `
			UPDATE progress_payments
			SET status=$2, current_step_no=$3, row_version=row_version+1
			WHERE id=$1`, ppID, newStatus, newStep)
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "progress_payments", EntityID: ppID.String(),
		Action: audit.ActionUpdate,
		After: map[string]interface{}{
			"step": step.Code, "decision": req.Decision, "status": newStatus,
		},
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"status":          newStatus,
		"current_step_no": newStep,
		"step":            step,
		"decision":        req.Decision,
	})
}
