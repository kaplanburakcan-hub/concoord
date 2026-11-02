package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/ipks/ipks/backend/internal/reportpdf"
	"github.com/ipks/ipks/backend/internal/reportphotos"
	"github.com/ipks/ipks/backend/internal/storage"
)

// weeklyPDFData — snapshot'ı PDF girdisine çevirir (fotoğraf baytları hariç).
// PDF'e giren her değer snapshot'tan gelir.
func weeklyPDFData(sn Snapshot) reportpdf.WeeklyData {
	start, _ := time.Parse("2006-01-02", sn.PeriodStart)
	end, _ := time.Parse("2006-01-02", sn.PeriodEnd)
	out := reportpdf.WeeklyData{
		ProjectName: sn.ProjectName, ProjectCode: sn.ProjectCode, WeekNo: sn.WeekNo,
		Start: start, End: end,
		DaysReported: sn.Totals.DaysReported, ManpowerDays: sn.Totals.ManpowerDays,
		EquipHours: sn.Totals.EquipmentHours, WorkEntries: sn.Totals.WorkEntryCount,
		DelayHours:  sn.DelayHours,
		GeneratedAt: sn.GeneratedAt,
		OpenTasks:   sn.OpenTasks, TasksDue: sn.TasksDueWeek, PendingMARs: sn.PendingMARs,
	}
	byDate := map[string]SnapDay{}
	for _, d := range sn.Days {
		byDate[d.Date] = d
	}
	for i := 0; i < 7; i++ {
		day := start.AddDate(0, 0, i)
		wd := reportpdf.WeeklyDay{Date: day}
		if sd, ok := byDate[day.Format("2006-01-02")]; ok {
			wd.Reported, wd.Manpower, wd.Cond = true, sd.ManpowerTotal, sd.WeatherCond
			wd.TempMin, wd.TempMax = sd.TempMin, sd.TempMax
		}
		out.Days = append(out.Days, wd)
	}
	if sn.PPC != nil {
		out.PPCPlanned, out.PPCCompleted = sn.PPC.Planned, sn.PPC.Completed
	}
	for _, ds := range sn.DisciplineSections {
		out.Disciplines = append(out.Disciplines, reportpdf.WeeklyDisc{Name: ds.Discipline, Items: ds.ThisWeek})
	}
	for _, l := range sn.LookAhead {
		s, e1 := time.Parse("2006-01-02", l.Start)
		f, e2 := time.Parse("2006-01-02", l.Finish)
		if e1 == nil && e2 == nil {
			out.Look = append(out.Look, reportpdf.WeeklyLook{Name: l.Name, Start: s, Finish: f})
		}
	}
	type dk struct{ sup, mat, unit string }
	agg := map[dk]*reportpdf.WeeklyDelivery{}
	var order []dk
	for _, dv := range sn.Deliveries {
		for _, it := range dv.Items {
			key := dk{dv.Supplier, it.MaterialName, it.Unit}
			if a, ok := agg[key]; ok {
				a.Qty += it.Qty
				continue
			}
			agg[key] = &reportpdf.WeeklyDelivery{Supplier: dv.Supplier, Material: it.MaterialName, Unit: it.Unit, Qty: it.Qty}
			order = append(order, key)
		}
	}
	for _, key := range order {
		out.Deliveries = append(out.Deliveries, *agg[key])
	}
	for _, st := range sn.Stock {
		if st.BelowMin {
			out.LowStock = append(out.LowStock, reportpdf.WeeklyStock{Name: st.MalzemeAdi, Unit: st.Birim, Cur: st.MevcutMiktar, Min: st.MinStok})
		}
	}
	for _, p := range sn.PendingPayments {
		out.PendingPayments = append(out.PendingPayments, fmt.Sprintf("%s #%d", p.Subcontractor, p.PeriodNo))
	}
	out.PendingPOs = len(sn.PendingPOs)
	for _, po := range sn.PendingPOs {
		if po.Overdue {
			out.OverduePOs++
		}
	}
	for _, t := range sn.CriticalTasks {
		wt := reportpdf.WeeklyTask{Title: t.Title, Assignee: t.Assignee, Status: t.Status, Priority: t.Priority}
		if due, err := time.Parse("2006-01-02", t.Due); err == nil {
			wt.Due = due
		}
		out.Tasks = append(out.Tasks, wt)
	}
	return out
}

var dayTag = []string{"PAZ", "PZT", "SAL", "ÇAR", "PER", "CUM", "CMT"}

// buildWeeklyPDF — snapshot ve (varsa) depodaki fotoğraflardan haftalık PDF üretir.
func buildWeeklyPDF(ctx context.Context, store *storage.Client, sn Snapshot) []byte {
	data := weeklyPDFData(sn)
	loc, _ := time.LoadLocation(reportphotos.TZ)
	for i, ph := range sn.Photos {
		taken, err := time.Parse(time.RFC3339, ph.TakenAt)
		if err != nil {
			continue
		}
		if loc != nil {
			taken = taken.In(loc)
		}
		sub := taken.Format("02.01.2006 15:04")
		if c := reportphotos.CategoryTR(ph.Category); c != "" {
			sub += " · " + c
		}
		data.Photos = append(data.Photos, reportpdf.Photo{
			Data: reportphotos.Fetch(ctx, store, ph.Key), Title: ph.Title, Sub: sub,
			Tag: dayTag[int(taken.Weekday())], Featured: i == 0,
		})
	}
	return reportpdf.BuildWeekly(data)
}
