package reportpdf

import (
	"fmt"
	"sort"
	"strings"
	"time"

	k "github.com/ipks/ipks/backend/internal/pdfkit"
)

// Dönem raporu (aylık / çeyrek / özel) satırları.
type (
	CurvePt struct {
		Month      string // YYYY-MM
		PV, EV, AC float64
	}
	PayRow struct {
		Sub             string
		PeriodNo        int
		Gross, Ded, Net float64
	}
	DedRow struct {
		Label  string
		Amount float64
	}
	MsRow struct {
		Name            string
		Planned, Actual time.Time
		Status          string
		Late            bool
	}
	AttnRow  struct{ Level, Title, Detail string }
	MonthRow struct {
		Label          string // "Temmuz"
		Delta          float64
		SPI, CPI       float64
		Net            float64
		ManDays        int
		Delay          float64
		Opened, Closed int
		Events         []string
	}
	RiskRow struct {
		No, Prob, Impact int
		Title, Status    string
	}
	DecisionRow struct {
		Date time.Time
		Text string
	}
	NextRow struct {
		Label                    string
		ProgressFrom, ProgressTo float64
		Milestones               []MsRow
	}
	PhotoGroup struct {
		Label  string
		Photos []Photo
	}
)

// PeriodData — aylık, çeyrek yıl veya özel dönem yönetim raporunun PDF girdisi.
type PeriodData struct {
	Kind                     string // Monthly | Quarterly | Custom
	ProjectName, ProjectCode string
	Currency                 string
	Title                    string // "Ekim 2026" / "2026 Q3" / "01.03.2026 – 15.05.2026"
	Start, End               time.Time
	BAC, PV, EV, AC          float64
	SPI, CPI, EAC, ETC       float64
	ProgressPct              float64
	BudgetVariance           float64
	PlanSource               string
	Curve                    []CurvePt
	AsOfMonth                string
	Payments                 []PayRow
	MonthGross, MonthDed     float64
	MonthNet                 float64
	Deductions               []DedRow
	Milestones               []MsRow
	OHSOpened, OHSClosed     int
	OHSOpen, OHSCritical     int
	PenaltyCount             int
	PenaltyTotal             float64
	POsOrdered, POsDelivered int
	POsOverdue, PRsPending   int
	Delays                   []DailyDelayRow
	CPIMin, SPIMin           float64
	Attention                []AttnRow
	Narrative                []string
	Months                   []MonthRow
	Risks                    []RiskRow
	Decisions                []DecisionRow
	Next                     *NextRow
	Photos                   []PhotoGroup // tek grup etiketsiz olabilir
	GeneratedAt              time.Time
}

func planFrac(v, base float64) float64 {
	if base <= 0 {
		return 0
	}
	return v / base * 100
}

func idxS(v float64) string {
	if v <= 0 {
		return "—"
	}
	return Num(v, 2)
}

// BuildPeriod — dönem türüne göre düzen seçer: aylık/özel dönem "Yönetici Özeti",
// çeyrek "Değerlendirme Raporu".
func BuildPeriod(in PeriodData) []byte {
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now()
	}
	if in.Kind == "Quarterly" {
		return buildQuarterly(in)
	}
	return buildMonthly(in)
}

// ------------------------------------------------------------------ grafikler

