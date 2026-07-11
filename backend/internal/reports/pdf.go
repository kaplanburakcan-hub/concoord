// Haftalık İlerleme Raporu PDF'i — payments/pdf.go ile aynı bilinçli karar
// (ADR-0004): stdlib ile deterministik, bağımlılıksız üretim. Base-14
// Helvetica WinAnsi Türkçe karakter taşımadığından metin ASCII'ye çevrilir.
// Zengin HTML→PDF şablonu Faz 9 yönetim raporlarıyla değerlendirilir.
//
// KURAL: PDF'e giren HER sayı Snapshot'tan gelir — başka kaynak yok
// (Plan Faz 6 kabul kriteri: rakamlar snapshot'tan doğrulanabilir).
package reports

import (
	"bytes"
	"fmt"
	"strings"
)

// BuildWeeklyPDF — snapshot'tan çok sayfalı PDF baytları üretir.
func BuildWeeklyPDF(sn Snapshot) []byte {
	var lines []textLine
	add := func(bold bool, size float64, s string) {
		lines = append(lines, textLine{bold: bold, size: size, text: asciiTR(s)})
	}
	gap := func() { lines = append(lines, textLine{gap: true}) }

	add(true, 15, "HAFTALIK ILERLEME RAPORU")
	add(false, 9, fmt.Sprintf("Proje: %s (%s)", sn.ProjectName, sn.ProjectCode))
	add(false, 9, fmt.Sprintf("Hafta: %d   Donem: %s - %s", sn.WeekNo, sn.PeriodStart, sn.PeriodEnd))
	add(false, 8, fmt.Sprintf("Uretim ani (snapshot): %s UTC", sn.GeneratedAt.Format("02.01.2006 15:04")))
	gap()

	// Hafta özeti.
	add(true, 10, "HAFTA OZETI")
	add(false, 9, kv("Raporlanan gun sayisi", itoa(sn.Totals.DaysReported)))
	add(false, 9, kv("Toplam personel (adam-gun)", itoa(sn.Totals.ManpowerDays)))
	add(false, 9, kv("Ekipman calisma saati", num(sn.Totals.EquipmentHours, 1)))
	add(false, 9, kv("Imalat girdisi sayisi", itoa(sn.Totals.WorkEntryCount)))
	gap()

	// Günler.
	for _, d := range sn.Days {
		hdr := fmt.Sprintf("GUN: %s  (rev %d, %s)", d.Date, d.RevisionNo, statusTR(d.Status))
		add(true, 10, hdr)
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
		gap()
	}
	if len(sn.Days) == 0 {
		add(false, 9, "Bu hafta icin gunluk rapor girilmemis.")
		gap()
	}

	// Haftalık kontrol ritmi özetleri (Plan §7).
	add(true, 10, "BEKLEYEN HAKEDIS STATULERI")
	if len(sn.PendingPayments) == 0 {
		add(false, 9, "Bekleyen hakedis yok.")
	}
	for _, p := range sn.PendingPayments {
		add(false, 9, fmt.Sprintf("- %s / Donem %d: %s", p.Subcontractor, p.PeriodNo, statusTR(p.Status)))
	}
	gap()

	add(true, 10, "GOREV OZETI")
	add(false, 9, kv("Acik gorev", itoa(sn.OpenTasks)))
	add(false, 9, kv("Bu hafta terminli", itoa(sn.TasksDueWeek)))
	gap()

	add(true, 10, "MALZEME ONAYLARI (MAR)")
	add(false, 9, kv("Bekleyen MAR", itoa(sn.PendingMARs)))
	gap()

	if sn.OHSNote != "" {
		add(true, 10, "ISG")
		add(false, 8, sn.OHSNote)
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

// --- minimal PDF motoru (payments/pdf.go ile aynı desen) ---

type textLine struct {
	bold bool
	size float64
	text string
	gap  bool
}

func renderPDF(lines []textLine) []byte {
	const (
		pageW, pageH = 595.0, 842.0
		left         = 40.0
		top          = 800.0
		bottom       = 48.0
	)
	var pages [][]textLine
	var cur []textLine
	y := top
	for _, ln := range lines {
		lh := ln.size + 5
		if ln.gap {
			lh = 8
		}
		if y-lh < bottom {
			pages = append(pages, cur)
			cur = nil
			y = top
		}
		cur = append(cur, ln)
		y -= lh
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	if len(pages) == 0 {
		pages = [][]textLine{{}}
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
	for _, pl := range pages {
		content := buildContent(pl, left, top)
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

func buildContent(lines []textLine, left, top float64) string {
	var sb strings.Builder
	y := top
	for _, ln := range lines {
		if ln.gap {
			y -= 8
			continue
		}
		font := "F1"
		if ln.bold {
			font = "F2"
		}
		sb.WriteString(fmt.Sprintf("BT /%s %.0f Tf %.0f %.0f Td (%s) Tj ET\n",
			font, ln.size, left, y, escapePDF(ln.text)))
		y -= ln.size + 5
	}
	return sb.String()
}

func escapePDF(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)", "\r", "", "\n", "")
	return r.Replace(s)
}
