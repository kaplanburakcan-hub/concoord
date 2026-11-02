package reportpdf

import (
	"fmt"
	"strings"
	"time"

	k "github.com/ipks/ipks/backend/internal/pdfkit"
)

// Haftalık rapor satırları.
type (
	WeeklyDay struct {
		Date             time.Time
		Reported         bool
		Manpower         int
		TempMin, TempMax *float64
		Cond             string
	}
	WeeklyDisc struct {
		Name  string
		Items []string
	}
	WeeklyDelivery struct {
		Supplier, Material, Unit string
		Qty                      float64
	}
	WeeklyStock struct {
		Name, Unit string
		Cur, Min   float64
	}
	WeeklyLook struct {
		Name          string
		Start, Finish time.Time
	}
	WeeklyTask struct {
		Title, Assignee, Status, Priority string
		Due                               time.Time // sıfır = termin yok
	}
)

// WeeklyData — haftalık ilerleme raporunun PDF girdisi.
type WeeklyData struct {
	ProjectName, ProjectCode string
	WeekNo                   int
	Start, End               time.Time
	DaysReported             int
	ManpowerDays             int
	EquipHours               float64
	WorkEntries              int
	DelayHours               float64
	Days                     []WeeklyDay // pazartesiden pazara 7 gün
	PPCPlanned, PPCCompleted int
	Disciplines              []WeeklyDisc
	Look                     []WeeklyLook
	Deliveries               []WeeklyDelivery
	LowStock                 []WeeklyStock
	OpenTasks, TasksDue      int
	PendingMARs              int
	PendingPayments          []string
	PendingPOs, OverduePOs   int
	Tasks                    []WeeklyTask
	Photos                   []Photo
	GeneratedAt              time.Time
}

var dayShort = []string{"Paz", "Pzt", "Sal", "Çar", "Per", "Cum", "Cmt"}

