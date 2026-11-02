package reportpdf

import (
	"fmt"
	"strings"
	"time"

	k "github.com/ipks/ipks/backend/internal/pdfkit"
)

// Günlük rapor satırları.
type (
	DailyManRow struct {
		Trade, Company string
		Headcount      int
	}
	DailyStaffRow struct{ Name, Title, Group, Status string }
	DailyEquipRow struct {
		Name       string
		Count      int
		Hours      *float64
		IdleReason string
	}
	DailyWorkRow struct {
		Poz, Location, Description string
		Qty                        *float64
		Unit, Discipline           string
	}
	DailyDelayRow struct {
		Label string
		Hours float64
		Note  string
	}
	DailyCashRow struct {
		Description, Receipt string
		Amount               float64
	}
)

// DailyData — günlük saha raporunun PDF girdisi.
type DailyData struct {
	ProjectName, ProjectCode string
	Date                     time.Time
	RevisionNo               int
	Author, AuthorTitle      string
	Status                   string // Draft | Submitted
	SubmittedAt              *time.Time
	IssueFlagged             bool
	WeatherCond              string
	TempMin, TempMax         *float64
	Manpower                 []DailyManRow
	Staff                    []DailyStaffRow
	Equipment                []DailyEquipRow
	Work                     []DailyWorkRow
	Delays                   []DailyDelayRow
	Cash                     []DailyCashRow
	Notes                    string
	Photos                   []Photo // ilki kapak fotoğrafı olabilir (Featured)
	GeneratedAt              time.Time
}

