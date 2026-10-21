// Hakediş özet PDF'inin alternatif düzenleri: "Özet panel" ve "İzlenebilir akış".
// Resmi rapor düzeni pdf.go'dadır; üçü de aynı veriyi (PDFData) ve aynı PDF
// motorunu kullanır, yalnızca yerleşim farklıdır.
package payments

import (
	"fmt"
	"math"
	"strings"
)

var (
	colNavy2    = hexc(0x1d3d68)
	colOK       = hexc(0x1f7a4d)
	colWait     = hexc(0xb26a00)
	colLate     = hexc(0x9aa6b4)
	colRed      = hexc(0xc4584a)
	colRejected = hexc(0x9b2c22)
	colAmberBg  = hexc(0xfbf3cf)
	colHeroDim  = hexc(0xc9d4e3)
)

// deductionLabel — kesinti satırının PDF'teki etiketi (kategori adı + açıklama + oran).
func deductionLabel(ded DeductionLine) string {
	lbl := dedLabel(ded.Type)
	fl, fd := foldTR(lbl), foldTR(ded.Description)
	switch {
	case fd == "" || fd == fl:
	case strings.HasPrefix(fd, fl):
		if rest := strings.TrimSpace(strings.TrimLeft(string([]rune(ded.Description)[len([]rune(lbl)):]), " ·—-")); rest != "" {
			lbl += " " + rest
		}
	default:
		lbl += " · " + ded.Description
	}
	if ded.RatePct != nil && *ded.RatePct > 0 && !strings.Contains(lbl, "%") {
		lbl += fmt.Sprintf(" (%%%s)", trimPct(*ded.RatePct))
	}
	return lbl
}

type pdfSumRow struct {
	k, label, val string
	tot           bool
}

func summaryRows(d PDFData) []pdfSumRow {
	rows := []pdfSumRow{
		{"A", "Kümülatif imalat tutarı", trNum(d.GrossCum, 2), false},
		{"B", "Önceki dönem kümülatifi", trNum(d.GrossPrev, 2), false},
		{"C", "Bu dönem brüt (A − B)", trNum(d.GrossThis, 2), true},
		{"D", fmt.Sprintf("KDV (%%%s)", trimPct(d.VatPct)), trNum(d.VatAmount, 2), false},
	}
	if d.VatWithheld > 0 {
		rows = append(rows, pdfSumRow{"", "Tevkif edilen KDV (ödemeye eklenmez)", "− " + trNum(d.VatWithheld, 2), false})
	}
	return append(rows, pdfSumRow{"E", "Toplam kesinti", "− " + trNum(d.TotalDeductions, 2), false})
}

func payFormula(d PDFData) string {
	if d.VatWithheld > 0 {
		return "C + tahsil edilen KDV − E"
	}
	return "C + D − E"
}

func currencyOf(d PDFData) string {
	if d.Currency == "" {
		return "TRY"
	}
	return d.Currency
}

func capPct(v float64) float64 { return math.Max(0, math.Min(100, v)) }

// ── B · Özet panel ────────────────────────────────────────────────────────

