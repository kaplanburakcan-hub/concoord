// Haftalık/Günlük Rapor PDF'leri — payments/pdf.go ile aynı bilinçli karar
// (ADR-0004): stdlib ile deterministik, bağımlılıksız üretim. Base-14
// Helvetica WinAnsi Türkçe karakter taşımadığından metin ASCII'ye çevrilir.
// Zengin HTML→PDF şablonu Faz 9 yönetim raporlarıyla değerlendirilir.
//
// KURAL: PDF'e giren HER sayı Snapshot'tan (veya dailyDTO'dan) gelir —
// başka kaynak yok (Plan Faz 6 kabul kriteri: rakamlar doğrulanabilir).
//
// Bölüm çerçeveleme: section() ile açılan her blok (ör. bir günün verisi,
// "AKTİF TAŞERON LİSTESİ" gibi bir başlık) sayfada ince bir çerçeve içinde
// tutulur; blok mevcut sayfaya sığmıyorsa ama tek sayfaya sığacak kadar
// küçükse önce sayfa kırılır (böylece başlık sayfa sonunda öksüz kalmaz);
// blok tek sayfadan uzunsa "(devam)" başlığıyla yeniden çerçevelenerek
// sonraki sayfa(lar)da sürer. Bu, veri miktarı değiştikçe bölümlerin
// birbirine karışmasını önler.
package reports

import (
	"bytes"
	"fmt"
	"strings"
)

// BuildWeeklyPDF — snapshot'tan çok sayfalı, bölüm çerçeveli PDF baytları üretir.
func BuildWeeklyPDF(sn Snapshot) []byte {
	var lines []textLine
	add := func(bold bool, size float64, s string) {
		lines = append(lines, textLine{bold: bold, size: size, text: asciiTR(s)})
	}
	gap := func() { lines = append(lines, textLine{gap: true}) }
	section := func(title string) {
		lines = append(lines, textLine{bold: true, size: 10, text: asciiTR(title), newBlock: true})
	}

	add(true, 15, "HAFTALIK ILERLEME RAPORU")
	add(false, 9, fmt.Sprintf("Proje: %s (%s)", sn.ProjectName, sn.ProjectCode))
	add(false, 9, fmt.Sprintf("Hafta: %d   Donem: %s - %s", sn.WeekNo, sn.PeriodStart, sn.PeriodEnd))
	add(false, 8, fmt.Sprintf("Uretim ani (snapshot): %s UTC", sn.GeneratedAt.Format("02.01.2006 15:04")))
	gap()

	// Hafta özeti.
	section("HAFTA OZETI")
	add(false, 9, kv("Raporlanan gun sayisi", itoa(sn.Totals.DaysReported)))
	add(false, 9, kv("Toplam personel (adam-gun)", itoa(sn.Totals.ManpowerDays)))
	add(false, 9, kv("Ekipman calisma saati", num(sn.Totals.EquipmentHours, 1)))
	add(false, 9, kv("Imalat girdisi sayisi", itoa(sn.Totals.WorkEntryCount)))

	// Günler — her gün kendi çerçevesinde (veri miktarı gün gün değişir).
	for _, d := range sn.Days {
		hdr := fmt.Sprintf("GUN: %s  (rev %d, %s)", d.Date, d.RevisionNo, statusTR(d.Status))
		section(hdr)
		wline := "Hava: "
		if d.WeatherCond != "" {
			wline += d.WeatherCond + "  "
		}
		if d.TempMin != nil || d.TempMax != nil {
			wline += fmt.Sprintf("Sicaklik: %s / %s C", numPtr(d.TempMin), numPtr(d.TempMax))
		}
		if wline != "Hava: " {
			add(false, 8, wline)
		}
		if d.ManpowerTotal > 0 {
			add(false, 8, fmt.Sprintf("Personel toplami: %d", d.ManpowerTotal))
			for _, m := range d.Manpower {
				sub := ""
				if m.SubcontractorName != nil {
					sub = " [" + *m.SubcontractorName + "]"
				}
				add(false, 8, fmt.Sprintf("  - %s%s: %d", m.Trade, sub, m.Headcount))
			}
		}
		for _, e := range d.Equipment {
			l := fmt.Sprintf("  Ekipman: %s x%d", e.EquipmentName, e.Count)
			if e.WorkingHours != nil {
				l += fmt.Sprintf(", %s saat", num(*e.WorkingHours, 1))
			}
			if e.IdleReason != nil && *e.IdleReason != "" {
				l += " (bekleme: " + trunc(*e.IdleReason, 30) + ")"
			}
			add(false, 8, l)
		}
		for _, we := range d.WorkEntries {
			l := "  Imalat: " + trunc(we.Description, 44)
			if we.WorkItemPoz != nil {
				l += " [" + *we.WorkItemPoz + "]"
			}
			if we.Qty != nil {
				u := ""
				if we.Unit != nil {
					u = " " + *we.Unit
				}
				l += fmt.Sprintf(" — %s%s", num(*we.Qty, 3), u)
			}
			if we.Location != nil && *we.Location != "" {
				l += " @" + trunc(*we.Location, 20)
			}
			add(false, 8, l)
		}
		if d.Notes != "" {
			add(false, 8, "  Not: "+trunc(d.Notes, 70))
		}
	}
	if len(sn.Days) == 0 {
		section("GUNLUK RAPORLAR")
		add(false, 9, "Bu hafta icin gunluk rapor girilmemis.")
	}

	// Haftalık kontrol ritmi özetleri (Plan §7).
	section("BEKLEYEN HAKEDIS STATULERI")
	if len(sn.PendingPayments) == 0 {
		add(false, 9, "Bekleyen hakedis yok.")
	}
	for _, p := range sn.PendingPayments {
		add(false, 9, fmt.Sprintf("- %s / Donem %d: %s", p.Subcontractor, p.PeriodNo, statusTR(p.Status)))
	}

	section("GOREV OZETI")
	add(false, 9, kv("Acik gorev", itoa(sn.OpenTasks)))
	add(false, 9, kv("Bu hafta terminli", itoa(sn.TasksDueWeek)))

	section("MALZEME ONAYLARI (MAR)")
	add(false, 9, kv("Bekleyen MAR", itoa(sn.PendingMARs)))

	if sn.OHSNote != "" {
		section("ISG")
		add(false, 8, sn.OHSNote)
	}

	return renderPDF(lines)
}

