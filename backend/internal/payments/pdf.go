// Hakediş özet PDF'i — stdlib ile üretilir. Bilinçli karar (ADR-0003 deseniyle
// tutarlı): gotenberg/chromedp bağımlılığı Faz 3'te devreye alınmaz; özet tek
// sayfalık metin PDF'i olarak elle kurulur (yeni go.mod bağımlılığı yok, çıktı
// tümüyle deterministik). Base-14 Helvetica WinAnsi Türkçe ş/ğ/İ karakterlerini
// taşımadığından etiketler ASCII'ye harf çevrilir; sayısal veriler zaten ASCII'dir.
// Zengin (HTML→PDF) şablon Faz 6/9 rapor motoruyla gelir.
package payments

import (
	"bytes"
	"fmt"
	"strings"
)

// PDFData — özet PDF'i için düzleştirilmiş görünüm modeli.
type PDFData struct {
	ProjectName    string
	ProjectCode    string
	Subcontractor  string
	ContractNo     string
	PeriodNo       int
	PeriodStart    string
	PeriodEnd      string
	Status         string
	Currency       string
	Lines          []CalcLine
	GrossCum       float64
	GrossPrev      float64
	GrossThis      float64
	Deductions     []DeductionLine
	TotalDeductions float64
	NetPayable     float64 // yükleniciye ödenecek nihai tutar
	VatPct         float64
	VatAmount      float64 // hesaplanan KDV (dönem brütü üzerinden)
	VatWithheld    float64 // tevkif edilen KDV (vergi dairesine)
	VatCollected   float64 // tahsil edilen KDV (ödemeye eklenen)
	PayableGross   float64 // brüt + tahsil edilen KDV
}

// BuildSummaryPDF — tek/çok sayfalı özet PDF baytları döner.
func BuildSummaryPDF(d PDFData) []byte {
	var lines []textLine
	add := func(bold bool, size float64, s string) {
		lines = append(lines, textLine{bold: bold, size: size, text: asciiTR(s)})
	}
	gap := func() { lines = append(lines, textLine{gap: true}) }

	cur := d.Currency
	if cur == "" {
		cur = "TRY"
	}

	add(true, 15, "HAKEDIS OZETI")
	add(false, 9, fmt.Sprintf("Proje: %s (%s)", d.ProjectName, d.ProjectCode))
	add(false, 9, fmt.Sprintf("Taseron: %s", d.Subcontractor))
	if d.ContractNo != "" {
		add(false, 9, fmt.Sprintf("Sozlesme No: %s", d.ContractNo))
	}
	period := fmt.Sprintf("Donem: %d", d.PeriodNo)
	if d.PeriodStart != "" || d.PeriodEnd != "" {
		period += fmt.Sprintf("  (%s - %s)", d.PeriodStart, d.PeriodEnd)
	}
	add(false, 9, period)
	add(false, 9, fmt.Sprintf("Durum: %s", d.Status))
	gap()

	// Kalem tablosu
	add(true, 10, "IMALAT KALEMLERI")
	add(true, 8, padRow("Poz", "Aciklama", "Birim", "Kum.Mik.", "B.Fiyat", "Kum.Tutar", "Bu Donem"))
	for _, l := range d.Lines {
		add(false, 8, padRow(
			l.PozNo, trunc(l.Description, 22), l.Unit,
			num(l.CumQty, 3), num(l.UnitPrice, 2), num(l.CumAmount, 2), num(l.ThisAmount, 2),
		))
	}
	gap()

	// Ozet (Plan §6.4 A-I)
	add(true, 10, "OZET")
	add(false, 9, kv("A) Kumulatif imalat tutari", num(d.GrossCum, 2), cur))
	add(false, 9, kv("B) Onceki donem kumulatifi", num(d.GrossPrev, 2), cur))
	add(false, 9, kv("C) Bu donem brut hakedis", num(d.GrossThis, 2), cur))
	for _, ded := range d.Deductions {
		label := "   - " + asciiTR(dedLabel(ded.Type))
		if ded.Description != "" {
			label += " (" + trunc(ded.Description, 24) + ")"
		}
		add(false, 9, kv(label, num(ded.Amount, 2), cur))
	}
	add(false, 9, kv("   Toplam kesinti", num(d.TotalDeductions, 2), cur))
	add(true, 10, kv("I) Net odenecek (KDV haric)", num(d.NetPayable, 2), cur))
	add(false, 9, kv(fmt.Sprintf("   KDV (%%%s)", trimPct(d.VatPct)), num(d.VatAmount, 2), cur))
	add(false, 10, kv("Tevkif edilen KDV", num(d.VatWithheld, 2), cur))
	add(false, 10, kv("Tahsil edilen KDV", num(d.VatCollected, 2), cur))
	add(false, 10, kv("Odenebilir toplam", num(d.PayableGross, 2), cur))
	add(true, 11, kv("YUKLENICIYE ODENECEK", num(d.NetPayable, 2), cur))

	return renderPDF(lines)
}

func dedLabel(t string) string {
	switch t {
	case "AdvanceOffset":
		return "Avans mahsubu"
	case "Retention":
		return "Teminat kesintisi"
	case "Tax":
		return "Vergi/stopaj"
	case "OHSPenalty":
		return "ISG ceza kesintisi"
	default:
		return "Diger"
	}
}

// --- düzen yardımcıları ---

func padRow(cols ...string) string {
	widths := []int{10, 24, 6, 12, 11, 14, 14}
	var sb strings.Builder
	for i, c := range cols {
		w := 10
		if i < len(widths) {
			w = widths[i]
		}
		sb.WriteString(padRight(c, w))
	}
	return strings.TrimRight(sb.String(), " ")
}

func kv(label, value, cur string) string {
	return padRight(label, 40) + padLeft(value+" "+cur, 20)
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

func num(f float64, dec int) string {
	return fmt.Sprintf("%.*f", dec, f)
}
func trimPct(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
}

// asciiTR — Türkçe karakterleri PDF-güvenli ASCII'ye çevirir.
func asciiTR(s string) string {
	r := strings.NewReplacer(
		"ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G", "ı", "i", "İ", "I",
		"ç", "c", "Ç", "C", "ö", "o", "Ö", "O", "ü", "u", "Ü", "U",
	)
	out := r.Replace(s)
	// Kalan ASCII-dışı baytları '?' yap (WinAnsi güvenliği).
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

// --- minimal PDF motoru ---

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
	// Sayfalara böl.
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

	// Nesne düzeni: 1=Catalog, 2=Pages, 3=F1(Helvetica), 4=F2(Bold),
	// sonra her sayfa için Page + Content nesneleri.
	type obj struct{ body string }
	var objs []obj
	objs = append(objs, obj{}) // 0 kullanılmaz (1-tabanlı)

	catalog := len(objs)
	objs = append(objs, obj{"<< /Type /Catalog /Pages 2 0 R >>"})
	pagesObj := len(objs)
	objs = append(objs, obj{}) // 2: sonra doldurulur
	f1 := len(objs)
	objs = append(objs, obj{"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"})
	f2 := len(objs)
	objs = append(objs, obj{"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>"})

	var kids []string
	for _, pl := range pages {
		content := buildContent(pl, left, top, pageW, pageH)
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
	_ = catalog

	// Seri hale getir + xref.
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

func buildContent(lines []textLine, left, top, _, _ float64) string {
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
