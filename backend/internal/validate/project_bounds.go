// Package validate — projedeki tarih alanları için tekrar kullanılabilir
// sınır kontrolleri. İlk kullanım: Makine/Ekipman/Araç Envanteri Faz D
// (iş başı tarihi). payments/period_check.go'daki desenle aynı: sınırları
// çek, karşılaştır, alan-anahtarlı hata map'i döndür.
package validate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// WithinProjectBounds — date, projenin ana sözleşme tarihinden ÖNCE
// olamaz ve bugünden SONRA olamaz (yalnızca bugün ve öncesi kabul edilir).
// Ana sözleşme kaydı yoksa (sozlesme_tarihi NULL) alt sınır kontrolü
// atlanır. Dönen map boşsa sorun yoktur.
func WithinProjectBounds(ctx context.Context, q Querier, projectID uuid.UUID, date time.Time, fieldName string) (map[string]string, error) {
	f := map[string]string{}
	d := truncateToDay(date)
	today := truncateToDay(time.Now())

	if d.After(today) {
		f[fieldName] = "Bugünden sonraki bir tarih girilemez."
		return f, nil
	}

	var sozlesmeTarihi *time.Time
	err := q.QueryRow(ctx,
		`SELECT sozlesme_tarihi FROM project_main_contracts WHERE project_id=$1`, projectID).
		Scan(&sozlesmeTarihi)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if sozlesmeTarihi != nil {
		st := truncateToDay(*sozlesmeTarihi)
		if d.Before(st) {
			f[fieldName] = fmt.Sprintf(
				"Proje ana sözleşme tarihinden (%s) önce bir tarih girilemez.", st.Format("02.01.2006"))
		}
	}
	return f, nil
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
