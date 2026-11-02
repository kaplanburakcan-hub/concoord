package reportpdf

import (
	"fmt"
	"strings"
	"time"

	k "github.com/ipks/ipks/backend/internal/pdfkit"
)

// buildQuarterly — çeyrek yıl "Değerlendirme Raporu" düzeni (taslak "Çeyrek B"
// + onaylanan eklemeler: kümülatif S-eğrisi, ay ay SPI/CPI, sonraki çeyrek hedefleri).
func buildQuarterly(in PeriodData) []byte {
	kind := "ÇEYREK YIL DEĞERLENDİRME RAPORU"
	footer := fmt.Sprintf("ConCoord · Çeyrek Yıl Değerlendirme Raporu · %s · %s", in.Title, in.ProjectCode)
	p := newPage(kind, footer, fmt.Sprintf("%s · %s", in.Title, in.ProjectCode))
	d := p.d
	q := fmt.Sprintf("Q%d", (int(in.Start.Month())-1)/3+1)

	// ── Kapak bandı ─────────────────────────────────────────────────────────
	d.NewPage()
	const coverH = 112.0
	d.FillRect(0, 0, k.PgW, coverH, k.Navy)
	d.FillRect(0, coverH, k.PgW, 3, k.Amber)
	brandOn(d, mL, 31, 14)
	d.Text(mL, 39, "İNŞAAT PROJE YÖNETİM PLATFORMU", false, 5.2, k.OnNavy2)
	d.Text(mL, 66, kind, true, 6.8, k.Amber)
	d.Text(mL, 85, d.Fit(in.ProjectName, true, 17, 330), true, 17, k.White)
	d.Text(mL, 99, fmt.Sprintf("%s · %s – %s", in.ProjectCode, DMY(in.Start), DMY(in.End)), false, 7.6, k.OnNavy)
	d.TextR(k.PgW-mR, 78, q, true, 66, k.Amber)
	d.TextR(k.PgW-mR, 98, fmt.Sprintf("%d · %s", in.Start.Year(), quarterMonthsText(in.Start)), false, 7.4, k.OnNavy)
	p.y = coverH + 3 + 11

	// ── Beş sayı ────────────────────────────────────────────────────────────
	totalDelay := 0.0
	for _, dl := range in.Delays {
		totalDelay += dl.Hours
	}
	p.kpiStrip([]kpi{
		{Label: "Kümülatif fiziki", Value: "%" + Num(in.ProgressPct, 1), Sub: "plan %" + Num(planFrac(in.PV, in.BAC), 1)},
		{Label: "SPI", Value: idxS(in.SPI), Sub: "dönem sonu", Warn: in.SPI > 0 && in.SPI < 1},
		{Label: "CPI", Value: idxS(in.CPI), Sub: "dönem sonu", Warn: in.CPI > 0 && in.CPI < 1},
		{Label: "EAC", Value: Mil(in.EAC), Sub: "BAC " + Mil(in.BAC)},
		{Label: "Gecikme", Value: Num(totalDelay, 1) + " sa", Sub: "3 ay toplam", Accent: true},
	}, 44)

	// ── Değerlendirme + bütçe ───────────────────────────────────────────────
	lw := cW*0.6 - 8
	rw := cW - lw - 18
	p.need(80)
	d.Text(mL, p.y+7, "YÖNETİCİ DEĞERLENDİRMESİ", true, 7.2, k.Navy)
	d.Text(mL+lw+18, p.y+7, "BÜTÇE DURUMU", true, 7.2, k.Navy)
	p.y += 12
	yb := p.y
	for _, para := range in.Narrative {
		for _, ln := range d.Wrap(para, false, 7.6, lw) {
			d.Text(mL, p.y+8, ln, false, 7.6, k.Ink)
			p.y += 10.6
		}
		p.y += 4
	}
	yl := p.y
	p.y = yb
	rx := mL + lw + 18
	if in.BAC > 0 {
		acF := in.AC / in.BAC
		p.bar(rx, p.y+4, rw, 11, acF, k.Navy)
		d.Text(rx, p.y+26, fmt.Sprintf("AC %s harcanan", Mil(in.AC)), true, 6.8, k.Navy)
		d.TextR(rx+rw, p.y+26, fmt.Sprintf("kalan %s", Mil(in.BAC-in.AC)), false, 6.8, k.Dim)
		d.Text(rx, p.y+39, fmt.Sprintf("BAC %s · EAC %s", Mil(in.BAC), Mil(in.EAC)), false, 6.6, k.Dim)
		lab := "lehte"
		if in.BudgetVariance < 0 {
			lab = "aleyhte"
		}
		d.Text(rx, p.y+49, fmt.Sprintf("Sapma %s (%s)", Mil(in.BudgetVariance), lab), true, 6.8, k.Ink)
	} else {
		d.Text(rx, p.y+12, "Proje bütçesi (BAC) girilmemiş.", false, 7, k.Dim)
	}
	if p.y+56 > yl {
		yl = p.y + 56
	}
	p.y = yl + 8

	// ── S-eğrisi + ay ay SPI/CPI ────────────────────────────────────────────
	cw := cW*0.6 - 8
	const ch = 126.0
	p.need(ch + 30)
	d.Text(mL, p.y+7, "KÜMÜLATİF S-EĞRİSİ — ÇEYREK VURGULU", true, 7.2, k.Navy)
	d.Text(mL+cw+18, p.y+7, "AY AY SPI VE CPI", true, 7.2, k.Navy)
	p.y += 12
	p.cardBox(mL, p.y, cw, ch)
	pts := curveUpTo(in.Curve, in.AsOfMonth)
	p.curveChart(mL+4, p.y+2, cw-8, ch-16, pts, in.BAC, in.Start.Format("2006-01"))
	p.curveLegend(mL+10, p.y+ch-5, "rapor çeyreği")
	tx := mL + cw + 18
	tw := cW - cw - 18
	chartTop := p.y
	rows := [][]string{}
	if prev := curveAtMonth(in.Curve, in.Start.AddDate(0, -1, 0).Format("2006-01")); prev != nil && prev.PV > 0 {
		rows = append(rows, []string{TRMonth(int(in.Start.AddDate(0, -1, 0).Month())) + " (başl.)", idxS(spiOf(*prev)), idxS(cpiOf(*prev))})
	}
	for _, m := range in.Months {
		rows = append(rows, []string{m.Label, idxS(m.SPI), idxS(m.CPI)})
	}
	yTab := p.y
	if len(rows) > 0 {
		p.tableAt(tx, fillCols([]col{{"Ay", -1, false}, {"SPI", 38, true}, {"CPI", 38, true}}, tw), rows, true)
	}
	// SPI/CPI çubukları
	barY := p.y + 8
	if len(in.Months) > 0 {
		const bh = 40.0
		d.FillRect(tx, barY, tw, bh+14, k.Wash)
		d.StrokeRect(tx, barY, tw, bh+14, k.Rule, 0.5)
		base := barY + bh + 2
		oneY := base - (1.0-0.8)/0.3*bh
		d.Polyline([][2]float64{{tx + 4, oneY}, {tx + tw - 4, oneY}}, k.BadFg, 0.8, 3, 2)
		slot := (tw - 8) / float64(len(in.Months))
		for i, m := range in.Months {
			cx := tx + 4 + slot*float64(i) + slot/2
			for j, v := range []float64{m.SPI, m.CPI} {
				h := (v - 0.8) / 0.3 * bh
				if h < 1 {
					h = 1
				}
				if h > bh {
					h = bh
				}
				c := k.Mid
				if j == 1 {
					c = k.Amber
				}
				d.FillRect(cx-9+float64(j)*10, base-h, 8, h, c)
			}
			d.TextC(cx, base+9, m.Label[:3], true, 5.8, k.Ink)
		}
		d.FillRect(tx+tw-60, barY+3, 5, 5, k.Mid)
		d.Text(tx+tw-53, barY+8, "SPI", false, 5.6, k.Dim)
		d.FillRect(tx+tw-32, barY+3, 5, 5, k.Amber)
		d.Text(tx+tw-25, barY+8, "CPI", false, 5.6, k.Dim)
	}
	_ = yTab
	p.y = chartTop + ch + 12

	// ── Ay ay olaylar ───────────────────────────────────────────────────────
	if len(in.Months) > 0 {
		bw := (cW - 12*float64(len(in.Months)-1)) / float64(len(in.Months))
		maxEv := 1
		for _, m := range in.Months {
			if len(m.Events) > maxEv {
				maxEv = len(m.Events)
			}
		}
		bh := 22 + float64(maxEv)*20
		p.need(bh + 22)
		d.Text(mL, p.y+7, "AY AY OLAYLAR", true, 7.2, k.Navy)
		p.y += 12
		for i, m := range in.Months {
			x := mL + float64(i)*(bw+12)
			d.StrokeRect(x, p.y, bw, bh, k.Rule, 0.6)
			d.FillRect(x, p.y, bw, 14, k.Navy)
			d.Text(x+6, p.y+9.6, k.TrUpper(m.Label), true, 6.8, k.White)
			d.TextR(x+bw-6, p.y+9.6, fmt.Sprintf("%s%%%s", signF(m.Delta), Num(abs(m.Delta), 1)), true, 6.8, k.Amber)
			by := p.y + 24
			if len(m.Events) == 0 {
				d.Text(x+8, by, "Dönemde kayda değer olay yok.", false, 6.6, k.Dim)
			}
			for _, ev := range m.Events {
				d.FillRect(x+8, by-4.2, 3, 3, k.Navy)
				lines := d.Wrap(ev, false, 6.8, bw-24)
				for li, ln := range lines {
					if li >= 2 {
						break
					}
					d.Text(x+15, by+float64(li)*8.4, ln, false, 6.8, k.Ink)
				}
				by += 8.4*float64(minInt(len(lines), 2)) + 3.6
			}
		}
		p.y += bh + 12
	}

	// ── Kilometre taşı çizelgesi + risk matrisi ─────────────────────────────
	p.need(150)
	mw := cW*0.56 - 8
	d.Text(mL, p.y+7, "KİLOMETRE TAŞI ÇİZELGESİ — PLAN / GERÇEK", true, 7.2, k.Navy)
	d.Text(mL+mw+18, p.y+7, "RİSK MATRİSİ", true, 7.2, k.Navy)
	p.y += 12
	yb = p.y
	p.timeline(mL, p.y, mw, in)
	yl = p.y
	p.y = yb
	p.riskMatrix(mL+mw+18, p.y, cW-mw-18, in.Risks)
	p.y = maxF(yl, yb+142) + 6

	// ── Kararlar ────────────────────────────────────────────────────────────
	p.section("Çeyrekte alınan kararlar", "karar günlüğünden")
	if len(in.Decisions) == 0 {
		p.emptyNote("Çeyrekte karar günlüğüne kayıt girilmemiş.")
	} else {
		p.need(rowH * float64(len(in.Decisions)))
		for _, dc := range in.Decisions {
			lines := d.Wrap(dc.Text, false, 7.2, cW-70)
			h := float64(len(lines))*9.2 + 5
			p.need(h + 2)
			d.Text(mL+2, p.y+9, DMY(dc.Date), true, 7, k.Navy)
			for li, ln := range lines {
				d.Text(mL+64, p.y+9+float64(li)*9.2, ln, false, 7.2, k.Ink)
			}
			d.HLine(mL, mL+cW, p.y+h, k.Rule, 0.5)
			p.y += h + 2
		}
	}
	p.y += 8

	// ── Sonraki çeyrek hedefleri (2. sayfanın başı) ────────────────────────
	cont := false
	if in.Next != nil {
		p.slimHeader()
		cont = true
		p.section("Sonraki çeyrek ("+in.Next.Label+") hedefleri", "plan ve kontrol eşiklerinden")
		cardW := (cW - 20) / 3
		card := func(i int, lab, val, sub string) {
			x := mL + float64(i%3)*(cardW+10)
			y := p.y + float64(i/3)*48
			p.cardBox(x, y, cardW, 42)
			d.Text(x+8, y+11, k.TrUpper(lab), true, 5.7, k.Dim)
			d.Text(x+8, y+27, d.Fit(val, true, 12, cardW-14), true, 12, k.Navy)
			if sub != "" {
				d.Text(x+8, y+37, d.Fit(sub, false, 6, cardW-14), false, 6, k.Dim)
			}
		}
		n := 0
		card(n, "Fiziki hedef", fmt.Sprintf("%%%s → %%%s", Num(in.Next.ProgressFrom, 0), Num(in.Next.ProgressTo, 0)),
			fmt.Sprintf("çeyrekte +%s puan (plana göre)", Num(in.Next.ProgressTo-in.Next.ProgressFrom, 0)))
		n++
		if len(in.Next.Milestones) > 0 {
			ms := in.Next.Milestones[0]
			sub := fmt.Sprintf("hedef tarih %s", DMY(ms.Planned))
			if len(in.Next.Milestones) > 1 {
				sub += fmt.Sprintf(" · +%d kilometre taşı daha", len(in.Next.Milestones)-1)
			}
			card(n, "Kilometre taşı", ms.Name, sub)
			n++
		}
		if in.SPI > 0 {
			card(n, "SPI hedefi", fmt.Sprintf("%s → 1,00", Num(in.SPI, 2)), "planla paralel ilerleme")
			n++
		}
		if in.CPI > 0 {
			card(n, "CPI hedefi", "≥ 1,00", fmt.Sprintf("eşik %s · şimdi %s", Num(in.CPIMin, 2), Num(in.CPI, 2)))
			n++
		}
		p.y += float64((n+2)/3)*48 + 6
	}

	// ── Fotoğraflar (ay ay) ─────────────────────────────────────────────────
	p.periodPhotos(in, "Çeyrek fotoğrafları — ay ay", 3, 96, cont)
	return p.finish(in.GeneratedAt)
}

