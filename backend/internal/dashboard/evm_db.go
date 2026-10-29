package dashboard

// evm_db.go — EVM girdilerini DB'den derler ve saf çekirdeğe (evm.go) besler.
// Dashboard ucu, aylık rapor snapshot'ı ve eşik uyarı taraması AYNI fonksiyonu
// kullanır — üç yerde üç farklı hesap olmaz (tek doğruluk kaynağı).

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/schedule"
)

// EVMResult — bir projenin güncel EVM durumu (kümülatif) + tam S-eğrisi.
type EVMResult struct {
	BAC            float64       `json:"bac"`
	ContractsTotal float64       `json:"contracts_total"`
	Currency       string        `json:"currency"`
	PV             float64       `json:"pv"`
	EV             float64       `json:"ev"`
	AC             float64       `json:"ac"`
	SPI            float64       `json:"spi"` // 0 = tanımsız
	CPI            float64       `json:"cpi"` // 0 = tanımsız
	EAC            float64       `json:"eac"`
	ETC            float64       `json:"etc"`
	ProgressPct    float64       `json:"progress_pct"`
	PlanSource     string        `json:"plan_source"` // manual | milestones | linear
	SCurve         []SCurvePoint `json:"s_curve"`
	AsOfMonth      string        `json:"as_of_month"` // kümülatiflerin kesildiği ay

	// Parasal ilerleme: yapılan İDARİ HAKEDİŞ toplamı (idareye/işverene
	// kesilen hakediş — cari ödeme durumundan bağımsız) / ANA SÖZLEŞME
	// bedeli (project_main_contracts.sozlesme_bedeli — Ana Sözleşme
	// modülü). AC/CPI ile KARIŞTIRILMAMALI: CPI zaten harcanan maliyeti
	// gösteriyor; parasal ilerleme burada "idareye ne kadarı hakedişe
	// bağlandı" sorusuna cevap verir. Ana sözleşme bedeli girilmemişse
	// nil kalır (frontend "izin yok" değil "sözleşme bedeli girilmemiş"
	// ayrımını yapar).
	MainContractAmount   *float64 `json:"contract_amount,omitempty"`
	FinancialProgressPct float64  `json:"financial_progress_pct"`

	// Zamansal ilerleme (Portföy'de "Zamansal İlerleme"): proje künyesi
	// başlangıç/bitiş tarihleri. Herkese açık (finansal izin gerekmez —
	// ProgressPct/fiziki ile aynı görünürlük ilkesi).
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
}

