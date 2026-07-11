// Aylık Yönetim Raporu PDF'i — reports/pdf.go ile aynı bilinçli karar
// (ADR-0004): stdlib ile deterministik, bağımlılıksız üretim. Base-14
// Helvetica WinAnsi Türkçe karakter taşımadığından metin ASCII'ye çevrilir.
//
// KURAL: PDF'e giren HER sayı MonthlySnapshot'tan gelir — başka kaynak yok
// (Plan Faz 9: rapor rakamları snapshot'tan doğrulanabilir).
package dashboard

import (
	"bytes"
	"fmt"
	"strings"
)

// BuildMonthlyPDF — snapshot'tan çok sayfalı PDF baytları üretir.
func BuildMonthlyPDF(sn MonthlySnapshot) []byte {
	var lines []textLine
	add := func(bold bool, size float64, s string) {
		lines = append(lines, textLine{bold: bold, size: size, text: asciiTR(s)})
	}
	gap := func() { lines = append(lines, textLine{gap: true}) }
	cur := " " + sn.Currency

	add(true, 15, "AYLIK YONETIM RAPORU")
	add(false, 9, fmt.Sprintf("Proje: %s (%s)", sn.ProjectName, sn.ProjectCode))
	add(false, 9, fmt.Sprintf("Donem: %04d-%02d  (%s - %s)", sn.Year, sn.Month, sn.PeriodStart, sn.PeriodEnd))
	add(false, 8, fmt.Sprintf("Uretim ani (snapshot): %s UTC", sn.GeneratedAt.Format("02.01.2006 15:04")))
	gap()

	// EVM (Plan §7: PV, EV, AC → SPI, CPI, EAC/ETC).
	add(true, 10, "EVM OZETI (kumulatif, donem sonu itibariyle)")
	add(false, 9, kv("BAC (proje butcesi)", num(sn.EVM.BAC, 2)+cur))
	add(false, 9, kv("PV (planlanan deger)", num(sn.EVM.PV, 2)+cur))
	add(false, 9, kv("EV (kazanilan deger)", num(sn.EVM.EV, 2)+cur))
	add(false, 9, kv("AC (gerceklesen maliyet)", num(sn.EVM.AC, 2)+cur))
	add(false, 9, kv("SPI (EV/PV)", idx(sn.EVM.SPI)))
	add(false, 9, kv("CPI (EV/AC)", idx(sn.EVM.CPI)))
	add(false, 9, kv("EAC (tamamlanma tahmini)", num(sn.EVM.EAC, 2)+cur))
	add(false, 9, kv("ETC (kalan maliyet)", num(sn.EVM.ETC, 2)+cur))
	add(false, 9, kv("Butce sapmasi (BAC-EAC)", num(sn.BudgetVariance, 2)+cur))
	add(false, 9, kv("Fiziki ilerleme", num(sn.EVM.ProgressPct, 1)+" %"))
	add(false, 8, "PV kaynagi: "+planSourceTR(sn.EVM.PlanSource))
	gap()

	// S-eğrisi tablosu.
	if len(sn.EVM.SCurve) > 0 {
		add(true, 10, "S-EGRISI (kumulatif)")
		add(true, 8, padRight("Ay", 10)+padLeft("PV", 16)+padLeft("EV", 16)+padLeft("AC", 16))
		for _, p := range sn.EVM.SCurve {
			add(false, 8, padRight(p.Month, 10)+
				padLeft(num(p.PV, 2), 16)+padLeft(num(p.EV, 2), 16)+padLeft(num(p.AC, 2), 16))
		}
		gap()
	}

	// Finansal: ay içinde kesinleşen hakedişler.
	add(true, 10, "AY ICINDE KESINLESEN HAKEDISLER")
	if len(sn.FinalizedPayments) == 0 {
		add(false, 9, "Bu ay kesinlesen hakedis yok.")
	} else {
		add(true, 8, padRight("Taseron", 34)+padLeft("Donem", 6)+
			padLeft("Brut", 14)+padLeft("Kesinti", 14)+padLeft("Net", 14))
		for _, p := range sn.FinalizedPayments {
			add(false, 8, trunc(p.Subcontractor, 34-1)+strings.Repeat(" ", 1)+
				padLeft(itoa(p.PeriodNo), 6)+padLeft(num(p.GrossThis, 2), 14)+
				padLeft(num(p.Deductions, 2), 14)+padLeft(num(p.NetPayable, 2), 14))
		}
		add(true, 8, padRight("TOPLAM", 40)+padLeft(num(sn.MonthGross, 2), 14)+
			padLeft(num(sn.MonthDeductions, 2), 14)+padLeft(num(sn.MonthNet, 2), 14))
	}
	gap()

	// Kesinti dökümü.
	if len(sn.DeductionsByType) > 0 {
		add(true, 10, "KESINTI DOKUMU (tip bazinda)")
		for _, t := range []string{"AdvanceOffset", "Retention", "Tax", "OHSPenalty", "Other"} {
			if v, ok := sn.DeductionsByType[t]; ok {
				add(false, 9, kv(deductionTR(t), num(v, 2)+cur))
			}
		}
		gap()
	}

	// Milestone gerçekleşme.
	add(true, 10, "MILESTONE GERCEKLESME")
	if len(sn.Milestones) == 0 {
		add(false, 9, "Tanimli milestone yok.")
	} else {
		for _, m := range sn.Milestones {
			flag := ""
			if m.Late {
				flag = "  [GECIKMIS]"
			}
			pd := m.PlannedDate
			if pd == "" {
				pd = "-"
			}
			ad := m.ActualDate
			if ad == "" {
				ad = "-"
			}
			add(false, 8, trunc(m.Name, 40)+"  plan: "+pd+"  gercek: "+ad+
				"  durum: "+m.Status+flag)
		}
	}
	gap()

	// İSG performansı.
	add(true, 10, "ISG PERFORMANSI")
	add(false, 9, kv("Ay icinde acilan bulgu", itoa(sn.OHS.FindingsOpened)))
	add(false, 9, kv("Ay icinde kapatilan bulgu", itoa(sn.OHS.FindingsClosed)))
	add(false, 9, kv("Acik bulgu (toplam)", itoa(sn.OHS.OpenTotal)))
	add(false, 9, kv("Acik KRITIK bulgu", itoa(sn.OHS.OpenCritical)))
	add(false, 9, kv("Kesilen ceza tutanagi", itoa(sn.OHS.PenaltiesCount)))
	add(false, 9, kv("Ceza toplami", num(sn.OHS.PenaltiesTotal, 2)+cur))
	gap()

	// Tedarik.
	add(true, 10, "TEDARIK OZETI")
	add(false, 9, kv("Ay icinde acilan siparis (PO)", itoa(sn.Procurement.POsOrdered)))
	add(false, 9, kv("Ay icinde teslim alinan PO", itoa(sn.Procurement.POsDelivered)))
	add(false, 9, kv("Geciken acik PO", itoa(sn.Procurement.POsOverdue)))
	add(false, 9, kv("Onay bekleyen PR", itoa(sn.Procurement.PRsPending)))

	return renderPDF(lines)
}

func idx(v float64) string {
	if v == 0 {
		return "-" // tanımsız (PV veya AC = 0)
	}
	return num(v, 3)
}

func planSourceTR(s string) string {
	switch s {
	case "manual":
		return "aylik dagilim girisi (manuel)"
	case "milestones":
		return "milestone agirliklari"
	default:
		return "dogrusal dagilim (varsayilan)"
	}
}

func deductionTR(t string) string {
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
		return "Diger kesintiler"
	}
}

// --- yardımcılar + minimal PDF motoru (reports/pdf.go ile aynı desen) ---

func kv(label, value string) string {
	return padRight(label, 40) + padLeft(value, 20)
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

func num(f float64, dec int) string { return fmt.Sprintf("%.*f", dec, f) }

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
		"ç", "c", "Ç", "C",
		"ğ", "g", "Ğ", "G",
		"ı", "i", "İ", "I",
		"ö", "o", "Ö", "O",
		"ş", "s", "Ş", "S",
		"ü", "u", "Ü", "U",
	)
	return r.Replace(s)
}

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