// timeline — kilometre taşları: planlanan (içi boş elmas) ve gerçekleşen (dolu daire).
func (p *page) timeline(x, y, w float64, in PeriodData) {
	d := p.d
	from := in.Start.AddDate(0, -1, 0)
	to := in.End.AddDate(0, 3, 0)
	var list []MsRow
	for _, m := range in.Milestones {
		t := m.Planned
		if t.IsZero() {
			t = m.Actual
		}
		if t.IsZero() || t.Before(from) || t.After(to) {
			continue
		}
		list = append(list, m)
		if len(list) >= 6 {
			break
		}
	}
	labW := 96.0
	ax, aw := x+labW+4, w-labW-8
	span := to.Sub(from).Hours() / 24
	xAt := func(t time.Time) float64 { return ax + aw*(t.Sub(from).Hours()/24)/span }
	// ay çizgileri
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC); m.Before(to); m = m.AddDate(0, 1, 0) {
		if m.Before(from) {
			continue
		}
		d.VLine(xAt(m), y, y+8+float64(maxInt(len(list), 1))*15, k.Rule, 0.4)
		d.TextC(xAt(m)+aw/span*15, y+6, TRMonth(int(m.Month()))[:3], false, 5.4, k.Dim)
	}
	// rapor çeyreği bandı
	d.FillRect(xAt(in.Start), y+8, xAt(in.End.AddDate(0, 0, 1))-xAt(in.Start), float64(maxInt(len(list), 1))*15, k.Hex(0xf7eecb))
	if len(list) == 0 {
		d.Text(x+4, y+22, "Bu aralıkta tanımlı kilometre taşı yok.", false, 7, k.Dim)
		p.y = y + 30
		return
	}
	ry := y + 8
	for _, m := range list {
		d.Text(x+2, ry+10, d.Fit(m.Name, false, 6.8, labW-4), false, 6.8, k.Ink)
		if !m.Planned.IsZero() {
			cx, cy := xAt(m.Planned), ry+7.5
			d.FillPoly([][2]float64{{cx, cy - 3.6}, {cx + 3.6, cy}, {cx, cy + 3.6}, {cx - 3.6, cy}}, k.Soft)
		}
		if !m.Actual.IsZero() && m.Status == "Completed" {
			d.FillCircle(xAt(m.Actual), ry+7.5, 3.2, k.Navy)
			if !m.Planned.IsZero() {
				if days := int(m.Actual.Sub(m.Planned).Hours() / 24); days != 0 {
					d.Text(xAt(m.Actual)+5, ry+10, fmt.Sprintf("%+d g", days), true, 5.8, k.WarnFg)
				}
			}
		} else if m.Late {
			d.FillCircle(xAt(minTime(in.End, to)), ry+7.5, 3.2, k.Amber)
		}
		d.HLine(x, x+w, ry+15, k.Rule, 0.4)
		ry += 15
	}
	ly := ry + 9
	d.FillPoly([][2]float64{{x + 4, ly - 7}, {x + 7, ly - 4}, {x + 4, ly - 1}, {x + 1, ly - 4}}, k.Soft)
	d.Text(x+10, ly-1, "planlanan", false, 5.8, k.Dim)
	d.FillCircle(x+58, ly-3.5, 2.6, k.Navy)
	d.Text(x+63, ly-1, "gerçekleşen", false, 5.8, k.Dim)
	d.FillCircle(x+118, ly-3.5, 2.6, k.Amber)
	d.Text(x+123, ly-1, "geciken", false, 5.8, k.Dim)
	p.y = ly + 4
}

