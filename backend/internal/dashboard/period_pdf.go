package dashboard

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ipks/ipks/backend/internal/reportpdf"
	"github.com/ipks/ipks/backend/internal/reportphotos"
	"github.com/ipks/ipks/backend/internal/storage"
)

var dedLabelTR = map[string]string{
	"AdvanceOffset": "Avans mahsubu", "Retention": "Teminat", "Withholding": "Stopaj",
	"Tax": "Vergi", "OHSPenalty": "İSG ceza",
}

func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// periodPDFData — snapshot'ı PDF girdisine çevirir (fotoğraf baytları hariç).
// PDF'e giren her sayı snapshot'tan gelir; anlatı metni de bu sayılardan kuraldır.
func periodPDFData(sn MonthlySnapshot) reportpdf.PeriodData {
	kind := sn.Kind
	if kind == "" {
		kind = KindMonthly
	}
	// v2 öncesi snapshot'larda dikkat listesi yoktur; mevcut rakamlardan türetilir.
	if sn.Thresholds == nil && len(sn.Attention) == 0 {
		sn.Attention = buildAttention(&sn)
	}
	title := sn.Label
	if kind == KindMonthly {
		title = fmt.Sprintf("%s %d", monthNameTR(fmt.Sprintf("%04d-%02d", sn.Year, sn.Month)), sn.Year)
	}
	out := reportpdf.PeriodData{
		Kind: kind, ProjectName: sn.ProjectName, ProjectCode: sn.ProjectCode, Currency: sn.Currency, Title: title,
		Start: parseDate(sn.PeriodStart), End: parseDate(sn.PeriodEnd),
		BAC: sn.EVM.BAC, PV: sn.EVM.PV, EV: sn.EVM.EV, AC: sn.EVM.AC,
		SPI: sn.EVM.SPI, CPI: sn.EVM.CPI, EAC: sn.EVM.EAC, ETC: sn.EVM.ETC,
		ProgressPct: sn.EVM.ProgressPct, BudgetVariance: sn.BudgetVariance,
		PlanSource: sn.EVM.PlanSource, AsOfMonth: sn.EVM.AsOfMonth,
		MonthGross: sn.MonthGross, MonthDed: sn.MonthDeductions, MonthNet: sn.MonthNet,
		OHSOpened: sn.OHS.FindingsOpened, OHSClosed: sn.OHS.FindingsClosed,
		OHSOpen: sn.OHS.OpenTotal, OHSCritical: sn.OHS.OpenCritical,
		PenaltyCount: sn.OHS.PenaltiesCount, PenaltyTotal: sn.OHS.PenaltiesTotal,
		POsOrdered: sn.Procurement.POsOrdered, POsDelivered: sn.Procurement.POsDelivered,
		POsOverdue: sn.Procurement.POsOverdue, PRsPending: sn.Procurement.PRsPending,
		GeneratedAt: sn.GeneratedAt,
	}
	if sn.Thresholds != nil {
		out.CPIMin, out.SPIMin = sn.Thresholds.CPIMin, sn.Thresholds.SPIMin
	}
	for _, c := range sn.EVM.SCurve {
		out.Curve = append(out.Curve, reportpdf.CurvePt{Month: c.Month, PV: c.PV, EV: c.EV, AC: c.AC})
	}
	for _, p := range sn.FinalizedPayments {
		out.Payments = append(out.Payments, reportpdf.PayRow{Sub: p.Subcontractor, PeriodNo: p.PeriodNo, Gross: p.GrossThis, Ded: p.Deductions, Net: p.NetPayable})
	}
	for t, v := range sn.DeductionsByType {
		l := dedLabelTR[t]
		if l == "" {
			l = "Diğer"
		}
		out.Deductions = append(out.Deductions, reportpdf.DedRow{Label: l, Amount: v})
	}
	sort.Slice(out.Deductions, func(i, j int) bool { return out.Deductions[i].Amount > out.Deductions[j].Amount })
	for _, m := range sn.Milestones {
		out.Milestones = append(out.Milestones, reportpdf.MsRow{Name: m.Name, Planned: parseDate(m.PlannedDate), Actual: parseDate(m.ActualDate), Status: m.Status, Late: m.Late})
	}
	for _, c := range sn.DelayCauses {
		out.Delays = append(out.Delays, reportpdf.DailyDelayRow{Label: c.Label, Hours: c.Hours})
	}
	for _, a := range sn.Attention {
		out.Attention = append(out.Attention, reportpdf.AttnRow{Level: a.Level, Title: a.Title, Detail: a.Detail})
	}
	for _, m := range sn.Months {
		out.Months = append(out.Months, reportpdf.MonthRow{
			Label: monthNameTR(m.Month), Delta: m.ProgressD, SPI: m.SPI, CPI: m.CPI, Net: m.NetPaid,
			ManDays: m.ManDays, Delay: m.DelayHours, Opened: m.Opened, Closed: m.Closed, Events: m.Events,
		})
	}
	for _, r := range sn.Risks {
		out.Risks = append(out.Risks, reportpdf.RiskRow{No: r.No, Prob: r.Prob, Impact: r.Impact, Title: r.Title, Status: r.Status})
	}
	for _, d := range sn.Decisions {
		out.Decisions = append(out.Decisions, reportpdf.DecisionRow{Date: parseDate(d.Date), Text: d.Text})
	}
	if sn.Next != nil {
		nx := &reportpdf.NextRow{Label: sn.Next.Label, ProgressFrom: sn.Next.ProgressFrom, ProgressTo: sn.Next.ProgressTo}
		for _, m := range sn.Next.Milestones {
			nx.Milestones = append(nx.Milestones, reportpdf.MsRow{Name: m.Name, Planned: parseDate(m.PlannedDate), Status: m.Status})
		}
		out.Next = nx
	}
	if kind == KindQuarterly {
		out.Narrative = quarterNarrative(sn)
	}
	return out
}