// LoadEVM — proje için EVM verisini derler. Kümülatif PV/EV/AC ve endeksler
// "bugünün ayı"na (proje bitmişse son plan ayına) kadar hesaplanır; S-eğrisi
// tüm plan ufkunu içerir.
func LoadEVM(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID, now time.Time) (*EVMResult, error) {
	res := &EVMResult{}

	// Proje künyesi: bütçe (BAC) + plan ufku.
	var budget *float64
	var start, end *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT budget_total, currency, start_date, end_date
		FROM projects WHERE id=$1 AND deleted_at IS NULL`, pid).
		Scan(&budget, &res.Currency, &start, &end); err != nil {
		return nil, err
	}
	if budget != nil {
		res.BAC = *budget
	}
	res.StartDate, res.EndDate = start, end

	// Parasal ilerleme girdileri: ana sözleşme bedeli + yapılan idari
	// hakediş toplamı (bkz. EVMResult.FinancialProgressPct doc).
	var sozlesmeBedeli *float64
	if err := pool.QueryRow(ctx,
		`SELECT sozlesme_bedeli FROM project_main_contracts WHERE project_id=$1`, pid).
		Scan(&sozlesmeBedeli); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var hakedisToplam float64
	if err := pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(tutar),0) FROM idari_hakedisler WHERE project_id=$1 AND deleted_at IS NULL`, pid).
		Scan(&hakedisToplam); err != nil {
		return nil, err
	}
	if sozlesmeBedeli != nil && *sozlesmeBedeli > 0 {
		res.MainContractAmount = sozlesmeBedeli
		res.FinancialProgressPct = round2(hakedisToplam / *sozlesmeBedeli * 100)
	}

	// Sözleşme toplamı (EV ölçeği için; ana sözleşme değil ALT sözleşmeler —
	// EV kesinleşmiş TAŞERON hakedişlerinden türediği için payda da odur).
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(sum(amount),0) FROM contracts
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND type IN ('Sub','Addendum') AND amount IS NOT NULL`, pid).
		Scan(&res.ContractsTotal); err != nil {
		return nil, err
	}

	// EV: kesinleşen hakediş brütleri (gross_this), kesinleşme ayına yazılır.
	scale := EVScale(res.BAC, res.ContractsTotal)
	evByMonth := map[string]float64{}
	rows, err := pool.Query(ctx, `
		SELECT to_char(finalized_at,'YYYY-MM'), sum(gross_this)
		FROM progress_payments
		WHERE project_id=$1 AND deleted_at IS NULL AND status='Finalized'
		  AND finalized_at IS NOT NULL
		GROUP BY 1`, pid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m string
		var v float64
		if err := rows.Scan(&m, &v); err != nil {
			rows.Close()
			return nil, err
		}
		evByMonth[m] = round2(v * scale)
	}
	rows.Close()

	// AC (1/2): kesinleşen hakedişlerin GERÇEKLEŞEN MALİYETİ.
	//
	// actual_cost hakediş hesaplanırken yazılır (Faz 11):
	//   AC = dönem brütü (KDV hariç) − maliyeti azaltan kesintilerin KDV hariç tutarı
	// Avans mahsubu, teminat ve stopaj finansman/vergi kalemidir; maliyeti
	// değiştirmez. KDV de devreden bir kalemdir, maliyete girmez.
	//
	// COALESCE: Faz 11 öncesi kesinleşmiş hakedişlerde actual_cost boştur;
	// bu kayıtlar için dönem brütü kullanılır (en yakın doğru değer).
	acByMonth := map[string]float64{}
	rows, err = pool.Query(ctx, `
		SELECT to_char(finalized_at,'YYYY-MM'),
		       sum(CASE WHEN actual_cost > 0 THEN actual_cost ELSE gross_this END)
		FROM progress_payments
		WHERE project_id=$1 AND deleted_at IS NULL AND status='Finalized'
		  AND finalized_at IS NOT NULL
		GROUP BY 1`, pid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m string
		var v float64
		if err := rows.Scan(&m, &v); err != nil {
			rows.Close()
			return nil, err
		}
		acByMonth[m] += v
	}
	rows.Close()

	// AC (2/2): teslim alınmış PO tutarları — son teslimatın ayına yazılır.
	rows, err = pool.Query(ctx, `
		SELECT to_char(max(d.delivered_at),'YYYY-MM'), po.amount
		FROM purchase_orders po
		JOIN deliveries d ON d.po_id = po.id AND d.deleted_at IS NULL
		WHERE po.project_id=$1 AND po.deleted_at IS NULL
		  AND po.status='Delivered' AND po.amount IS NOT NULL
		GROUP BY po.id, po.amount`, pid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m string
		var v float64
		if err := rows.Scan(&m, &v); err != nil {
			rows.Close()
			return nil, err
		}
		acByMonth[m] += v
	}
	rows.Close()

	// Plan ufku: künye tarihleri → yoksa PV girişleri/gerçekleşen aylar → bugün.
	ctrl, err := loadControlSettings(ctx, pool, pid)
	if err != nil {
		return nil, err
	}
	months, plannedPct, source, err := loadPlan(ctx, pool, pid, start, end, evByMonth, acByMonth, now, ctrl.PVSource)
	if err != nil {
		return nil, err
	}
	res.PlanSource = source
	res.SCurve = BuildSCurve(SCurveInput{
		BAC: res.BAC, Months: months,
		PlannedPct: plannedPct, EVByMonth: evByMonth, ACByMonth: acByMonth,
	})

	// Kümülatifler bugünün ayına (ufuk daha kısaysa son aya) göre kesilir.
	asOf := MonthKey(now)
	res.AsOfMonth = asOf
	for _, p := range res.SCurve {
		if p.Month > asOf {
			break
		}
		res.PV, res.EV, res.AC = p.PV, p.EV, p.AC
		res.AsOfMonth = p.Month
	}
	res.SPI = SPI(res.EV, res.PV)
	res.CPI = CPI(res.EV, res.AC)
	rawCPI := 0.0
	if res.AC > 0 {
		rawCPI = res.EV / res.AC
	}
	res.EAC = EAC(res.BAC, res.EV, res.AC, rawCPI)
	res.ETC = ETC(res.EAC, res.AC)
	res.ProgressPct = ProgressPct(res.EV, res.BAC)
	return res, nil
}

// loadPlan — PV dağılımını öncelik sırasıyla türetir:
//  1. pv_plan_entries (manuel aylık dağılım — Faz 9 girişi)
//  2. milestone ağırlıkları (planned_date ayına weight_pct yazılır)
//  3. doğrusal dağılım
func loadPlan(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID,
	start, end *time.Time, evBy, acBy map[string]float64, now time.Time, pvSource string,
) (months []string, plannedPct map[string]float64, source string, err error) {

	// 1) Manuel giriş.
	manual := map[string]float64{}
	rows, err := pool.Query(ctx, `
		SELECT to_char(month,'YYYY-MM'), planned_pct
		FROM pv_plan_entries
		WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY month`, pid)
	if err != nil {
		return nil, nil, "", err
	}
	var manualMonths []string
	for rows.Next() {
		var m string
		var p float64
		if err := rows.Scan(&m, &p); err != nil {
			rows.Close()
			return nil, nil, "", err
		}
		manual[m] = p
		manualMonths = append(manualMonths, m)
	}
	rows.Close()

	// Ufuk: künye tarihleri esas; yoksa eldeki verinin min/max ayı; o da yoksa bugün.
	s, e := start, end
	if s == nil || e == nil {
		lo, hi := "", ""
		consider := func(m string) {
			if m == "" {
				return
			}
			if lo == "" || m < lo {
				lo = m
			}
			if hi == "" || m > hi {
				hi = m
			}
		}
		for _, m := range manualMonths {
			consider(m)
		}
		for m := range evBy {
			consider(m)
		}
		for m := range acBy {
			consider(m)
		}
		consider(MonthKey(now))
		ls, _ := time.Parse("2006-01", lo)
		le, _ := time.Parse("2006-01", hi)
		s, e = &ls, &le
	}
	months = MonthsBetween(*s, *e)

	// 0) Kullanıcı PV kaynağı olarak baseline'ı seçtiyse ve baseline varsa.
	if pvSource == pvSourceBaseline {
		pct, label, ok, err := baselinePlanPct(ctx, pool, pid, months)
		if err != nil {
			return nil, nil, "", err
		}
		if ok {
			return months, pct, label, nil
		}
	}

	if len(manual) > 0 {
		// Manuel girişte ufuk dışı aylar da eğriye dahil edilir.
		set := map[string]bool{}
		for _, m := range months {
			set[m] = true
		}
		for _, m := range manualMonths {
			if !set[m] {
				months = append(months, m)
				set[m] = true
			}
		}
		sortMonths(months)
		return months, manual, "manual", nil
	}

	// 2) Milestone ağırlıkları.
	byMonth := map[string]float64{}
	total := 0.0
	rows, err = pool.Query(ctx, `
		SELECT to_char(planned_date,'YYYY-MM'), weight_pct
		FROM milestones
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND planned_date IS NOT NULL AND weight_pct IS NOT NULL`, pid)
	if err != nil {
		return nil, nil, "", err
	}
	for rows.Next() {
		var m string
		var w float64
		if err := rows.Scan(&m, &w); err != nil {
			rows.Close()
			return nil, nil, "", err
		}
		byMonth[m] += w
		total += w
	}
	rows.Close()
	if total > 0 {
		// Ağırlıklar 100'e normalize edilir (Σweight ≠ 100 girilmiş olabilir).
		plannedPct = map[string]float64{}
		set := map[string]bool{}
		for _, m := range months {
			set[m] = true
		}
		for m, w := range byMonth {
			plannedPct[m] = w / total * 100
			if !set[m] {
				months = append(months, m)
				set[m] = true
			}
		}
		sortMonths(months)
		return months, plannedPct, "milestones", nil
	}

	// 3) Doğrusal.
	return months, LinearPlanPct(months), "linear", nil
}

func sortMonths(m []string) {
	// "YYYY-MM" leksikografik = kronolojik.
	for i := 1; i < len(m); i++ {
		for j := i; j > 0 && m[j] < m[j-1]; j-- {
			m[j], m[j-1] = m[j-1], m[j]
		}
	}
}

// baselinePlanPct — son dondurulmuş iş programı baseline'ından AY BAZLI
// planlanan artış yüzdesini üretir. BuildSCurve değerleri kümülatif topladığı
// için her ay, ayın sonundaki kümülatif planlanan ilerleme ile bir önceki ayın
// sonu arasındaki fark olarak verilir. Baseline yoksa ok=false döner ve çağıran
// mevcut öncelik sırasına düşer.
func baselinePlanPct(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID, months []string,
) (pct map[string]float64, label string, ok bool, err error) {
	if len(months) == 0 {
		return nil, "", false, nil
	}
	var rev int
	var snap []byte
	err = pool.QueryRow(ctx, `
		SELECT revision_no, snapshot FROM schedule_baselines
		WHERE project_id=$1 ORDER BY revision_no DESC LIMIT 1`, pid).Scan(&rev, &snap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	first, err := time.Parse("2006-01", months[0])
	if err != nil {
		return nil, "", false, err
	}
	// Serinin başlangıcından önceki planlanan ilerleme taban olarak alınır;
	// böylece baseline ufuktan önce başlamışsa ilk ayın artışı şişmez.
	prev, err := schedule.PlannedPhysicalFromSnapshot(snap, first.AddDate(0, 0, -1))
	if err != nil {
		return nil, "", false, err
	}
	pct = make(map[string]float64, len(months))
	for _, m := range months {
		t, err := time.Parse("2006-01", m)
		if err != nil {
			return nil, "", false, err
		}
		cum, err := schedule.PlannedPhysicalFromSnapshot(snap, t.AddDate(0, 1, -1))
		if err != nil {
			return nil, "", false, err
		}
		pct[m] = cum - prev
		prev = cum
	}
	return pct, fmt.Sprintf("baseline:R%d", rev), true, nil
}