// BuildDailyPDF — tek bir günlük raporun bölüm çerçeveli PDF baytlarını üretir.
func BuildDailyPDF(projectName, projectCode string, d dailyDTO) []byte {
	var lines []textLine
	add := func(bold bool, size float64, s string) {
		lines = append(lines, textLine{bold: bold, size: size, text: asciiTR(s)})
	}
	gap := func() { lines = append(lines, textLine{gap: true}) }
	section := func(title string) {
		lines = append(lines, textLine{bold: true, size: 10, text: asciiTR(title), newBlock: true})
	}

	add(true, 15, "GUNLUK SAHA RAPORU")
	add(false, 9, fmt.Sprintf("Proje: %s (%s)", projectName, projectCode))
	add(false, 9, fmt.Sprintf("Tarih: %s   Revizyon: %d   Durum: %s", d.ReportDate, d.RevisionNo, statusTR(d.Status)))
	wline := ""
	if d.TempMin != nil || d.TempMax != nil {
		wline = fmt.Sprintf("Sicaklik: %s / %s C", numPtr(d.TempMin), numPtr(d.TempMax))
	}
	if wline != "" {
		add(false, 8, wline)
	}
	add(false, 8, fmt.Sprintf("Raporu giren: %s", d.AuthorName))
	gap()

	if len(d.Manpower) == 0 {
		section("PERSONEL")
		add(false, 9, "Personel girisi yapilmamis.")
	} else {
		section("PERSONEL")
		for _, m := range d.Manpower {
			sub := ""
			if m.SubcontractorName != nil {
				sub = " [" + *m.SubcontractorName + "]"
			}
			add(false, 9, fmt.Sprintf("- %s%s: %d kisi", m.Trade, sub, m.Headcount))
		}
	}

	if len(d.Equipment) > 0 {
		section("EKIPMAN")
		for _, e := range d.Equipment {
			l := fmt.Sprintf("- %s x%d", e.EquipmentName, e.Count)
			if e.WorkingHours != nil {
				l += fmt.Sprintf(", %s saat", num(*e.WorkingHours, 1))
			}
			if e.IdleReason != nil && *e.IdleReason != "" {
				l += " (bekleme: " + trunc(*e.IdleReason, 40) + ")"
			}
			add(false, 9, l)
		}
	}

	if len(d.WorkEntries) > 0 {
		section("IMALAT / IS KALEMLERI")
		for _, we := range d.WorkEntries {
			l := "- " + trunc(we.Description, 50)
			if we.WorkItemPoz != nil {
				l += " [" + *we.WorkItemPoz + "]"
			}
			if we.Qty != nil {
				u := ""
				if we.Unit != nil {
					u = " " + *we.Unit
				}
				l += fmt.Sprintf(" — %s%s", num(*we.Qty, 3), u)
			}
			if we.Location != nil && *we.Location != "" {
				l += " @" + trunc(*we.Location, 24)
			}
			add(false, 9, l)
		}
	}

	if len(d.CashExpenses) > 0 {
		section("SANTIYE KASA HARCAMASI")
		var total float64
		for _, c := range d.CashExpenses {
			l := fmt.Sprintf("- %s (%s): %s TRY", trunc(c.Description, 40), c.Category, num(c.Amount, 2))
			if c.ReceiptNo != nil && *c.ReceiptNo != "" {
				l += " [Fis: " + *c.ReceiptNo + "]"
			}
			add(false, 9, l)
			total += c.Amount
		}
		add(true, 9, kv("Toplam", num(total, 2)+" TRY"))
	}

	if d.Notes != nil && *d.Notes != "" {
		section("NOTLAR")
		add(false, 9, *d.Notes)
	}

	return renderPDF(lines)
}