// BuildDaily — günlük saha raporu PDF'i (taslak "Günlük A — Saha Künyesi").
func BuildDaily(in DailyData) []byte {
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now()
	}
	dateStr := fmt.Sprintf("%d %s %d", in.Date.Day(), TRMonth(int(in.Date.Month())), in.Date.Year())
	footer := fmt.Sprintf("ConCoord · Günlük Saha Raporu · %s · %s", in.ProjectCode, DMY(in.Date))
	p := newPage("GÜNLÜK SAHA RAPORU", footer, fmt.Sprintf("%s · %s", DMY(in.Date), in.ProjectCode))
	d := p.d

	rev := ""
	if in.RevisionNo > 1 {
		rev = fmt.Sprintf(" · Rev. %d", in.RevisionNo)
	}
	p.bigHeader("GÜNLÜK SAHA RAPORU", dateStr, 22,
		TRDay(in.Date)+rev, fmt.Sprintf("%s · %s", in.ProjectName, in.ProjectCode))

	// ── Künye şeridi ────────────────────────────────────────────────────────
	author := in.Author
	if in.AuthorTitle != "" {
		author += " — " + in.AuthorTitle
	}
	sent := "—"
	if in.SubmittedAt != nil {
		sent = in.SubmittedAt.Format("02.01.2006 · 15:04")
	}
	cell := cW / 3
	for i, kv := range [][2]string{{"Hazırlayan", author}, {"Gönderim", sent}} {
		x := mL + float64(i)*cell
		d.Text(x, p.y+7, k.TrUpper(kv[0]), true, 5.7, k.Dim)
		d.Text(x, p.y+17, d.Fit(kv[1], true, 7.6, cell-8), true, 7.6, k.Ink)
	}
	d.Text(mL+2*cell, p.y+7, "DURUM", true, 5.7, k.Dim)
	cx := p.chipStatus(mL+2*cell, p.y+17, in.Status)
	if in.IssueFlagged {
		d.Chip(cx+4, p.y+17, "Sorun işaretli", k.WarnFg, k.Hex(0xfdeed6))
	}
	p.y += 25
	d.HLine(mL, k.PgW-mR, p.y, k.Rule, 0.6)
	p.y += 9

	// ── Hava + KPI ──────────────────────────────────────────────────────────
	totalMan := 0
	for _, m := range in.Manpower {
		totalMan += m.Headcount
	}
	equipN, equipH := 0, 0.0
	for _, e := range in.Equipment {
		equipN += e.Count
		if e.Hours != nil {
			equipH += *e.Hours
		}
	}
	delayH := 0.0
	for _, dl := range in.Delays {
		delayH += dl.Hours
	}
	const kh = 49.0
	wcW := 147.0
	p.cardBox(mL, p.y, wcW, kh)
	drawWeather(d, mL+22, p.y+kh/2, in.WeatherCond)
	cond := in.WeatherCond
	if cond == "" {
		cond = "Hava bilgisi yok"
	}
	d.Text(mL+44, p.y+17, d.Fit(cond, true, 7.6, wcW-50), true, 7.6, k.Ink)
	d.Text(mL+44, p.y+33, tempText(in.TempMin, in.TempMax), true, 13, k.Navy)
	if in.TempMin != nil && in.TempMax != nil {
		d.Text(mL+44, p.y+43, "en düşük / en yüksek", false, 5.9, k.Dim)
	}
	p.kpiStripAtY(mL+wcW+7, cW-wcW-7, []kpi{
		{Label: "Personel", Value: fmt.Sprint(totalMan), Sub: "adam / gün"},
		{Label: "Ekipman", Value: fmt.Sprint(equipN), Sub: fmt.Sprintf("adet · %s saat", Qty(equipH))},
		{Label: "İmalat", Value: fmt.Sprint(len(in.Work)), Sub: "kalem işlendi"},
		{Label: "Gecikme", Value: Num(delayH, 1), Sub: fmt.Sprintf("saat · %d neden", len(in.Delays)), Accent: true},
	}, kh)
	p.y += kh + 12

	// ── İmalat ──────────────────────────────────────────────────────────────
	p.section("Bugünkü imalat", "Poz no, iş kalemi listesinden gelir")
	if len(in.Work) == 0 {
		p.emptyNote("Bu gün için imalat kaydı girilmemiş.")
	} else {
		rows := make([][]string, 0, len(in.Work))
		for _, w := range in.Work {
			q := ""
			if w.Qty != nil {
				q = Qty(*w.Qty)
			}
			poz := w.Poz
			if poz == "" {
				poz = "—"
			}
			loc := w.Location
			if loc == "" {
				loc = "—"
			}
			rows = append(rows, []string{poz, loc, w.Description, q, w.Unit, w.Discipline})
		}
		p.table(fillCols([]col{{"Poz", 60, false}, {"Konum", 84, false}, {"Yapılan iş", -1, false},
			{"Miktar", 44, true}, {"Birim", 32, false}, {"Disiplin", 70, false}}, cW), rows, false)
	}
	p.y += 10

	// ── Personel / Ekipman ──────────────────────────────────────────────────
	half := (cW - 14) / 2
	manRows := make([][]string, 0, len(in.Manpower)+1)
	for _, m := range in.Manpower {
		manRows = append(manRows, []string{m.Trade, m.Company, fmt.Sprint(m.Headcount)})
	}
	if len(in.Manpower) > 0 {
		manRows = append(manRows, []string{"Toplam", "", fmt.Sprint(totalMan)})
	}
	eqRows := make([][]string, 0, len(in.Equipment)+1)
	for _, e := range in.Equipment {
		name := e.Name
		if e.IdleReason != "" {
			name += " (boşta: " + e.IdleReason + ")"
		}
		h := "—"
		if e.Hours != nil {
			h = Qty(*e.Hours)
		}
		eqRows = append(eqRows, []string{name, fmt.Sprint(e.Count), h})
	}
	if len(in.Equipment) > 0 {
		eqRows = append(eqRows, []string{"Toplam", fmt.Sprint(equipN), Qty(equipH)})
	}
	nRows := len(manRows)
	if len(eqRows) > nRows {
		nRows = len(eqRows)
	}
	p.need(24 + rowH*float64(nRows+1))
	y0 := p.y
	p.d.Text(mL, p.y+7, "PERSONEL DAĞILIMI", true, 7.2, k.Navy)
	p.d.Text(mL+half+14, p.y+7, "EKİPMAN", true, 7.2, k.Navy)
	p.y += 12
	yBody := p.y
	if len(manRows) == 0 {
		p.emptyNote("Personel girilmemiş.")
	} else {
		p.tableAt(mL, fillCols([]col{{"Branş", 82, false}, {"Firma", -1, false}, {"Kişi", 34, true}}, half), manRows, false)
	}
	yMan := p.y
	p.y = yBody
	if len(eqRows) == 0 {
		p.d.Text(mL+half+18, p.y+9, "Ekipman girilmemiş.", false, bodySize, k.Dim)
		p.y += 14
	} else {
		p.tableAt(mL+half+14, fillCols([]col{{"Makine", -1, false}, {"Adet", 32, true}, {"Saat", 36, true}}, half), eqRows, false)
	}
	if yMan > p.y {
		p.y = yMan
	}
	_ = y0
	p.y += 10

	// ── Gecikme / Kadro ─────────────────────────────────────────────────────
	need := 22 + rowH*float64(maxInt(len(in.Delays)*2, len(in.Staff))+1)
	p.need(need)
	p.d.Text(mL, p.y+7, "GECİKME NEDENLERİ", true, 7.2, k.Navy)
	p.d.Text(mL+half+14, p.y+7, "KADRO DURUMU", true, 7.2, k.Navy)
	p.y += 12
	yb := p.y
	yy := yb
	if len(in.Delays) == 0 {
		d.Text(mL+4, yy+9, "Bu gün gecikme kaydı yok.", false, bodySize, k.Dim)
		yy += 14
	}
	for _, dl := range in.Delays {
		d.Text(mL+2, yy+8, dl.Label, false, 7.6, k.Ink)
		d.TextR(mL+half, yy+8, Num(dl.Hours, 1)+" sa", true, 7.6, k.Ink)
		yy += 11
		frac := 0.0
		if delayH > 0 {
			frac = dl.Hours / delayH
		}
		p.bar(mL+2, yy, half-2, 4.4, frac, k.Mid)
		yy += 7
		if dl.Note != "" {
			d.Text(mL+2, yy+6, d.Fit(dl.Note, false, 6.4, half-4), false, 6.4, k.Dim)
			yy += 9
		}
		yy += 4
	}
	sy := yb
	if len(in.Staff) == 0 {
		d.Text(mL+half+18, sy+9, "Kadro durumu girilmemiş.", false, bodySize, k.Dim)
		sy += 14
	}
	for _, s := range in.Staff {
		who := s.Name
		if s.Title != "" {
			who += " · " + s.Title
		}
		d.Text(mL+half+16, sy+9.4, d.Fit(who, false, 7.4, half-70), false, 7.4, k.Ink)
		fg, bg := k.Dim, k.Hex(0xe8edf3)
		if strings.EqualFold(s.Status, "Sahada") {
			fg, bg = k.OkFg, k.OkBg
		}
		d.Chip(mL+2*half+14-d.Width(k.TrUpper(s.Status), true, 6.4)-9, sy+9.4, s.Status, fg, bg)
		d.HLine(mL+half+14, k.PgW-mR, sy+rowH, k.Rule, 0.5)
		sy += rowH
	}
	if sy > yy {
		yy = sy
	}
	p.y = yy + 8

	// ── Notlar / Kasa ───────────────────────────────────────────────────────
	if strings.TrimSpace(in.Notes) != "" || len(in.Cash) > 0 {
		lw := cW * 0.58
		rw := cW - lw - 14
		p.need(60)
		d.Text(mL, p.y+7, "NOTLAR VE SORUNLAR", true, 7.2, k.Navy)
		d.Text(mL+lw+14, p.y+7, "ŞANTİYE KASA HARCAMASI", true, 7.2, k.Navy)
		p.y += 12
		ny := p.y
		if strings.TrimSpace(in.Notes) != "" {
			head := ""
			if in.IssueFlagged {
				head = "Sorun işaretli"
			}
			ny += p.noteBox(mL, ny, lw, head, in.Notes) + 4
		} else {
			d.Text(mL+4, ny+9, "Not girilmemiş.", false, bodySize, k.Dim)
			ny += 14
		}
		cy := p.y
		total := 0.0
		for _, c := range in.Cash {
			total += c.Amount
			desc := c.Description
			if c.Receipt != "" {
				desc += " · Fiş " + c.Receipt
			}
			d.Text(mL+lw+16, cy+9.4, d.Fit(desc, false, 7.4, rw-70), false, 7.4, k.Ink)
			d.TextR(k.PgW-mR-2, cy+9.4, Num(c.Amount, 2)+" TL", false, 7.4, k.Ink)
			d.HLine(mL+lw+14, k.PgW-mR, cy+rowH, k.Rule, 0.5)
			cy += rowH
		}
		if len(in.Cash) > 0 {
			d.Text(mL+lw+16, cy+9.4, "Toplam", true, 7.4, k.Ink)
			d.TextR(k.PgW-mR-2, cy+9.4, Num(total, 2)+" TL", true, 7.4, k.Navy)
			cy += rowH
		} else {
			d.Text(mL+lw+16, cy+9, "Harcama girilmemiş.", false, bodySize, k.Dim)
			cy += 14
		}
		if cy > ny {
			ny = cy
		}
		p.y = ny + 8
	}

	// ── İmza şeridi ─────────────────────────────────────────────────────────
	p.need(40)
	sigY := bottom - 30
	if p.y+8 > sigY {
		sigY = p.y + 8
	}
	w3 := (cW - 24) / 3
	for i, kv := range [][2]string{
		{"Hazırlayan", author}, {"Kontrol", "Müşavir"}, {"Onay", "Proje Müdürü"},
	} {
		x := mL + float64(i)*(w3+12)
		d.HLine(x, x+w3, sigY, k.Navy, 1.1)
		d.Text(x, sigY+8, k.TrUpper(kv[0]), true, 5.7, k.Dim)
		d.Text(x, sigY+18, d.Fit(kv[1], true, 7.4, w3), true, 7.4, k.Ink)
	}
	p.y = sigY + 24

	// ── Fotoğraf eki ────────────────────────────────────────────────────────
	if len(in.Photos) == 0 {
		p.need(24)
		p.section("Fotoğraf ekleri", "")
		p.emptyNote("Bu gün için fotoğraf yüklenmemiş.")
	} else {
		p.slimHeader()
		p.section("Fotoğraf ekleri", fmt.Sprintf("%d fotoğraf", len(in.Photos)))
		rest := in.Photos
		if in.Photos[0].Featured {
			p.photoFeatured(in.Photos[0], 205)
			rest = in.Photos[1:]
		}
		p.photoGrid(rest, 3, 106)
	}
	return p.finish(in.GeneratedAt)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tempText(min, max *float64) string {
	switch {
	case min != nil && max != nil:
		return fmt.Sprintf("%s° / %s°C", Qty(*min), Qty(*max))
	case max != nil:
		return Qty(*max) + "°C"
	case min != nil:
		return Qty(*min) + "°C"
	}
	return "—"
}