func buildPanelPDF(d PDFData) []byte {
	cur := currencyOf(d)
	docNo := fmt.Sprintf("HK-%s-%04d", d.ProjectCode, d.PeriodNo)
	footer := fmt.Sprintf("%s · %s · Dönem %d", docNo, d.Subcontractor, d.PeriodNo)
	p := newPdfDoc()
	p.newPage()

	// Koyu başlık bandı
	const heroH = 112.0
	p.fillRect(0, 0, pgW, heroH, colNavy)
	p.brandOn(mL, 30)
	title := fmt.Sprintf("Hakediş #%d · %s", d.PeriodNo, d.Subcontractor)
	p.text(mL, 52, p.fit(title, true, 15, contentW-130), true, 15, colWhite)
	period := fmt.Sprintf("Dönem %d", d.PeriodNo)
	if d.PeriodStart != "" || d.PeriodEnd != "" {
		period = trDate(d.PeriodStart) + " – " + trDate(d.PeriodEnd)
	}
	contractNo := d.ContractNo
	if contractNo == "" {
		contractNo = "sözleşme no yok"
	}
	sub := fmt.Sprintf("%s (%s) · %s · %s", d.ProjectName, d.ProjectCode, contractNo, period)
	p.text(mL, 64, p.fit(sub, false, 7.5, contentW-130), false, 7.5, colHeroDim)

	label, _, _ := statusStyle(d.Status)
	cw := p.width(trUpper(label), true, 7) + 14
	p.fillRect(pgW-mR-cw, 20, cw, 12, colAmber)
	p.textR(pgW-mR-7, 28.5, trUpper(label), true, 7, colNavy)
	p.textR(pgW-mR, 44, docNo, false, 7.5, colWhite)
	p.textR(pgW-mR, 55, d.GeneratedAt.Format("02.01.2006"), false, 7.5, colHeroDim)

	// Dört ana rakam
	sozBedel := "—"
	if d.ContractAmount > 0 {
		sozBedel = trNum(d.ContractAmount, 0)
	}
	kpis := [][2]string{
		{"Sözleşme bedeli", sozBedel}, {"Kümülatif", trNum(d.GrossCum, 0)},
		{"Bu dönem brüt", trNum(d.GrossThis, 0)}, {"Ödenecek", trNum(d.NetPayable, 0)},
	}
	kw := (contentW - 3) / 4
	for i, k := range kpis {
		kx := mL + float64(i)*(kw+1)
		bg, fgL, fgV := colNavy2, colHeroDim, colWhite
		if i == 3 {
			bg, fgL, fgV = colAmber, colNavy, colNavy
		}
		p.fillRect(kx, 74, kw, 30, bg)
		p.text(kx+8, 85, trUpper(k[0]), true, 6, fgL)
		p.text(kx+8, 98, p.fit(k[1], true, 12, kw-14), true, 12, fgV)
	}
	y := heroH + 18

	// Sözleşme ilerleme çubuğu
	if d.ContractAmount > 0 {
		pO := capPct(d.GrossPrev / d.ContractAmount * 100)
		pB := capPct(d.GrossThis / d.ContractAmount * 100)
		if pO+pB > 100 {
			pB = 100 - pO
		}
		pK := d.GrossCum / d.ContractAmount * 100
		p.text(mL, y+7, trUpper(fmt.Sprintf("Sözleşme ilerlemesi · %%%s", trNum(pK, 1))), true, 7.5, colNavy)
		y += 12
		p.fillRect(mL, y, contentW, 12, colWash)
		p.strokeRect(mL, y, contentW, 12, colRule, 0.5)
		p.fillRect(mL, y, contentW*pO/100, 12, colNavy2)
		p.fillRect(mL+contentW*pO/100, y, contentW*pB/100, 12, colAmber)
		y += 22
		lx := mL
		for _, lg := range []struct {
			c   rgb
			txt string
		}{
			{colNavy2, fmt.Sprintf("Önceki dönemler %%%s", trNum(pO, 1))},
			{colAmber, fmt.Sprintf("Bu dönem %%%s", trNum(pB, 1))},
			{colWash, fmt.Sprintf("Kalan %%%s", trNum(math.Max(0, 100-pK), 1))},
		} {
			p.fillRect(lx, y-6, 7, 7, lg.c)
			p.strokeRect(lx, y-6, 7, 7, colRule, 0.4)
			p.text(lx+11, y, lg.txt, false, 7, colDim)
			lx += 11 + p.width(lg.txt, false, 7) + 16
		}
		y += 16
	}

	// İmalat kalemleri (ilerleme göstergeli)
	p.text(mL, y+7, "İMALAT KALEMLERİ", true, 7.5, colNavy)
	y += 12
	cols := []tblCol{
		{"POZ", 40, false}, {"AÇIKLAMA", 140, false}, {"BİRİM", 28, false}, {"B. FİYAT", 52, true},
		{"ÖNCEKİ", 44, true}, {"BU DÖNEM", 44, true}, {"KÜM.", 44, true}, {"İLERLEME", 70, false}, {"TUTAR", 73, true},
	}
	progX := mL
	for _, c := range cols[:7] {
		progX += c.w
	}
	y = p.tableHeader(y, cols)
	if len(d.Lines) == 0 {
		p.text(mL+7, y+11, "Bu hakedişte imalat kalemi yok.", false, 8, colDim)
		y += rowH
	}
	for i, l := range d.Lines {
		if y+rowH+rowH > pgBottom {
			p.newPage()
			y = p.contHeader(footer, d)
			y = p.tableHeader(y, cols)
		}
		if i%2 == 1 {
			p.fillRect(mL, y, contentW, rowH, colWash)
		}
		p.tableRow(y, cols, []string{
			l.PozNo, l.Description, l.Unit, trNum(l.UnitPrice, 2),
			trQty(l.PrevCumQty), trQty(l.ThisPeriodQty), trQty(l.CumQty), "", trNum(l.ThisAmount, 2),
		}, false)
		if l.ContractQty > 0 {
			pc := l.CumQty / l.ContractQty * 100
			p.fillRect(progX+6, y+4.5, 30, 6, colWash)
			p.strokeRect(progX+6, y+4.5, 30, 6, colRule, 0.4)
			p.fillRect(progX+6, y+4.5, 30*capPct(pc)/100, 6, colNavy2)
			p.text(progX+40, y+10.5, "%"+trNum(pc, 0), false, 7, colInk)
		} else {
			p.text(progX+6, y+10.5, "—", false, 8, colDim)
		}
		p.hline(mL, mL+contentW, y+rowH, colRule, 0.5)
		y += rowH
	}
	p.fillRect(mL, y, contentW, rowH+1, colWash)
	p.fillRect(mL, y, contentW, 1.5, colNavy)
	p.text(mL+7, y+11.5, "Bu dönem brüt hakediş", true, 8, colInk)
	p.textR(mL+contentW-7, y+11.5, trNum(d.GrossThis, 2), true, 8, colInk)
	y += rowH + 1 + 18

	// Brütten ödenecek tutara (yatay çubuklar) + hesap özeti
	const wfRow = 17.0
	rows := summaryRows(d)
	wfH := 12 + float64(3+len(d.Deductions))*wfRow
	sumH := 12 + float64(len(rows))*rowH + 6 + 36
	if y+math.Max(wfH, sumH) > pgBottom {
		p.newPage()
		y = p.contHeader(footer, d)
	}
	leftW, rightW := 300.0, contentW-300.0-16
	leftX, rightX := mL, mL+leftW+16

	p.text(leftX, y+7, "BRÜTTEN ÖDENECEK TUTARA", true, 7.5, colNavy)
	max := d.GrossThis + d.VatAmount
	if max <= 0 {
		max = 1
	}
	const lblW, trackW = 100.0, 128.0
	trackX := leftX + lblW + 4
	bw := func(v float64) float64 { return math.Max(0, math.Min(trackW, v/max*trackW)) }
	wy := y + 12
	wfLine := func(lbl string, val float64, bold bool) float64 {
		p.text(leftX, wy+10, p.fit(lbl, bold, 7.5, lblW), bold, 7.5, colInk)
		p.fillRect(trackX, wy+2, trackW, 10, colWash)
		p.textR(leftX+leftW, wy+10, trNum(val, 2), bold, 7.5, colInk)
		return wy
	}
	wfLine("Bu dönem brüt", d.GrossThis, false)
	p.fillRect(trackX, wy+2, bw(d.GrossThis), 10, colNavy2)
	wy += wfRow
	wfLine(fmt.Sprintf("+ KDV %%%s", trimPct(d.VatPct)), d.VatAmount, false)
	p.fillRect(trackX+bw(d.GrossThis), wy+2, math.Min(bw(d.VatAmount), trackW-bw(d.GrossThis)), 10, colLate)
	wy += wfRow
	for _, ded := range d.Deductions {
		wfLine("− "+deductionLabel(ded), ded.Amount, false)
		w := bw(ded.Amount)
		p.fillRect(trackX+trackW-w, wy+2, w, 10, colRed)
		wy += wfRow
	}
	wfLine("Ödenecek", d.NetPayable, true)
	p.fillRect(trackX, wy+2, bw(d.NetPayable), 10, colAmber)

	p.text(rightX, y+7, "HESAP ÖZETİ", true, 7.5, colNavy)
	ry := y + 12
	for _, r := range rows {
		if r.tot {
			p.fillRect(rightX, ry, rightW, rowH, colWash)
		}
		p.text(rightX+4, ry+11, r.k, true, 8, colNavy)
		p.text(rightX+20, ry+11, p.fit(r.label, r.tot, 8, rightW-28-p.width(r.val, r.tot, 8)), r.tot, 8, colInk)
		p.textR(rightX+rightW-4, ry+11, r.val, r.tot, 8, colInk)
		p.hline(rightX, rightX+rightW, ry+rowH, colRule, 0.5)
		ry += rowH
	}
	ry += 6
	p.fillRect(rightX, ry, rightW, 36, colNavy)
	p.text(rightX+9, ry+13, trUpper("Ödenecek ("+payFormula(d)+")"), true, 6.5, colHeroDim)
	p.textR(rightX+rightW-9, ry+29, trNum(d.NetPayable, 2)+" "+cur, true, 13, colWhite)

	return p.finish(footer)
}