func statusTR(s string) string {
	switch s {
	case "Draft":
		return "Taslak"
	case "Submitted":
		return "Gonderildi"
	case "SiteApproved":
		return "Saha Onayli"
	default:
		return s
	}
}

func kv(label, value string) string {
	return padRight(label, 40) + padLeft(value, 16)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func num(f float64, dec int) string { return fmt.Sprintf("%.*f", dec, f) }

func numPtr(f *float64) string {
	if f == nil {
		return "-"
	}
	return num(*f, 1)
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}
func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}
func trunc(s string, w int) string {
	s = asciiTR(s)
	if len(s) <= w {
		return s
	}
	return s[:w-1] + "."
}

// asciiTR — Türkçe karakterleri PDF-güvenli ASCII'ye çevirir (payments ile aynı).
func asciiTR(s string) string {
	r := strings.NewReplacer(
		"ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G", "ı", "i", "İ", "I",
		"ç", "c", "Ç", "C", "ö", "o", "Ö", "O", "ü", "u", "Ü", "U",
	)
	out := r.Replace(s)
	var sb strings.Builder
	for _, ch := range out {
		if ch < 128 {
			sb.WriteRune(ch)
		} else {
			sb.WriteByte('?')
		}
	}
	return sb.String()
}

// --- minimal PDF motoru: bölüm-çerçeveli sayfa akışı ---

type textLine struct {
	bold     bool
	size     float64
	text     string
	gap      bool
	newBlock bool // bu satır yeni bir çerçeveli bölümün başlığıdır
}

type pdfBlock struct {
	framed bool
	lines  []textLine
}

// splitBlocks — newBlock işaretli satırlardan önce akan satırlar çerçevesiz
// bir "önsöz" bloğu oluşturur; her newBlock işaretinden itibaren yeni bir
// çerçeveli blok başlar (işaretli satırın kendisi o bloğun başlığıdır).
func splitBlocks(lines []textLine) []pdfBlock {
	var blocks []pdfBlock
	var cur []textLine
	framed := false
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, pdfBlock{framed: framed, lines: cur})
		}
		cur = nil
	}
	for _, ln := range lines {
		if ln.newBlock {
			flush()
			framed = true
		}
		cur = append(cur, ln)
	}
	flush()
	return blocks
}