// quarterNarrative — çeyrek yönetici değerlendirmesi: snapshot rakamlarından
// kuralla üretilen iki kısa paragraf.
func quarterNarrative(sn MonthlySnapshot) []string {
	var paras []string
	gained := 0.0
	for _, m := range sn.Months {
		gained += m.ProgressD
	}
	plan := pct(sn.EVM.PV, sn.EVM.BAC)
	s := fmt.Sprintf("%s döneminde proje fiziki olarak %%%s ilerledi ve kümülatif gerçekleşme %%%s oldu", sn.Label, trf(gained, 1), trf(sn.EVM.ProgressPct, 1))
	if sn.EVM.BAC > 0 {
		diff := plan - sn.EVM.ProgressPct
		switch {
		case diff > 0.05:
			s += fmt.Sprintf("; planın (%%%s) %s puan gerisinde.", trf(plan, 1), trf(diff, 1))
		case diff < -0.05:
			s += fmt.Sprintf("; planın (%%%s) %s puan ilerisinde.", trf(plan, 1), trf(-diff, 1))
		default:
			s += fmt.Sprintf("; plan (%%%s) ile aynı hizada.", trf(plan, 1))
		}
	} else {
		s += "."
	}
	paras = append(paras, s)

	var s2 string
	if sn.EVM.CPI > 0 {
		s2 = "Maliyet performansı CPI " + trf(sn.EVM.CPI, 2)
		switch {
		case sn.BudgetVariance > 0:
			s2 += fmt.Sprintf("; tamamlanma tahmini bütçenin %s altında.", fmtMil(sn.BudgetVariance))
		case sn.BudgetVariance < 0:
			s2 += fmt.Sprintf("; tamamlanma tahmini bütçenin %s üzerinde.", fmtMil(-sn.BudgetVariance))
		default:
			s2 += "."
		}
	}
	if len(sn.DelayCauses) > 0 {
		top := sn.DelayCauses[0]
		if s2 != "" {
			s2 += " "
		}
		s2 += fmt.Sprintf("En büyük gecikme nedeni %s (%s saat).", lowerFirst(top.Label), trf(top.Hours, 1))
	}
	late, done := 0, 0
	for _, m := range sn.Milestones {
		if m.Late {
			late++
		}
		if m.Status == "Completed" && len(m.ActualDate) >= 7 && m.ActualDate >= sn.PeriodStart && m.ActualDate <= sn.PeriodEnd {
			done++
		}
	}
	if done > 0 || late > 0 {
		if s2 != "" {
			s2 += " "
		}
		s2 += fmt.Sprintf("Çeyrekte %d kilometre taşı tamamlandı, %d kilometre taşı gecikmede.", done, late)
	}
	if s2 != "" {
		paras = append(paras, s2)
	}
	return paras
}

// lowerFirst — yalnızca baş harfi Türkçe kurala göre küçültür (İ → i, I → ı).
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	switch {
	case r[0] == 'İ':
		r[0] = 'i'
	case r[0] == 'I':
		r[0] = 'ı'
	case r[0] >= 'A' && r[0] <= 'Z':
		r[0] += 32
	}
	return string(r)
}

// buildPeriodPDF — snapshot ve depodaki fotoğraflardan dönem PDF'i üretir.
func buildPeriodPDF(ctx context.Context, store *storage.Client, sn MonthlySnapshot) []byte {
	data := periodPDFData(sn)
	loc, _ := time.LoadLocation(reportphotos.TZ)
	groups := map[string][]reportpdf.Photo{}
	var order []string
	for _, ph := range sn.Photos {
		t, err := time.Parse(time.RFC3339, ph.TakenAt)
		if err != nil {
			continue
		}
		if loc != nil {
			t = t.In(loc)
		}
		sub := t.Format("02.01.2006 15:04")
		if c := reportphotos.CategoryTR(ph.Category); c != "" {
			sub += " · " + c
		}
		label := ""
		if sn.Kind == KindQuarterly {
			label = monthNameTR(t.Format("2006-01"))
		}
		if _, ok := groups[label]; !ok {
			order = append(order, label)
		}
		groups[label] = append(groups[label], reportpdf.Photo{Data: reportphotos.Fetch(ctx, store, ph.Key), Title: ph.Title, Sub: sub})
	}
	for _, l := range order {
		data.Photos = append(data.Photos, reportpdf.PhotoGroup{Label: l, Photos: groups[l]})
	}
	return reportpdf.BuildPeriod(data)
}