// chipStatus — rapor durumu etiketi; sağ kenarı döner.
func (p *page) chipStatus(x, y float64, status string) float64 {
	if strings.EqualFold(status, "Submitted") {
		return p.d.Chip(x, y, "Gönderildi", k.OkFg, k.OkBg)
	}
	return p.d.Chip(x, y, "Taslak", k.Dim, k.Hex(0xe8edf3))
}

// kpiStripAtY — hava kartının yanındaki KPI şeridi (y'yi ilerletmez).
func (p *page) kpiStripAtY(x, w float64, cells []kpi, h float64) {
	p.kpiStripAt(x, w, cells, h)
}

// drawWeather — hava durumu metnine göre küçük simge (güneş / bulut / yağmur).
func drawWeather(d *k.Doc, cx, cy float64, cond string) {
	c := strings.ToLower(cond)
	has := func(ss ...string) bool {
		for _, s := range ss {
			if strings.Contains(c, s) {
				return true
			}
		}
		return false
	}
	sun := has("açık", "acik", "güneş", "gunes", "sunny", "clear")
	rain := has("yağ", "yag", "sağanak", "rain", "kar", "snow")
	if sun && !has("bulut") {
		d.FillCircle(cx, cy, 7, k.Amber)
		for a := 0; a < 8; a++ {
			rad := float64(a) * 3.14159 / 4
			x1, y1 := cx+10*cosf(rad), cy+10*sinf(rad)
			x2, y2 := cx+13.5*cosf(rad), cy+13.5*sinf(rad)
			d.Polyline([][2]float64{{x1, y1}, {x2, y2}}, k.Amber, 1.2)
		}
		return
	}
	if has("parçalı", "parcali", "partly") {
		d.FillCircle(cx-5, cy-5, 5.5, k.Amber)
	}
	d.FillCircle(cx-4, cy+2, 6, k.Soft)
	d.FillCircle(cx+3, cy-1, 7.5, k.Soft)
	d.FillCircle(cx+9, cy+3, 5, k.Soft)
	d.FillRect(cx-4, cy+2, 13, 6, k.Soft)
	if rain {
		for i := 0; i < 3; i++ {
			x := cx - 3 + float64(i)*6
			d.Polyline([][2]float64{{x, cy + 11}, {x - 1.6, cy + 15}}, k.Mid, 1.2)
		}
	}
}
