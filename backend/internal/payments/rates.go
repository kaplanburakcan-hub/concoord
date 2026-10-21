package payments

// Hakedişe esas oranlar (KDV, KDV tevkifatı, teminat, stopaj): sözleşme
// girişinde belirlenir; hakedişler varsayılan olarak bunları kullanır.
// Hakedişte değiştirmek progress_payments.edit_rates izni ister.

import (
	"context"
	"math"
	"net/http"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/auth"
)

type contractTax struct {
	Found          bool
	IsTuru         string
	VatPct         float64
	VatRatio       float64 // 0 = tevkifat yok, 0.4 = 4/10
	RetentionPct   float64
	WithholdingPct float64
}

// contractTax — taşeronun yürürlükteki (Main/Sub, en yeni) sözleşmesindeki oranlar.
// Sözleşme yoksa eski varsayılanlar (KDV %20, teminat %3, stopaj %5) döner.
func (h *Handler) contractTax(ctx context.Context, q pgxQuerier, subID uuid.UUID) contractTax {
	t := contractTax{IsTuru: "yapim", VatPct: 20, RetentionPct: 3, WithholdingPct: 5}
	var c contractTax
	err := q.QueryRow(ctx, `
		SELECT is_turu, vat_pct::float8, vat_withholding_ratio::float8,
		       retention_pct::float8, COALESCE(withholding_pct,5)::float8
		FROM contracts
		WHERE subcontractor_id=$1 AND deleted_at IS NULL AND type IN ('Main','Sub')
		ORDER BY sign_date DESC NULLS LAST, created_at DESC LIMIT 1`, subID).
		Scan(&c.IsTuru, &c.VatPct, &c.VatRatio, &c.RetentionPct, &c.WithholdingPct)
	if err != nil {
		return t
	}
	c.Found = true
	return c
}

// canEditRates — kullanıcı bu projede hakediş oranlarını değiştirebilir mi?
func (h *Handler) canEditRates(r *http.Request, pid uuid.UUID) bool {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		return false
	}
	allowed, err := h.eval.Can(r.Context(), uid, &pid, "progress_payments.edit_rates")
	return err == nil && allowed
}

func sameRate(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