// curveChart — S-eğrisi: PV (kesikli), EV (koyu), AC (amber). pts YYYY-MM ekseni;
// hi != "" ise o aydan sona kadar sarı vurgu bandı çizilir.
func (p *page) curveChart(x, y, w, h float64, pts []CurvePt, bac float64, hiFrom string) {
	d := p.d
	if len(pts) < 2 || bac <= 0 {
		d.Text(x+8, y+h/2, "S-eğrisi için plan ve bütçe verisi yok.", false, 7, k.Dim)
		return
	}
	left, right, top, bot := x+22, x+w-6, y+6, y+h-18
	maxV := 0.0
	for _, pt := range pts {
		for _, v := range []float64{pt.PV, pt.EV, pt.AC} {
			if v/bac*100 > maxV {
				maxV = v / bac * 100
			}
		}
	}
	step := 10.0
	if maxV > 60 {
		step = 20
	}
	maxScale := float64(int(maxV/step)+1) * step
	if maxScale < step {
		maxScale = step
	}
	xAt := func(i int) float64 { return left + (right-left)*float64(i)/float64(len(pts)-1) }
	yAt := func(pctV float64) float64 { return bot - (bot-top)*pctV/maxScale }
	if hiFrom != "" {
		for i, pt := range pts {
			if pt.Month == hiFrom {
				d.FillRect(xAt(i), top, right-xAt(i), bot-top, k.Hex(0xf7eecb))
				break
			}
		}
	}
	for g := 0.0; g <= maxScale; g += step {
		d.HLine(left, right, yAt(g), k.Rule, 0.5)
		d.TextR(left-3, yAt(g)+2, fmt.Sprintf("%.0f", g), false, 5.6, k.Dim)
	}
	every := 1
	if len(pts) > 9 {
		every = (len(pts) + 8) / 9
	}
	for i, pt := range pts {
		if i%every == 0 || i == len(pts)-1 {
			d.TextC(xAt(i), bot+9, monthShort(pt.Month), i == len(pts)-1, 5.8, k.Dim)
		}
	}
	line := func(get func(CurvePt) float64, c k.RGB, lw float64, dash ...float64) {
		var xy [][2]float64
		for i, pt := range pts {
			xy = append(xy, [2]float64{xAt(i), yAt(planFrac(get(pt), bac))})
		}
		d.Polyline(xy, c, lw, dash...)
		last := xy[len(xy)-1]
		d.FillCircle(last[0], last[1], 2, c)
	}
	line(func(c CurvePt) float64 { return c.PV }, k.Soft, 1.4, 3.5, 2.2)
	line(func(c CurvePt) float64 { return c.AC }, k.Amber, 1.4)
	line(func(c CurvePt) float64 { return c.EV }, k.Navy, 1.8)
}

func monthShort(ym string) string {
	t, err := time.Parse("2006-01", ym)
	if err != nil {
		return ym
	}
	return TRMonth(int(t.Month()))[:3]
}

// curveLegend — S-eğrisi altı açıklama.
func (p *page) curveLegend(x, y float64, extra string) {
	d := p.d
	items := []struct {
		l string
		c k.RGB
		w float64
		s bool
	}{{"PV planlanan", k.Soft, 1.4, true}, {"EV kazanılan", k.Navy, 1.8, false}, {"AC gerçekleşen", k.Amber, 1.4, false}}
	for _, it := range items {
		if it.s {
			d.Polyline([][2]float64{{x, y - 2.5}, {x + 12, y - 2.5}}, it.c, it.w, 3.5, 2.2)
		} else {
			d.Polyline([][2]float64{{x, y - 2.5}, {x + 12, y - 2.5}}, it.c, it.w)
		}
		d.Text(x+15, y, it.l, false, 5.9, k.Dim)
		x += 18 + d.Width(it.l, false, 5.9) + 8
	}
	if extra != "" {
		d.FillRect(x, y-6, 9, 7, k.Hex(0xf7eecb))
		d.Text(x+12, y, extra, false, 5.9, k.Dim)
	}
}

// donut — net hakediş dağılımı halkası + açıklama listesi.
func (p *page) donut(x, y float64, rows []PayRow, centerTop, centerVal string) {
	d := p.d
	total := 0.0
	byCo := map[string]float64{}
	var order []string
	for _, r := range rows {
		if _, ok := byCo[r.Sub]; !ok {
			order = append(order, r.Sub)
		}
		byCo[r.Sub] += r.Net
		total += r.Net
	}
	cols := []k.RGB{k.Navy, k.Amber, k.Steel, k.Mid, k.Soft, k.Hex(0x3a3a8a)}
	cx, cy := x+34, y+34
	if total <= 0 {
		d.RingSeg(cx, cy, 30, 20, 0, 359.9, k.Track)
		d.Text(x+80, y+30, "Dönemde kesinleşen hakediş yok.", false, 7, k.Dim)
		return
	}
	a := 0.0
	for i, co := range order {
		sweep := byCo[co] / total * 360
		d.RingSeg(cx, cy, 30, 20, a, a+sweep-0.4, cols[i%len(cols)])
		a += sweep
	}
	d.TextC(cx, cy-1, centerTop, false, 5.6, k.Dim)
	d.TextC(cx, cy+8, centerVal, true, 8.4, k.Navy)
	ly := y + 8
	for i, co := range order {
		if i >= 5 {
			break
		}
		d.FillRect(x+78, ly-5.5, 6.5, 6.5, cols[i%len(cols)])
		d.Text(x+88, ly, d.Fit(co, false, 7, 100), false, 7, k.Ink)
		d.TextR(k.PgW-mR, ly, fmt.Sprintf("%%%d", int(byCo[co]/total*100+0.5)), true, 7, k.Ink)
		ly += 11.5
	}
}

