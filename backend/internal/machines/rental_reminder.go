package machines

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/equipmenttransfers"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
)

// CheckRentalContracts — Faz E: dışarıdan (Render Cron Job) günlük
// tetiklenen bakım işi. Kiralık olup "Kiralama Sözleşmesi" belgesi hiç
// yüklenmemiş company_equipment kayıtlarını tarar; son hatırlatmanın
// üzerinden 7+ gün geçmişse (ya da hiç hatırlatma gitmemişse) atandığı
// projenin equipment.approve_transfer yetkilileri bildirilir.
//
// company_equipment'te kaydı kimin açtığı tutulmaz (şema bilinçli
// olarak sade tutuldu — bkz. migration 000047); bu yüzden yalnızca
// onaycılara gider, "kaydı açan kullanıcı" ayrıca bildirilmez. Merkez
// envanterde (current_project_id NULL — hiçbir projeye atanmamış)
// duran kiralık kayıtlar için bildirilecek bir proje bağlamı yoktur;
// bu kayıtlar taranır ama atlanır (last_rental_reminder_at güncellenmez
// — projeye atandıklarında tekrar değerlendirilirler).
func (h *Handler) CheckRentalContracts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.pool.Query(ctx, `
		SELECT ce.id, ce.ad, ce.current_project_id
		FROM company_equipment ce
		WHERE ce.sahiplik = 'kiralik'
		  AND NOT EXISTS (
		    SELECT 1 FROM documents d
		    WHERE d.entity_type = 'company_equipment' AND d.entity_id = ce.id
		      AND d.doc_category = 'KiralamaSozlesmesi' AND d.deleted_at IS NULL
		  )
		  AND (ce.last_rental_reminder_at IS NULL OR ce.last_rental_reminder_at < now() - interval '7 days')`)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	type candidate struct {
		id        uuid.UUID
		ad        string
		projectID *uuid.UUID
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.ad, &c.projectID); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		candidates = append(candidates, c)
	}
	rows.Close()

	notified := 0
	for _, c := range candidates {
		if c.projectID == nil {
			continue // proje bağlamı yok — bildirim atlanır, hatırlatma zamanı güncellenmez
		}
		id := c.id
		equipmenttransfers.NotifyApprovers(ctx, h.pool, h.nt, *c.projectID, uuid.Nil, notify.Input{
			Type:       notify.TypeRentalContractMissing,
			Title:      c.ad + " için kiralama sözleşmesi hâlâ yüklenmedi",
			EntityType: "company_equipment", EntityID: &id, ProjectID: c.projectID,
		})
		if _, err := h.pool.Exec(ctx,
			`UPDATE company_equipment SET last_rental_reminder_at = now() WHERE id = $1`, c.id); err != nil {
			httpx.Internal(w, r)
			return
		}
		notified++
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"checked": len(candidates), "notified": notified})
}