func renderPDF(lines []textLine) []byte {
	const (
		pageW, pageH = 595.0, 842.0
		left         = 40.0
		right        = pageW - 40.0
		top          = 800.0
		bottom       = 48.0
		pad          = 6.0
		blockGap     = 10.0
	)

	lineHeight := func(ln textLine) float64 {
		if ln.gap {
			return 8
		}
		return ln.size + 5
	}

	var pages []*strings.Builder
	cur := &strings.Builder{}
	pages = append(pages, cur)
	y := top

	newPage := func() {
		cur = &strings.Builder{}
		pages = append(pages, cur)
		y = top
	}

	writeText := func(ln textLine, x, yy float64) {
		if ln.gap {
			return
		}
		font := "F1"
		if ln.bold {
			font = "F2"
		}
		fmt.Fprintf(cur, "BT /%s %.0f Tf %.0f %.0f Td (%s) Tj ET\n",
			font, ln.size, x, yy, escapePDF(ln.text))
	}
	writeRect := func(y0, y1 float64) {
		fmt.Fprintf(cur, "0.6 G\n0.75 w\n%.1f %.1f %.1f %.1f re S\n0 G\n",
			left-pad, y0, (right-left)+2*pad, y1-y0)
	}

	// place — mevcut sayfaya, y konumundan başlayarak sığdığı kadar satır
	// yazar; sığmayan artık satırları döndürür.
	place := func(ls []textLine, indent, limit float64) []textLine {
		i := 0
		for i < len(ls) {
			lh := lineHeight(ls[i])
			if y-lh < limit {
				break
			}
			writeText(ls[i], left+indent, y)
			y -= lh
			i++
		}
		return ls[i:]
	}

	fullPageH := top - bottom

	for _, blk := range splitBlocks(lines) {
		if !blk.framed {
			rem := blk.lines
			for len(rem) > 0 {
				rem = place(rem, 0, bottom)
				if len(rem) > 0 {
					newPage()
				}
			}
			continue
		}

		h := 0.0
		for _, ln := range blk.lines {
			h += lineHeight(ln)
		}
		boxH := h + 2*pad

		// Mevcut sayfaya sığmıyor ama tek sayfaya sığıyorsa: başlık öksüz
		// kalmasın diye önce sayfa kır.
		if boxH > y-pad-bottom && boxH <= fullPageH {
			newPage()
		}

		rem := blk.lines
		first := true
		for len(rem) > 0 {
			if !first {
				newPage()
				cont := blk.lines[0]
				cont.text += " (devam)"
				rem = append([]textLine{cont}, rem...)
			}
			top0 := y + pad
			y -= pad
			rem = place(rem, pad, bottom+pad)
			bottomY := y
			y -= pad
			writeRect(bottomY, top0)
			first = false
		}
		y -= blockGap
	}

	type obj struct{ body string }
	var objs []obj
	objs = append(objs, obj{}) // 0 kullanılmaz

	catalog := len(objs)
	objs = append(objs, obj{"<< /Type /Catalog /Pages 2 0 R >>"})
	pagesObj := len(objs)
	objs = append(objs, obj{})
	f1 := len(objs)
	objs = append(objs, obj{"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"})
	f2 := len(objs)
	objs = append(objs, obj{"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>"})

	var kids []string
	for _, pageBuf := range pages {
		content := pageBuf.String()
		contentObj := len(objs)
		objs = append(objs, obj{fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)})
		pageObj := len(objs)
		objs = append(objs, obj{fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.0f %.0f] "+
				"/Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
			pagesObj, pageW, pageH, f1, f2, contentObj)})
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObj))
	}
	objs[pagesObj] = obj{fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>",
		len(kids), strings.Join(kids, " "))}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objs))
	for i := 1; i < len(objs); i++ {
		offsets[i] = buf.Len()
		buf.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i, objs[i].body))
	}
	xref := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(objs)))
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i < len(objs); i++ {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF",
		len(objs), catalog, xref))
	return buf.Bytes()
}

func escapePDF(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\r", "", "\n", "")
	return r.Replace(s)
}