// ── C · İzlenebilir akış ──────────────────────────────────────────────────

func buildFlowPDF(d PDFData) []byte {
	cur := currencyOf(d)
	docNo := fmt.Sprintf("HK-%s-%04d", d.ProjectCode, d.PeriodNo)
	footer := fmt.Sprintf("%s · %s · Dönem %d", docNo, d.Subcontractor, d.PeriodNo)
	p := newPdfDoc()
	p.newPage()

	// Başlık
	y := firstTopY
	p.brand(mL, y+10)
	p.text(mL, y+30, fmt.Sprintf("HAKEDİŞ #%d · TAKİP ÖZETİ", d.PeriodNo), true, 15, colNavy)
	contractNo := d.ContractNo
	if contractNo == "" {
		contractNo = "—"
	}
	p.text(mL, y+41, p.fit(fmt.Sprintf("%s · %s (%s) · %s", d.Subcontractor, d.ProjectName, d.ProjectCode, contractNo), false, 7.5, contentW-140), false, 7.5, colDim)
	period := "—"
	if d.PeriodStart != "" || d.PeriodEnd != "" {
		period = trDate(d.PeriodStart) + " – " + trDate(d.PeriodEnd)
	}
	label, fg, bg := statusStyle(d.Status)
	my := y + 9.0
	for _, m := range [][2]string{{"Belge no", docNo}, {"Dönem", period}} {
		p.textR(pgW-mR, my, m[1], true, 8, colInk)
		p.textR(pgW-mR-p.width(m[1], true, 8)-8, my, m[0], false, 8, colDim)
		my += 11
	}
	cw := p.width(trUpper(label), true, 7) + 14
	p.fillRect(pgW-mR-cw, my-7.5, cw, 11.5, bg)
	p.textR(pgW-mR-7, my+0.5, trUpper(label), true, 7, fg)
	y += 46
	p.fillRect(mL, y, contentW, 2.25, colNavy)
	startY := y + 16

	const sideW = 150.0
	mx, mw := mL+sideW+16, contentW-sideW-16
	p.vline(mL+sideW+8, startY-4, pgBottom-4, colRule, 0.75)

	// ── Sol sütun: onay zinciri + ödeme planı ──
	sy := startY
	done := 0
	for _, c := range d.Chain {
		if c.State == "done" {
			done++
		}
	}
	p.text(mL, sy+7, trUpper(fmt.Sprintf("Onay zinciri · %d / %d", done, len(d.Chain))), true, 7.5, colNavy)
	sy += 18
	if len(d.Chain) == 0 {
		p.text(mL, sy+8, "Onay zinciri tanımlı değil.", false, 7.5, colDim)
		sy += 20
	}
	const pitch = 27.0
	for i, c := range d.Chain {
		dot := colLate
		switch c.State {
		case "done":
			dot = colOK
		case "now":
			dot = colAmber
		case "rej":
			dot = colRejected
		}
		if i < len(d.Chain)-1 {
			lc := colRule
			if c.State == "done" {
				lc = colOK
			}
			p.vline(mL+4, sy+6, sy+pitch+2, lc, 1.5)
		}
		if c.State == "now" {
			p.fillCircle(mL+4, sy+4, 6.5, colAmberBg)
		}
		p.fillCircle(mL+4, sy+4, 3.8, dot)
		nameCol := colInk
		if c.State == "now" {
			nameCol = colWait
		}
		p.text(mL+16, sy+6, p.fit(c.Label, true, 7.5, sideW-18), true, 7.5, nameCol)
		sub := c.Actor
		if c.Date != "" {
			if sub != "" {
				sub += " · "
			}
			sub += c.Date
		}
		if sub == "" && c.State == "now" {
			sub = "bekliyor"
		}
		if sub != "" {
			p.text(mL+16, sy+15, p.fit(sub, false, 6.5, sideW-18), false, 6.5, colDim)
		}
		sy += pitch
	}
	sy += 8
	p.text(mL, sy+7, "ÖDEME PLANI", true, 7.5, colNavy)
	sy += 14
	var planned float64
	if len(d.Plan) == 0 {
		p.text(mL, sy+8, "Ödeme planı girilmemiş.", false, 7.5, colDim)
	}
	for _, pl := range d.Plan {
		planned += pl.Amount
		amt := trNum(pl.Amount, 2)
		p.text(mL, sy+9, p.fit(strings.TrimSpace(pl.Method+" · "+pl.Date), false, 7, sideW-p.width(amt, true, 7)-8), false, 7, colDim)
		p.textR(mL+sideW, sy+9, amt, true, 7, colInk)
		p.hline(mL, mL+sideW, sy+13, colRule, 0.5)
		sy += 14
	}
	if len(d.Plan) > 0 {
		for _, r := range [][2]string{{"Planlanan", trNum(planned, 2)}, {"Ödenecek", trNum(d.NetPayable, 2)}} {
			p.text(mL, sy+9, r[0], true, 7, colInk)
			p.textR(mL+sideW, sy+9, r[1], true, 7, colInk)
			sy += 12
		}
	}

	// ── Sağ sütun: rakamlar, geçmiş, imalat, ödeme yolu ──
	y = startY
	ensure := func(need float64) {
		if y+need > pgBottom {
			p.newPage()
			y = p.contHeader(footer, d)
		}
	}
	sw := mw / 3
	ilerleme := "—"
	if d.ContractAmount > 0 {
		ilerleme = "%" + trNum(d.GrossCum/d.ContractAmount*100, 1)
	}
	for i, st := range [][2]string{{"Kümülatif ilerleme", ilerleme}, {"Bu dönem brüt", trNum(d.GrossThis, 0)}, {"Ödenecek", trNum(d.NetPayable, 0)}} {
		sx := mx + float64(i)*sw
		p.text(sx, y+7, trUpper(st[0]), true, 6.3, colDim)
		col := colNavy
		if i == 2 {
			col = colWait
		}
		p.text(sx, y+22, st[1], true, 12, col)
	}
	y += 38

	// Dönem geçmişi
	histCols := []tblCol{{"DÖNEM", 38, false}, {"BRÜT", 62, true}, {"KÜMÜLATİF", 68, true}, {"%", 34, true}, {"NET", 68, true}, {"DURUM", 99, false}}
	ensure(12 + 17 + rowH*2)
	p.text(mx, y+7, "DÖNEM GEÇMİŞİ", true, 7.5, colNavy)
	y += 12
	y = p.tableHeaderAt(mx, mw, y, histCols)
	pctOf := func(cum float64) string {
		if d.ContractAmount > 0 {
			return trNum(cum/d.ContractAmount*100, 1)
		}
		return "—"
	}
	var cum float64
	for _, h := range d.History {
		ensure(rowH * 2)
		cum += h.Gross
		lbl, _, _ := statusStyle(h.Status)
		if h.Date != "" {
			lbl += " " + h.Date
		}
		p.tableRowAt(mx, y, histCols, []string{
			fmt.Sprintf("#%d", h.No), trNum(h.Gross, 0), trNum(cum, 0), pctOf(cum), trNum(h.Net, 0), lbl,
		}, false)
		p.hline(mx, mx+mw, y+rowH, colRule, 0.5)
		y += rowH
	}
	ensure(rowH + 4)
	p.fillRect(mx, y, mw, rowH, colAmberBg)
	curLbl, _, _ := statusStyle(d.Status)
	p.tableRowAt(mx, y, histCols, []string{
		fmt.Sprintf("#%d", d.PeriodNo), trNum(d.GrossThis, 0), trNum(d.GrossCum, 0), pctOf(d.GrossCum), trNum(d.NetPayable, 0), curLbl,
	}, true)
	y += rowH + 16

	// Bu dönem imalat
	itemCols := []tblCol{{"POZ", 40, false}, {"AÇIKLAMA", 107, false}, {"BİRİM", 28, false}, {"B. FİYAT", 54, true}, {"BU DÖNEM", 52, true}, {"TUTAR", 88, true}}
	ensure(12 + 17 + rowH*3)
	p.text(mx, y+7, "BU DÖNEM İMALAT", true, 7.5, colNavy)
	y += 12
	y = p.tableHeaderAt(mx, mw, y, itemCols)
	if len(d.Lines) == 0 {
		p.text(mx+7, y+11, "Bu hakedişte imalat kalemi yok.", false, 8, colDim)
		y += rowH
	}
	for i, l := range d.Lines {
		if y+rowH*2 > pgBottom {
			p.newPage()
			y = p.contHeader(footer, d)
			y = p.tableHeaderAt(mx, mw, y, itemCols)
		}
		if i%2 == 1 {
			p.fillRect(mx, y, mw, rowH, colWash)
		}
		p.tableRowAt(mx, y, itemCols, []string{
			l.PozNo, l.Description, l.Unit, trNum(l.UnitPrice, 2), trQty(l.ThisPeriodQty), trNum(l.ThisAmount, 2),
		}, false)
		p.hline(mx, mx+mw, y+rowH, colRule, 0.5)
		y += rowH
	}
	y += 16

	// Brütten ödenecek tutara
	ensure(12 + rowH*float64(2+len(d.Deductions)) + 6 + 36)
	p.text(mx, y+7, "BRÜTTEN ÖDENECEK TUTARA", true, 7.5, colNavy)
	y += 12
	line := func(lbl, val string, bold, wash bool, indent float64) {
		if wash {
			p.fillRect(mx, y, mw, rowH, colWash)
		}
		col := colInk
		if indent > 0 {
			col = colDim
		}
		p.text(mx+4+indent, y+11, p.fit(lbl, bold, 8, mw-16-indent-p.width(val, bold, 8)), bold, 8, col)
		p.textR(mx+mw-4, y+11, val, bold, 8, colInk)
		p.hline(mx, mx+mw, y+rowH, colRule, 0.5)
		y += rowH
	}
	line("C · Bu dönem brüt", trNum(d.GrossThis, 2), true, true, 0)
	line(fmt.Sprintf("+ KDV (%%%s)", trimPct(d.VatPct)), trNum(d.VatAmount, 2), false, false, 0)
	for _, ded := range d.Deductions {
		line("− "+deductionLabel(ded), trNum(ded.Amount, 2), false, false, 8)
	}
	y += 6
	p.fillRect(mx, y, mw, 36, colNavy)
	p.text(mx+9, y+13, trUpper("Yükleniciye ödenecek"), true, 6.5, colHeroDim)
	p.textR(mx+mw-9, y+29, trNum(d.NetPayable, 2)+" "+cur, true, 13, colWhite)

	return p.finish(footer)
}