// BuildWeekly — haftalık ilerleme raporu PDF'i (taslak "Haftalık A — Hafta Panosu").
func BuildWeekly(in WeeklyData) []byte {
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now()
	}
	period := fmt.Sprintf("%s – %d %s %d", fmt.Sprintf("%d", in.Start.Day()), in.End.Day(), TRMonth(int(in.End.Month())), in.End.Year())
	if in.Start.Month() != in.End.Month() {
		period = fmt.Sprintf("%d %s – %d %s %d", in.Start.Day(), TRMonth(int(in.Start.Month())), in.End.Day(), TRMonth(int(in.End.Month())), in.End.Year())
	}
	footer := fmt.Sprintf("ConCoord · Haftalık İlerleme Raporu · Hafta %d · %s", in.WeekNo, in.ProjectCode)
	p := newPage("HAFTALIK İLERLEME RAPORU", footer, fmt.Sprintf("Hafta %d · %s", in.WeekNo, in.ProjectCode))
	d := p.d
	p.bigHeader("HAFTALIK İLERLEME RAPORU", fmt.Sprintf("Hafta %d", in.WeekNo), 26, period,
		fmt.Sprintf("%s · %s", in.ProjectName, in.ProjectCode))

	// ── KPI ─────────────────────────────────────────────────────────────────
	p.kpiStrip([]kpi{
		{Label: "Raporlanan gün", Value: fmt.Sprintf("%d / 7", in.DaysReported)},
		{Label: "Adam-gün", Value: Num(float64(in.ManpowerDays), 0)},
		{Label: "Ekipman saati", Value: Qty(in.EquipHours)},
		{Label: "İmalat kalemi", Value: fmt.Sprint(in.WorkEntries)},
		{Label: "Gecikme", Value: Num(in.DelayHours, 1) + " sa", Accent: true},
	}, 44)

	// ── Gün gün + PPC ───────────────────────────────────────────────────────
	chartW := cW*0.62 - 6
	ppcW := cW - chartW - 12
	const boxH = 112.0
	p.need(boxH + 24)
	d.Text(mL, p.y+7, "GÜN GÜN PERSONEL VE HAVA", true, 7.2, k.Navy)
	d.Text(mL+chartW+12, p.y+7, "PLANLANAN TAMAMLANMA (PPC)", true, 7.2, k.Navy)
	p.y += 12
	p.cardBox(mL, p.y, chartW, boxH)
	maxMan := 1
	for _, dy := range in.Days {
		if dy.Manpower > maxMan {
			maxMan = dy.Manpower
		}
	}
	n := len(in.Days)
	if n == 0 {
		n = 1
	}
	colW := (chartW - 16) / float64(n)
	baseY := p.y + 66
	for i, dy := range in.Days {
		cx := mL + 8 + float64(i)*colW + colW/2
		bw := colW * 0.52
		if bw > 26 {
			bw = 26
		}
		label := "—"
		h := 2.0
		colr := k.Hex(0xc4cfdd)
		if dy.Reported {
			label = fmt.Sprint(dy.Manpower)
			h = float64(dy.Manpower) / float64(maxMan) * 46
			if h < 2 {
				h = 2
			}
			colr = k.Mid
			if dy.Manpower == maxMan {
				colr = k.Navy
			}
			if dy.Date.Weekday() == time.Sunday || dy.Date.Weekday() == time.Saturday {
				colr = k.Steel
			}
		}
		d.FillRect(cx-bw/2, baseY-h, bw, h, colr)
		d.TextC(cx, baseY-h-3, label, true, 7, k.Ink)
		d.TextC(cx, baseY+9.5, dayShort[int(dy.Date.Weekday())], true, 7, k.Ink)
		temp := ""
		if dy.TempMax != nil {
			temp = Qty(*dy.TempMax) + "°"
		}
		d.TextC(cx, baseY+19, temp, false, 6.2, k.Dim)
		cond := dy.Cond
		fg := k.Dim
		if strings.Contains(strings.ToLower(cond), "yağ") || strings.Contains(strings.ToLower(cond), "rüzgâr") {
			fg = k.WarnFg
		}
		d.TextC(cx, baseY+27.5, d.Fit(strings.ToLower(cond), false, 5.6, colW-2), false, 5.6, fg)
	}
	d.HLine(mL+8, mL+chartW-8, baseY+0.5, k.Navy, 1)

	px := mL + chartW + 12
	p.cardBox(px, p.y, ppcW, boxH)
	if in.PPCPlanned > 0 {
		frac := float64(in.PPCCompleted) / float64(in.PPCPlanned)
		cx, cy := px+42, p.y+boxH/2
		d.RingSeg(cx, cy, 31, 23, 0, 359.9, k.Track)
		d.RingSeg(cx, cy, 31, 23, 0, frac*359.9, k.Mid)
		d.TextC(cx, cy+5, fmt.Sprintf("%%%d", int(frac*100+0.5)), true, 14, k.Navy)
		d.Text(px+84, p.y+44, fmt.Sprintf("%d / %d iş", in.PPCCompleted, in.PPCPlanned), true, 8, k.Ink)
		d.Text(px+84, p.y+53, "tamamlandı / planlandı", false, 5.9, k.Dim)
		d.Text(px+84, p.y+68, fmt.Sprintf("%d iş devir", in.PPCPlanned-in.PPCCompleted), true, 7.6, k.WarnFg)
		d.Text(px+84, p.y+77, "iş programına göre", false, 5.9, k.Dim)
	} else {
		d.Text(px+10, p.y+boxH/2-2, "Bu hafta bitmesi planlanan", false, 7, k.Dim)
		d.Text(px+10, p.y+boxH/2+8, "iş programı kalemi yok.", false, 7, k.Dim)
	}
	p.y += boxH + 12

	// ── Disiplin + ileri bakış ──────────────────────────────────────────────
	half := (cW - 14) / 2
	p.need(130)
	d.Text(mL, p.y+7, "DİSİPLİN BAZLI BU HAFTA", true, 7.2, k.Navy)
	d.Text(mL+half+14, p.y+7, "3 HAFTALIK İLERİ BAKIŞ", true, 7.2, k.Navy)
	p.y += 12
	yTop := p.y
	if len(in.Disciplines) == 0 {
		d.Text(mL+4, p.y+9, "Haftada imalat kaydı yok.", false, bodySize, k.Dim)
		p.y += 14
	} else {
		rows := make([][]string, 0, len(in.Disciplines))
		for _, dc := range in.Disciplines {
			feat := strings.Join(firstN(dc.Items, 2), "; ")
			rows = append(rows, []string{dc.Name, feat, fmt.Sprint(len(dc.Items))})
		}
		if len(rows) > 7 {
			rows = rows[:7]
		}
		p.tableAt(mL, fillCols([]col{{"Disiplin", 62, false}, {"Öne çıkan imalat", -1, false}, {"Kalem", 32, true}}, half), rows, true)
	}
	yLeft := p.y
	p.y = yTop
	lx := mL + half + 14
	p.cardBox(lx, p.y, half, 116)
	if len(in.Look) == 0 {
		d.Text(lx+8, p.y+20, "Önümüzdeki 3 haftada süren iş", false, 7, k.Dim)
		d.Text(lx+8, p.y+30, "programı kalemi yok.", false, 7, k.Dim)
	} else {
		labW := 90.0
		gx := lx + 8 + labW
		gw := half - 16 - labW
		weekStart := in.End.AddDate(0, 0, 1)
		for w := 0; w < 3; w++ {
			d.Text(gx+gw/3*float64(w)+2, p.y+11, fmt.Sprintf("H%d", in.WeekNo+1+w), true, 6.2, k.Dim)
			if w > 0 {
				d.VLine(gx+gw/3*float64(w), p.y+14, p.y+108, k.Rule, 0.5)
			}
		}
		ry := p.y + 20
		for _, l := range in.Look {
			if ry+14 > p.y+112 {
				break
			}
			d.Text(lx+8, ry+7.5, d.Fit(l.Name, false, 6.6, labW-6), false, 6.6, k.Ink)
			d.FillRect(gx, ry+2, gw, 6, k.Track)
			s := l.Start.Sub(weekStart).Hours() / 24
			e := l.Finish.Sub(weekStart).Hours()/24 + 1
			if s < 0 {
				s = 0
			}
			if e > 21 {
				e = 21
			}
			if e > s {
				d.FillRect(gx+gw*s/21, ry+2, gw*(e-s)/21, 6, k.Mid)
			}
			ry += 14.5
		}
	}
	if p.y+116 > yLeft {
		yLeft = p.y + 116
	}
	p.y = yLeft + 12

	// ── Teslimat + bekleyenler ──────────────────────────────────────────────
	lw := cW * 0.57
	rw := cW - lw - 14
	p.need(110)
	d.Text(mL, p.y+7, "HAFTANIN TESLİMATLARI", true, 7.2, k.Navy)
	d.Text(mL+lw+14, p.y+7, "BEKLEYEN İŞLER", true, 7.2, k.Navy)
	p.y += 12
	yTop = p.y
	if len(in.Deliveries) == 0 {
		d.Text(mL+4, p.y+9, "Bu hafta teslimat kaydı yok.", false, bodySize, k.Dim)
		p.y += 14
	} else {
		rows := make([][]string, 0, len(in.Deliveries))
		for _, dv := range in.Deliveries {
			rows = append(rows, []string{dv.Supplier, dv.Material, Qty(dv.Qty) + " " + dv.Unit})
		}
		if len(rows) > 6 {
			rows = rows[:6]
		}
		p.tableAt(mL, fillCols([]col{{"Tedarikçi", 110, false}, {"Malzeme", -1, false}, {"Miktar", 62, true}}, lw), rows, false)
	}
	for _, s := range firstNStock(in.LowStock, 2) {
		p.y += p.noteBox(mL, p.y+4, lw, "Stok uyarısı", fmt.Sprintf("%s: %s / min %s %s — sipariş önerilir.", s.Name, Qty(s.Cur), Qty(s.Min), s.Unit)) + 4
	}
	yLeft = p.y
	p.y = yTop
	cards := []struct{ lab, val, sub string }{
		{"Açık görev", fmt.Sprint(in.OpenTasks), fmt.Sprintf("%d'i bu hafta dolacak", in.TasksDue)},
		{"Malzeme onayı", fmt.Sprint(in.PendingMARs), "onay bekliyor"},
		{"Hakediş", fmt.Sprint(len(in.PendingPayments)), strings.Join(firstN(in.PendingPayments, 2), " · ")},
		{"Sipariş", fmt.Sprint(in.PendingPOs), fmt.Sprintf("teslim bekleyen · %d geciken", in.OverduePOs)},
	}
	cw := (rw - 8) / 2
	for i, c := range cards {
		x := mL + lw + 14 + float64(i%2)*(cw+8)
		y := p.y + float64(i/2)*50
		p.cardBox(x, y, cw, 44)
		d.Text(x+8, y+11, k.TrUpper(c.lab), true, 5.7, k.Dim)
		d.Text(x+8, y+29, c.val, true, 15, k.Navy)
		d.Text(x+8, y+39, d.Fit(c.sub, false, 5.9, cw-14), false, 5.9, k.Dim)
	}
	if p.y+100 > yLeft {
		yLeft = p.y + 100
	}
	p.y = yLeft + 12

	// ── Kritik konular ──────────────────────────────────────────────────────
	p.section("Kritik konular ve alınacak aksiyonlar", "açık görevlerden, öncelik sırasıyla")
	if len(in.Tasks) == 0 {
		p.emptyNote("Açık görev yok.")
	} else {
		p.need(12.5 + rowH*float64(len(in.Tasks)))
		cols := fillCols([]col{{"#", 18, false}, {"Konu", -1, false}, {"Sorumlu", 96, false}, {"Termin", 56, false}, {"Durum", 56, false}}, cW)
		p.tableHead(mL, cols)
		for i, t := range in.Tasks {
			x := mL
			cells := []string{fmt.Sprint(i + 1), t.Title, orDash(t.Assignee), "—", ""}
			overdue := false
			if !t.Due.IsZero() {
				cells[3] = DMY(t.Due)
				overdue = t.Due.Before(in.GeneratedAt.Truncate(24*time.Hour)) && t.Status != "Done"
			}
			for ci, c := range cols[:4] {
				bold := ci == 0
				d.Text(x+4, p.y+9.4, d.Fit(cells[ci], bold, bodySize, c.W-8), bold, bodySize, k.Ink)
				x += c.W
			}
			st, fg, bg := taskStatus(t.Status)
			if overdue {
				st, fg, bg = "Gecikti", k.WarnFg, k.Hex(0xfdeed6)
			}
			d.Chip(x+4, p.y+9.4, st, fg, bg)
			d.HLine(mL, mL+cW, p.y+rowH, k.Rule, 0.5)
			p.y += rowH
		}
	}

	// ── Fotoğraflar ─────────────────────────────────────────────────────────
	if len(in.Photos) == 0 {
		p.y += 8
		p.need(24)
		p.section("Haftanın fotoğrafları", "")
		p.emptyNote("Bu hafta Fotoğraflar sayfasına yüklenen fotoğraf yok.")
	} else {
		p.slimHeader()
		p.section("Haftanın fotoğrafları", fmt.Sprintf("%d fotoğraf", len(in.Photos)))
		rest := in.Photos
		if in.Photos[0].Featured {
			p.photoFeatured(in.Photos[0], 205)
			rest = in.Photos[1:]
		}
		p.photoGrid(rest, 3, 100)
	}
	return p.finish(in.GeneratedAt)
}

func taskStatus(s string) (string, k.RGB, k.RGB) {
	switch s {
	case "InProgress":
		return "Devam", k.Mid, k.Hex(0xe1e9f5)
	case "Review":
		return "Kontrol", k.Mid, k.Hex(0xe1e9f5)
	case "Todo":
		return "Yapılacak", k.Dim, k.Hex(0xe8edf3)
	}
	return "Bekliyor", k.Dim, k.Hex(0xe8edf3)
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func firstNStock(s []WeeklyStock, n int) []WeeklyStock {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