// riskMatrix — 5×5 olasılık × etki ızgarası; riskler numaralı daire, altında liste.
func (p *page) riskMatrix(x, y, w float64, risks []RiskRow) {
	d := p.d
	if len(risks) == 0 {
		d.Text(x+2, y+14, "Açık veya izlenen risk kaydı yok.", false, 7, k.Dim)
		return
	}
	const cell = 17.0
	gx := x + 10
	for pr := 5; pr >= 1; pr-- {
		for im := 1; im <= 5; im++ {
			sc := pr * im
			c := k.Hex(0xe8f1e8)
			if sc >= 15 {
				c = k.Hex(0xf3b8b2)
			} else if sc >= 8 {
				c = k.Hex(0xf9d9d5)
			} else if sc >= 4 {
				c = k.Hex(0xfdeed6)
			}
			d.FillRect(gx+float64(im-1)*(cell+1), y+float64(5-pr)*(cell+1), cell, cell, c)
		}
	}
	// aynı hücredeki riskleri yan yana diz
	counts := map[[2]int]int{}
	for i, r := range risks {
		if i >= 8 {
			break
		}
		key := [2]int{r.Prob, r.Impact}
		off := counts[key]
		counts[key]++
		cx := gx + float64(r.Impact-1)*(cell+1) + 5 + float64(off%2)*8
		cy := y + float64(5-r.Prob)*(cell+1) + 5 + float64(off/2)*8
		d.FillCircle(cx+1, cy+1, 4.2, k.WarnFg)
		d.TextC(cx+1, cy+3.4, fmt.Sprint(r.No), true, 5.8, k.White)
	}
	d.Text(gx, y+5*(cell+1)+8, "ETKİ →", true, 5.4, k.Dim)
	d.Text(x, y+5*(cell+1)+17, "↑ olasılık (1–5)", false, 5.4, k.Dim)
	lx := gx + 5*(cell+1) + 8
	ly := y + 6
	for i, r := range risks {
		if i >= 6 {
			break
		}
		d.FillCircle(lx+3, ly-2.3, 3.2, k.WarnFg)
		d.TextC(lx+3, ly-0.3, fmt.Sprint(r.No), true, 4.8, k.White)
		d.Text(lx+9, ly, d.Fit(r.Title, false, 6.4, x+w-lx-12), false, 6.4, k.Ink)
		ly += 11.5
	}
}

func quarterMonthsText(start time.Time) string {
	var m []string
	for i := 0; i < 3; i++ {
		m = append(m, TRMonth(int(start.AddDate(0, i, 0).Month())))
	}
	return strings.Join(m, " · ")
}

func curveAtMonth(pts []CurvePt, m string) *CurvePt {
	for i := range pts {
		if pts[i].Month == m {
			return &pts[i]
		}
	}
	return nil
}

func spiOf(c CurvePt) float64 {
	if c.PV <= 0 {
		return 0
	}
	return c.EV / c.PV
}

func cpiOf(c CurvePt) float64 {
	if c.AC <= 0 {
		return 0
	}
	return c.EV / c.AC
}

func signF(v float64) string {
	if v >= 0 {
		return "+"
	}
	return "−"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
