// Package pdfjobs — API sürecinde çalışan PDF iş döngüsü.
//
// Render'da ayrı bir worker servisi yok (ücretsiz plan); haftalık ve aylık/dönem
// rapor PDF'lerini üreten işler bu döngüyle API içinde işlenir. Yalnızca PDF
// türleri çekilir: e-posta/SMS işleri ve periyodik taramalar cmd/worker'ın
// işi olarak kalır. FOR UPDATE SKIP LOCKED sayesinde ayrı bir worker da
// çalışıyorsa aynı iş iki kez işlenmez.
package pdfjobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/dashboard"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/reports"
	"github.com/ipks/ipks/backend/internal/storage"
)

var kinds = []string{reports.JobKindWeeklyPDF, dashboard.JobKindMonthlyPDF}

// Run — ctx iptal edilene dek kuyruğu sorgular.
func Run(ctx context.Context, pool *pgxpool.Pool, store *storage.Client, log *slog.Logger) {
	nt := notify.NewService(pool, log)
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			drain(ctx, pool, store, nt, log)
		}
	}
}

func drain(ctx context.Context, pool *pgxpool.Pool, store *storage.Client, nt *notify.Service, log *slog.Logger) {
	for i := 0; i < 10; i++ {
		job, err := notify.DequeueKinds(ctx, pool, kinds)
		if err != nil {
			log.Error("PDF kuyruğundan iş alınamadı", "err", err)
			return
		}
		if job == nil {
			return
		}
		switch job.Kind {
		case reports.JobKindWeeklyPDF:
			err = reports.ProcessWeeklyPDF(ctx, pool, store, nt, log, job.Payload)
		case dashboard.JobKindMonthlyPDF:
			err = dashboard.ProcessMonthlyPDF(ctx, pool, store, nt, log, job.Payload)
		}
		if err != nil {
			log.Warn("PDF işi başarısız", "id", job.ID, "kind", job.Kind, "attempt", job.Attempts, "err", err)
			if ferr := notify.Fail(ctx, pool, job, err); ferr != nil {
				log.Error("iş durumu güncellenemedi", "id", job.ID, "err", ferr)
			}
			continue
		}
		if cerr := notify.Complete(ctx, pool, job.ID); cerr != nil {
			log.Error("iş tamamlandı işaretlenemedi", "id", job.ID, "err", cerr)
		}
	}
}