// ------------------------------------------------------------ aylık / özel dönem

func buildMonthly(in PeriodData) []byte {
	kindTitle := "AYLIK YÖNETİM RAPORU"
	if in.Kind == "Custom" {
		kindTitle = "ÖZEL DÖNEM YÖNETİM RAPORU"
	}
	footer := fmt.Sprintf("ConCoord · %s · %s · %s", periodWord(in.Kind), in.Title, in.ProjectCode)
	p := newPage(kindTitle, footer, fmt.Sprintf("%s · %s", in.Title, in.ProjectCode))
	d := p.d
	cur := in.Currency
	if cur == "" {
		cur = "TRY"
	}
	p.bigHeader(kindTitle, in.Title, 24, DMY(in.Start)+" – "+DMY(in.End),
		fmt.Sprintf("%s · %s · Para birimi: %s", in.ProjectName, in.ProjectCode, cur))

	// ── KPI ─────────────────────────────────────────────────────────────────
	planPct := planFrac(in.PV, in.BAC)
	spiWarn := in.SPI > 0 && in.SPI < 1
	cpiWarn := in.CPI > 0 && in.CPI < 1
	bv := Mil(in.BudgetVariance)
	if in.BudgetVariance > 0 {
		bv = "+" + bv
	}
	bvSub := "bütçe içinde"
	if in.BudgetVariance < 0 {
		bvSub = "bütçe aşımı"
	} else if in.BudgetVariance > 0 {
		bvSub = "lehte"
	}
	p.kpiStrip([]kpi{
		{Label: "Fiziki ilerleme", Value: "%" + Num(in.ProgressPct, 1), Sub: "plan %" + Num(planPct, 1)},
		{Label: "SPI", Value: idxS(in.SPI), Sub: "EV / PV", Warn: spiWarn},
		{Label: "CPI", Value: idxS(in.CPI), Sub: "EV / AC", Warn: cpiWarn},
		{Label: "EAC", Value: Mil(in.EAC), Sub: "BAC " + Mil(in.BAC)},
		{Label: "Bütçe sapması", Value: bv, Sub: bvSub},
		{Label: "Kesinleşen hakediş", Value: fmt.Sprint(len(in.Payments)), Sub: "net " + Mil(in.MonthNet), Accent: true},
	}, 44)

	// ── S-eğrisi + EVM ──────────────────────────────────────────────────────
	chartW := cW*0.6 - 4
	rw := cW - chartW - 14
	const ch = 124.0
	p.need(ch + 30)
	d.Text(mL, p.y+7, "S-EĞRİSİ — KÜMÜLATİF İLERLEME (%)", true, 7.2, k.Navy)
	d.Text(mL+chartW+14, p.y+7, "EVM ÖZETİ (DÖNEM SONU)", true, 7.2, k.Navy)
	p.y += 12
	p.cardBox(mL, p.y, chartW, ch)
	p.curveChart(mL+4, p.y+2, chartW-8, ch-16, curveUpTo(in.Curve, in.AsOfMonth), in.BAC, "")
	p.curveLegend(mL+10, p.y+ch-5, "")
	yTop := p.y
	p.y = yTop
	ev := [][]string{
		{"BAC · proje bütçesi", Num(in.BAC, 0)},
		{"PV · planlanan değer", Num(in.PV, 0)},
		{"EV · kazanılan değer", Num(in.EV, 0)},
		{"AC · gerçekleşen maliyet", Num(in.AC, 0)},
		{"EAC · tamamlanma tahmini", Num(in.EAC, 0)},
		{"ETC · kalan maliyet", Num(in.ETC, 0)},
		{"PV kaynağı", planSourceTR(in.PlanSource)},
	}
	for i, r := range ev {
		d.Text(mL+chartW+14+2, p.y+9.4, r[0], false, 7.2, k.Ink)
		bold := i != len(ev)-1
		d.TextR(k.PgW-mR-2, p.y+9.4, r[1], bold, 7.2, k.Ink)
		d.HLine(mL+chartW+14, k.PgW-mR, p.y+rowH, k.Rule, 0.5)
		p.y += rowH
	}
	p.y += 4
	if in.SPI > 0 && in.SPI < 1 {
		p.y += p.noteBox(mL+chartW+14, p.y, rw, fmt.Sprintf("SPI %s", Num(in.SPI, 2)), spiNote(in)) + 2
	}
	p.y = yTop + ch + 12

	// ── Hakedişler ──────────────────────────────────────────────────────────
	note := ""
	if len(in.Deductions) > 0 {
		var parts []string
		for _, dr := range in.Deductions {
			parts = append(parts, fmt.Sprintf("%s %s", strings.ToLower(dr.Label), Num(dr.Amount, 0)))
		}
		note = "Kesinti dökümü: " + strings.Join(parts, " · ")
	}
	p.section("Dönemde kesinleşen hakedişler", note)
	if len(in.Payments) == 0 {
		p.emptyNote("Bu dönemde kesinleşen hakediş yok.")
	} else {
		rows := make([][]string, 0, len(in.Payments)+1)
		for _, r := range in.Payments {
			rows = append(rows, []string{r.Sub, fmt.Sprintf("#%d", r.PeriodNo), Num(r.Gross, 2), Num(r.Ded, 2), Num(r.Net, 2)})
		}
		rows = append(rows, []string{"Toplam", "", Num(in.MonthGross, 2), Num(in.MonthDed, 2), Num(in.MonthNet, 2)})
		p.table(fillCols([]col{{"Taşeron", -1, false}, {"Dönem", 40, false}, {"Brüt (bu dönem)", 92, true}, {"Kesintiler", 82, true}, {"Net ödenecek", 92, true}}, cW), rows, false)
	}
	p.y += 10

	// ── Dikkat gerektirenler + hakediş dağılımı ─────────────────────────────
	half := (cW - 16) / 2
	lw := cW*0.56 - 8
	p.need(110)
	d.Text(mL, p.y+7, "DİKKAT GEREKTİRENLER", true, 7.2, k.Navy)
	d.Text(mL+lw+16, p.y+7, "KESİNLEŞEN HAKEDİŞ DAĞILIMI (NET)", true, 7.2, k.Navy)
	p.y += 12
	yb := p.y
	if len(in.Attention) == 0 {
		d.Text(mL+4, p.y+9, "Eşikleri aşan bir gösterge yok.", false, bodySize, k.Dim)
		p.y += 14
	}
	for i, a := range in.Attention {
		fg, bg, bd := k.Dim, k.White, k.Rule
		if a.Level == "warn" || a.Level == "bad" {
			fg, bg, bd = k.WarnFg, k.WarnBg, k.Hex(0xe8c88f)
		}
		det := d.Wrap(a.Detail, false, 6.6, lw-34)
		h := 15.0 + float64(len(det))*8.2
		d.FillRect(mL, p.y, lw, h, bg)
		d.StrokeRect(mL, p.y, lw, h, bd, 0.6)
		d.Text(mL+8, p.y+h/2+4, fmt.Sprint(i+1), true, 11, fg)
		d.Text(mL+24, p.y+10, d.Fit(a.Title, true, 7.4, lw-30), true, 7.4, k.Ink)
		for li, ln := range det {
			d.Text(mL+24, p.y+19+float64(li)*8.2, ln, false, 6.6, k.Dim)
		}
		p.y += h + 4
	}
	yl := p.y
	p.y = yb
	p.donut(mL+lw+16, p.y, in.Payments, "NET", Mil(in.MonthNet))
	d.Text(mL+lw+16, p.y+78, fmt.Sprintf("Brüt %s · kesinti %s", Mil(in.MonthGross), Mil(in.MonthDed)), false, 6.4, k.Dim)
	if yb+88 > yl {
		yl = yb + 88
	}
	p.y = yl + 10
	_ = half

	// ── Kilometre taşları | İSG | Tedarik ──────────────────────────────────
	c1 := cW*0.44 - 8
	c2 := cW*0.27 - 8
	c3 := cW - c1 - c2 - 32
	p.need(116)
	x2, x3 := mL+c1+16, mL+c1+c2+32
	d.Text(mL, p.y+7, "KİLOMETRE TAŞLARI", true, 7.2, k.Navy)
	d.Text(x2, p.y+7, "İSG PERFORMANSI", true, 7.2, k.Navy)
	d.Text(x3, p.y+7, "TEDARİK", true, 7.2, k.Navy)
	p.y += 12
	yb = p.y
	if len(in.Milestones) == 0 {
		d.Text(mL+4, p.y+9, "Tanımlı kilometre taşı yok.", false, bodySize, k.Dim)
		p.y += 14
	}
	for i, m := range in.Milestones {
		if i >= 6 {
			break
		}
		d.Text(mL+2, p.y+9.4, d.Fit(m.Name, false, 7.2, c1-120), false, 7.2, k.Ink)
		ds := "—"
		if !m.Planned.IsZero() {
			ds = DMY(m.Planned)
		}
		d.TextR(mL+c1-62, p.y+9.4, ds, false, 6.6, k.Dim)
		lab, fg, bg := msStatus(m)
		d.Chip(mL+c1-58, p.y+9.4, lab, fg, bg)
		d.HLine(mL, mL+c1, p.y+rowH, k.Rule, 0.5)
		p.y += rowH
	}
	yl = p.y
	p.y = yb
	mini := func(x, y, w float64, lab, val string, fg, bg k.RGB) {
		d.FillRect(x, y, w, 28, bg)
		d.StrokeRect(x, y, w, 28, k.Rule, 0.5)
		d.Text(x+6, y+10, k.TrUpper(lab), true, 5.4, k.Dim)
		d.Text(x+6, y+23, val, true, 12, fg)
	}
	mw := (c2 - 6) / 2
	mini(x2, p.y, mw, "Açılan bulgu", fmt.Sprint(in.OHSOpened), k.Navy, k.Wash)
	mini(x2+mw+6, p.y, mw, "Kapatılan", fmt.Sprint(in.OHSClosed), k.OkFg, k.Wash)
	mini(x2, p.y+34, mw, "Açık toplam", fmt.Sprint(in.OHSOpen), k.Navy, k.Wash)
	critBg := k.Wash
	critFg := k.Navy
	if in.OHSCritical > 0 {
		critBg, critFg = k.Hex(0xfdeed6), k.WarnFg
	}
	mini(x2+mw+6, p.y+34, mw, "Açık kritik", fmt.Sprint(in.OHSCritical), critFg, critBg)
	d.Text(x2, p.y+75, fmt.Sprintf("Ceza: %d adet · %s TL", in.PenaltyCount, Num(in.PenaltyTotal, 0)), false, 6.4, k.Dim)
	ty := yb
	for _, r := range [][2]string{
		{"Açılan sipariş (PO)", fmt.Sprint(in.POsOrdered)}, {"Teslim alınan PO", fmt.Sprint(in.POsDelivered)},
		{"Geciken PO", fmt.Sprint(in.POsOverdue)}, {"Onay bekleyen PR", fmt.Sprint(in.PRsPending)},
	} {
		d.Text(x3+2, ty+9.4, r[0], false, 7.2, k.Ink)
		fg := k.Ink
		if r[0] == "Geciken PO" && in.POsOverdue > 0 {
			fg = k.WarnFg
		}
		d.TextR(x3+c3-2, ty+9.4, r[1], true, 7.2, fg)
		d.HLine(x3, x3+c3, ty+rowH, k.Rule, 0.5)
		ty += rowH
	}
	if yb+84 > yl {
		yl = yb + 84
	}
	if ty > yl {
		yl = ty
	}
	p.y = yl + 10

	// ── Gecikme nedenleri ───────────────────────────────────────────────────
	total := 0.0
	for _, dl := range in.Delays {
		total += dl.Hours
	}
	p.section("Gecikme nedenleri (günlük raporlardan)", fmt.Sprintf("toplam %s saat", Num(total, 1)))
	if len(in.Delays) == 0 {
		p.emptyNote("Dönemde gecikme kaydı yok.")
	} else {
		bw := (cW - 22) / 2
		maxH := in.Delays[0].Hours
		for i, dl := range in.Delays {
			if i >= 6 {
				break
			}
			x := mL + float64(i%2)*(bw+22)
			if i%2 == 0 {
				p.need(24)
			}
			y := p.y
			d.Text(x+1, y+8, dl.Label, false, 7.2, k.Ink)
			d.TextR(x+bw, y+8, Num(dl.Hours, 1)+" sa", true, 7.2, k.Ink)
			frac := 0.0
			if maxH > 0 {
				frac = dl.Hours / maxH
			}
			c := k.Mid
			if i == 2 {
				c = k.Amber
			}
			p.bar(x, y+11, bw, 4.4, frac, c)
			if i%2 == 1 || i == len(in.Delays)-1 || i == 5 {
				p.y += 23
			}
		}
	}

	// ── Fotoğraflar ─────────────────────────────────────────────────────────
	p.periodPhotos(in, "Dönem fotoğrafları", 4, 80, false)
	return p.finish(in.GeneratedAt)
}

