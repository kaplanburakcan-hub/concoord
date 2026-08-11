package dashboard

// evm_db.go — EVM girdilerini DB'den derler ve saf çekirdeğe (evm.go) besler.
// Dashboard ucu, aylık rapor snapshot'ı ve eşik uyarı taraması AYNI fonksiyonu
// kullanır — üç yerde üç farklı hesap olmaz (tek doğruluk kaynağı).

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// Parasal ilerleme (Panel'de "Fiziki ilerleme"nin yanındaki halka grafik):
	// AC (kesinleşen hakediş + teslim alınmış satınalma) / sözleşme bedeli.
	// BAC (Yapım Bütçesi) ile KARIŞTIRILMAMALI — projects.contract_amount
	// künyedeki ayrı bir alandır, boş bırakılabilir.
	ContractAmount       *float64 `json:"contract_amount,omitempty"`
	FinancialProgressPct float64  `json:"financial_progress_pct"`
}

// LoadEVM — proje için EVM verisini derler. Kümülatif PV/EV/AC ve endeksler
// "bugünün ayı"na (proje bitmişse son plan ayına) kadar hesaplanır; S-eğrisi
// tüm plan ufkunu içerir.
func LoadEVM(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID, now time.Time) (*EVMResult, error) {
	res := &EVMResult{}

	// Proje künyesi: bütçe (BAC) + sözleşme bedeli + plan ufku.
	var budget, contractAmt *float64
	var start, end *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT budget_total, contract_amount, currency, start_date, end_date
		FROM projects WHERE id=$1 AND deleted_at IS NULL`, pid).
		Scan(&budget, &contractAmt, &res.Currency, &start, &end); err != nil {
		return nil, err
	}
	if budget != nil {
		res.BAC = *budget
	}
	res.ContractAmount = contractAmt

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
	months, plannedPct, source, err := loadPlan(ctx, pool, pid, start, end, evByMonth, acByMonth, now)
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
	if res.ContractAmount != nil && *res.ContractAmount > 0 {
		res.FinancialProgressPct = round2(res.AC / *res.ContractAmount * 100)
	}
	return res, nil
}

// loadPlan — PV dağılımını öncelik sırasıyla türetir:
//  1. pv_plan_entries (manuel aylık dağılım — Faz 9 girişi)
//  2. milestone ağırlıkları (planned_date ayına weight_pct yazılır)
//  3. doğrusal dağılım
func loadPlan(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID,
	start, end *time.Time, evBy, acBy map[string]float64, now time.Time,
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
