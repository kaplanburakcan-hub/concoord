package payments

// Faz 11 — Hakediş dönem kontrolleri.
//
// Şimdiye dek yalnızca dönem NUMARASI tekildi (uq_pp_period). Tarih aralığı
// serbest olduğundan aynı aya iki hakediş girilebiliyor, sözleşme süresini aşan
// dönemler engellenmiyordu. Üç kontrol eklenir:
//
//   1. period_start ≤ period_end
//   2. Aynı taşeron için tarih aralıkları çakışamaz (revizyonlar hariç —
//      revizyon, düzelttiği dönemle bilinçli olarak aynı aralığı taşır)
//   3. Dönem, sözleşme süresi içinde olmalı (süre uzatımı varsa uzatılmış tarih)
//
// Son söz veritabanınındır (chk_pp_period_order + excl_pp_period_overlap);
// buradaki kontroller kullanıcıya anlaşılır hata mesajı vermek içindir.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// PeriodCheckInput — dönem doğrulaması için gerekli bağlam.
type PeriodCheckInput struct {
	SubcontractorID string
	PaymentID       string // güncellemede kendi kaydını dışla; oluşturmada boş
	Start           *time.Time
	End             *time.Time
	IsRevision      bool
}

// ValidatePeriod — dönem tarihlerini doğrular. Dönen map boşsa sorun yoktur;
// doluysa doğrudan httpx.ValidationFailed'e verilebilir.
func ValidatePeriod(ctx context.Context, q pgxQuerier, in PeriodCheckInput) (map[string]string, error) {
	f := map[string]string{}

	// Tarih girilmemişse bu kontroller uygulanmaz (taslak aşamasında serbest).
	if in.Start == nil || in.End == nil {
		return f, nil
	}
	if in.End.Before(*in.Start) {
		f["period_end"] = "Dönem bitişi başlangıçtan önce olamaz."
		return f, nil
	}

	// --- 2) Aynı taşeronda tarih aralığı çakışması ---
	if !in.IsRevision {
		var conflictNo *int
		var cs, ce *time.Time
		err := q.QueryRow(ctx, `
			SELECT period_no, period_start, period_end
			FROM progress_payments
			WHERE subcontractor_id=$1
			  AND deleted_at IS NULL
			  AND revision_of IS NULL
			  AND period_start IS NOT NULL AND period_end IS NOT NULL
			  AND ($4 = '' OR id <> $4::uuid)
			  AND daterange(period_start, period_end, '[]') && daterange($2::date, $3::date, '[]')
			LIMIT 1`,
			in.SubcontractorID, in.Start.Format("2006-01-02"), in.End.Format("2006-01-02"), in.PaymentID).
			Scan(&conflictNo, &cs, &ce)
		if err == nil && conflictNo != nil {
			f["period_start"] = fmt.Sprintf(
				"Bu tarih aralığı %d. hakediş ile çakışıyor (%s – %s). Aynı döneme ikinci hakediş düzenlenemez.",
				*conflictNo, fmtDate(cs), fmtDate(ce))
			return f, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	// --- 3) Sözleşme süresi aşımı ---
	var cStart, cEnd *time.Time
	var contractNo string
	err := q.QueryRow(ctx, `
		SELECT contract_no, start_date, COALESCE(revised_end_date, end_date)
		FROM contracts
		WHERE subcontractor_id=$1 AND deleted_at IS NULL AND type IN ('Main','Sub')
		ORDER BY sign_date DESC NULLS LAST, created_at DESC
		LIMIT 1`, in.SubcontractorID).Scan(&contractNo, &cStart, &cEnd)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return f, nil // sözleşme yoksa süre kontrolü yapılamaz
		}
		return nil, err
	}
	if cStart != nil && in.Start.Before(*cStart) {
		f["period_start"] = fmt.Sprintf(
			"Dönem başlangıcı sözleşme başlangıcından (%s) önce olamaz. Sözleşme: %s",
			fmtDate(cStart), contractNo)
	}
	if cEnd != nil && in.End.After(*cEnd) {
		f["period_end"] = fmt.Sprintf(
			"Dönem bitişi sözleşme süresini (%s) aşıyor. Süre uzatımı verildiyse sözleşmeye işleyin. Sözleşme: %s",
			fmtDate(cEnd), contractNo)
	}
	return f, nil
}

func fmtDate(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("02.01.2006")
}