// periodPhotos — grup başlıklı (ay etiketli) fotoğraf sayfası; hiç fotoğraf yoksa
// bölüm başlığı ve boş durum yazılır.
func (p *page) periodPhotos(in PeriodData, title string, cols int, imgH float64, cont bool) {
	n := 0
	for _, g := range in.Photos {
		n += len(g.Photos)
	}
	if n == 0 {
		p.y += 6
		p.need(24)
		p.section(title, "")
		p.emptyNote("Dönemde Fotoğraflar sayfasına yüklenen fotoğraf yok.")
		return
	}
	if !cont {
		p.slimHeader()
	} else {
		p.y += 6
	}
	p.section(title, fmt.Sprintf("%d fotoğraf", n))
	for _, g := range in.Photos {
		if len(g.Photos) == 0 {
			continue
		}
		if g.Label != "" {
			p.need(30 + imgH)
			p.d.FillRect(mL, p.y, cW, 12, k.Navy)
			p.d.Text(mL+6, p.y+8.6, k.TrUpper(g.Label), true, 6.6, k.White)
			p.y += 17
		}
		p.photoGrid(g.Photos, cols, imgH)
	}
}

func periodWord(kind string) string {
	switch kind {
	case "Quarterly":
		return "Çeyrek Yıl Yönetim Raporu"
	case "Custom":
		return "Özel Dönem Yönetim Raporu"
	}
	return "Aylık Yönetim Raporu"
}

