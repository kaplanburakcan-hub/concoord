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

// KesinKabulTarihi — projenin Kesin Kabul tarihini hesaplar: Yer Teslim
// Tarihi + İşin Süresi (gün, = Geçici Kabul) + Geçici Kabul Sonrası (gün).
// Ana sözleşme kaydı yoksa ya da bu üç alandan biri eksikse nil döner (henüz
// hesaplanamaz — çağıran taraf bu durumda sınır kontrolünü ATLAMALI, eksik
// sözleşme bilgisiyle kullanıcıyı haksız yere engellememek için).
func KesinKabulTarihi(ctx context.Context, q Querier, projectID uuid.UUID) (*time.Time, error) {
	var yerTeslim *time.Time
	var isSuresiGun, geciciKabulSonrasiGun *int
	err := q.QueryRow(ctx,
		`SELECT yer_teslim_tarihi, is_suresi_gun, gecici_kabul_sonrasi_gun
		 FROM project_main_contracts WHERE project_id=$1`, projectID).
		Scan(&yerTeslim, &isSuresiGun, &geciciKabulSonrasiGun)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if yerTeslim == nil || isSuresiGun == nil || geciciKabulSonrasiGun == nil {
		return nil, nil
	}
	kesinKabul := truncateToDay(*yerTeslim).
		AddDate(0, 0, *isSuresiGun).
		AddDate(0, 0, *geciciKabulSonrasiGun)
	return &kesinKabul, nil
}

// NotAfterKesinKabul — date, projenin Kesin Kabul tarihini AŞAMAZ (iş/teslimat
// niteliğindeki kayıtlar için — hakediş, tutanak, toplantı, milestone, görev,
// puantaj, rapor, depo hareketi, İSG bulgusu, yazışma, satınalma tarihleri).
// Finansal/kapsam tarihleri (vade, çek keşide, sigorta bitişi, taşeron
// sözleşme bitişi, sabit gider) BİLİNÇLİ OLARAK bu kontrolün dışındadır —
// bunlar doğal olarak proje sonrasına uzanabilir. Kesin Kabul henüz
// hesaplanamıyorsa (ana sözleşme eksik) kontrol sessizce atlanır.
func NotAfterKesinKabul(ctx context.Context, q Querier, projectID uuid.UUID, date time.Time, fieldName string) (map[string]string, error) {
	kesinKabul, err := KesinKabulTarihi(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	if kesinKabul == nil {
		return nil, nil
	}
	if truncateToDay(date).After(*kesinKabul) {
		return map[string]string{
			fieldName: fmt.Sprintf("Proje Kesin Kabul tarihini (%s) geçen bir tarih girilemez.", kesinKabul.Format("02.01.2006")),
		}, nil
	}
	return nil, nil
}
