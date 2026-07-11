// Ceza tutanağı PDF'i — stdlib ile üretilir (Faz 3 hakediş özeti kararıyla
// tutarlı, ADR-0004): dış bağımlılık yok, çıktı deterministik, üretim
// milisaniyeler sürer → "60 sn içinde PDF" kabul kriteri istek içinde senkron
// karşılanır. Base-14 Helvetica Türkçe ş/ğ/İ taşımadığından metin ASCII'ye
// harf çevrilir. Zengin (HTML→PDF) şablon Faz 9 rapor motoruyla gelir.
package ohs

import (
	"bytes"
	"fmt"
	"strings"
)

// PenaltyPDFData — tutanak PDF'i için düzleştirilmiş görünüm modeli.
type PenaltyPDFData struct {
	PenaltyNo     string
	ProjectName   string
	ProjectCode   string
	Subcontractor string
	ViolationType string
	PenaltyLevel  string // Warning | Fine
	Amount        *float64
	Note          string
	IssuedBy      string
	IssuedAt      string
	HasEvidence   bool
}

// BuildPenaltyPDF — tek sayfalık tutanak PDF baytları döner.
func BuildPenaltyPDF(d PenaltyPDFData) []byte {
	var lines []textLine
	add := func(bold bool, size float64, s string) {
		lines = append(lines, textLine{bold: bold, size: size, text: asciiTR(s)})
	}
	gap := func() { lines = append(lines, textLine{gap: true}) }

	add(true, 15, "IS SAGLIGI VE GUVENLIGI CEZA TUTANAGI")
	add(true, 11, "Tutanak No: "+d.PenaltyNo)
	gap()
	add(false, 9, fmt.Sprintf("Proje: %s (%s)", d.ProjectName, d.ProjectCode))
	add(false, 9, "Taseron: "+d.Subcontractor)
	add(false, 9, "Duzenleyen: "+d.IssuedBy)
	add(false, 9, "Tarih: "+d.IssuedAt)
	gap()
	add(true, 10, "Ihlal Turu")
	add(false, 10, d.ViolationType)
	gap()
	level := "UYARI"
	if d.PenaltyLevel == "Fine" {
		level = "PARA CEZASI"
	}
	add(true, 10, "Yaptirim: "+level)
	if d.Amount != nil {
		add(true, 12, fmt.Sprintf("Ceza Tutari: %s TL", num2(*d.Amount)))
		add(false, 8, "Bu tutar, taseronun bir sonraki hakedisinde kesinti onerisi olarak")
		add(false, 8, "otomatik listelenir; proje yoneticisi onayiyla uygulanir.")
	}
	if d.Note != "" {
		gap()
		add(true, 10, "Aciklama")
		for _, ln := range wrapText(d.Note, 90) {
			add(false, 9, ln)
		}
	}
	gap()
	if d.HasEvidence {
		add(false, 8, "Kanit fotografi sistemde tutanaga bagli olarak saklanmaktadir.")
	}
	add(false, 8, "Bu tutanak IPKS uzerinden elektronik olarak uretilmistir; her degisiklik")
	add(false, 8, "denetim izinde (audit log) kayitlidir.")
	gap()
	gap()
	add(false, 9, "Duzenleyen (imza): ______________________")
	gap()
	add(false, 9, "Taseron Temsilcisi (tebellug): ______________________")

	return renderPDF(lines)
}

func num2(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	// binlik ayracı (basit, ASCII): 1234567.89 → 1.234.567,89
	parts := strings.SplitN(s, ".", 2)
	intPart, frac := parts[0], parts[1]
	var out []byte
	for i, c := range []byte(intPart) {
		if i > 0 && (len(intPart)-i)%3 == 0 && c != '-' {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	return string(out) + "," + frac
}

func wrapText(s string, w int) []string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, word := range words {
		if cur == "" {
			cur = word
		} else if len(cur)+1+len(word) <= w {
			cur += " " + word
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// asciiTR — Türkçe karakterleri ASCII karşılıklarına çevirir (WinAnsi Base-14
// kısıtı; Faz 3 hakediş PDF'iyle aynı yaklaşım).
func asciiTR(s string) string {
	repl := strings.NewReplacer(
		"ş", "s", "Ş", "S", "ğ", "g", "Ğ", "G", "ı", "i", "İ", "I",
		"ç", "c", "Ç", "C", "ö", "o", "Ö", "O", "ü", "u", "Ü", "U",
	)
	out := repl.Replace(s)
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

// --- minimal PDF motoru (payments/pdf.go ile aynı; paket-yerel kopya) -------

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