func planSourceTR(s string) string {
	switch s {
	case "manual":
		return "manuel dağılım"
	case "milestones":
		return "kilometre taşları"
	case "baseline":
		return "iş programı baseline"
	case "linear":
		return "doğrusal"
	}
	return s
}

func spiNote(in PeriodData) string {
	if in.SPIMin > 0 && in.SPI >= in.SPIMin {
		return fmt.Sprintf("Eşik %s üzerinde; ancak plana göre geride.", Num(in.SPIMin, 2))
	}
	return "Eşiğin altında; telafi planı gerekli."
}

func msStatus(m MsRow) (string, k.RGB, k.RGB) {
	switch {
	case m.Status == "Completed" && m.Late:
		return "Tamam · geç", k.OkFg, k.OkBg
	case m.Status == "Completed":
		return "Tamam", k.OkFg, k.OkBg
	case m.Late:
		return "Geç", k.WarnFg, k.Hex(0xfdeed6)
	}
	return "Planda", k.Dim, k.Hex(0xe8edf3)
}

// curveUpTo — as-of aya kadar noktalar (EV/AC gelecekte anlamsız); boşsa tümü.
func curveUpTo(pts []CurvePt, asOf string) []CurvePt {
	if asOf == "" {
		return pts
	}
	var out []CurvePt
	for _, p := range pts {
		if p.Month <= asOf {
			out = append(out, p)
		}
	}
	return out
}

// sortedDelays — saate göre azalan.
func sortedDelays(in []DailyDelayRow) []DailyDelayRow {
	out := append([]DailyDelayRow(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Hours > out[j].Hours })
	return out
}
