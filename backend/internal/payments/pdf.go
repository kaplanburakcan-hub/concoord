// Hakediş özet PDF'i — stdlib ile üretilir. Bilinçli karar (ADR-0003 deseniyle
// tutarlı): gotenberg/chromedp ya da PDF kütüphanesi eklenmez; PDF elle kurulur
// (yeni go.mod bağımlılığı yok, çıktı tümüyle deterministik).
//
// Yazı tipi gömülmez: PDF'in standart Helvetica / Helvetica-Bold yazı tipleri
// kullanılır. Türkçe karakterler için kodlama /Differences ile genişletilir:
// WinAnsi'de olmayan Ğ ğ Ş ş İ ı, boş kontrol kodlarına (1..6) Helvetica'nın
// kendi glif adlarıyla (Gbreve, gbreve, Scedilla, scedilla, Idotaccent,
// dotlessi) bağlanır; Ç ç Ö ö Ü ü zaten WinAnsi'de vardır. Böylece etiketler
// harf çevrilmeden (ş→s) olduğu gibi basılır.
//
// Düzen: A4, "resmi rapor" şablonu — başlık + künye ızgarası, imalat kalemleri
// tablosu (çok sayfaya taşar, başlık satırı tekrarlanır), kesintiler ile
// A–E hesap zinciri yan yana, ödenecek tutar kutusu ve imza kutuları.
package payments

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

// PDFData — özet PDF'i için düzleştirilmiş görünüm modeli.
type PDFData struct {
	ProjectName     string
	ProjectCode     string
	Subcontractor   string
	ContractNo      string
	ContractAmount  float64 // taşeronun bu projedeki toplam sözleşme bedeli (0 = bilinmiyor)
	PeriodNo        int
	PeriodStart     string // YYYY-MM-DD
	PeriodEnd       string
	Status          string
	Currency        string
	Lines           []CalcLine
	GrossCum        float64
	GrossPrev       float64
	GrossThis       float64
	Deductions      []DeductionLine
	TotalDeductions float64
	NetPayable      float64 // yükleniciye ödenecek nihai tutar
	VatPct          float64
	VatAmount       float64 // hesaplanan KDV (dönem brütü üzerinden)
	VatWithheld     float64 // tevkif edilen KDV (vergi dairesine)
	VatCollected    float64 // tahsil edilen KDV (ödemeye eklenen)
	PayableGross    float64 // brüt + tahsil edilen KDV
	GeneratedAt     time.Time
}

type rgb struct{ r, g, b float64 }

func hexc(h uint32) rgb {
	return rgb{float64(h>>16&255) / 255, float64(h>>8&255) / 255, float64(h&255) / 255}
}

var (
	colNavy  = hexc(0x0b2545)
	colAmber = hexc(0xe0b400)
	colInk   = hexc(0x14202e)
	colDim   = hexc(0x5b6877)
	colRule  = hexc(0xd5dce5)
	colWash  = hexc(0xf3f6fa)
	colWhite = hexc(0xffffff)
)

const (
	pgW, pgH   = 595.0, 842.0
	mL, mR     = 30.0, 30.0
	contentW   = pgW - mL - mR
	pgBottom   = pgH - 46 // içerik alt sınırı (altta alt bilgi var)
	rowH       = 15.0
	contHeadH  = 34.0
	firstTopY  = 30.0
	contStartY = 52.0
)

// BuildSummaryPDF — çok sayfalı özet PDF baytları döner.
func BuildSummaryPDF(d PDFData) []byte {
	if d.GeneratedAt.IsZero() {
		d.GeneratedAt = time.Now()
	}
	cur := d.Currency
	if cur == "" {
		cur = "TRY"
	}
	docNo := fmt.Sprintf("HK-%s-%04d", d.ProjectCode, d.PeriodNo)
	p := newPdfDoc()
	footer := fmt.Sprintf("%s · %s · Dönem %d", docNo, d.Subcontractor, d.PeriodNo)

	// ── Sayfa 1 başlığı ────────────────────────────────────────────────────
	p.newPage()
	y := firstTopY
	p.brand(mL, y+10)
	p.text(mL, y+34, "HAKEDİŞ ÖZETİ", true, 17, colNavy)

	// Sağ üst künye: belge no / düzenleme / durum
	metaX := pgW - mR
	meta := [][2]string{{"Belge no", docNo}, {"Düzenleme", d.GeneratedAt.Format("02.01.2006")}}
	my := y + 9.0
	for _, m := range meta {
		p.textR(metaX, my, m[1], true, 8, colInk)
		p.textR(metaX-p.width(m[1], true, 8)-8, my, m[0], false, 8, colDim)
		my += 11
	}
	label, fg, bg := statusStyle(d.Status)
	cw := p.width(trUpper(label), true, 7) + 14
	p.fillRect(metaX-cw, my-7.5, cw, 11.5, bg)
	p.textR(metaX-7, my+0.5, trUpper(label), true, 7, fg)
	y += 46
	p.fillRect(mL, y, contentW, 2.25, colNavy)
	y += 12

	// Künye ızgarası (3 satır × 2 sütun)
	period := fmt.Sprintf("No %d", d.PeriodNo)
	if d.PeriodStart != "" || d.PeriodEnd != "" {
		period += fmt.Sprintf(" · %s – %s", trDate(d.PeriodStart), trDate(d.PeriodEnd))
	}
	contractNo := d.ContractNo
	if contractNo == "" {
		contractNo = "—"
	}
	sozBedel, ilerleme := "—", "—"
	if d.ContractAmount > 0 {
		sozBedel = trNum(d.ContractAmount, 2) + " " + cur
		ilerleme = "%" + trNum(d.GrossCum/d.ContractAmount*100, 1)
	}
	cells := [][2]string{
		{"Proje", fmt.Sprintf("%s (%s)", d.ProjectName, d.ProjectCode)}, {"Taşeron", d.Subcontractor},
		{"Sözleşme no", contractNo}, {"Hakediş dönemi", period},
		{"Sözleşme bedeli", sozBedel}, {"Kümülatif ilerleme", ilerleme},
	}
	const gridRowH = 18.0
	colW := contentW / 2
	p.strokeRect(mL, y, contentW, gridRowH*3, colRule, 0.75)
	for i, c := range cells {
		cx := mL + float64(i%2)*colW
		cy := y + float64(i/2)*gridRowH
		if i%2 == 1 {
			p.vline(cx, cy, cy+gridRowH, colRule, 0.75)
		}
		if i/2 < 2 {
			p.hline(cx, cx+colW, cy+gridRowH, colRule, 0.75)
		}
		p.text(cx+7, cy+12, c[0], false, 7.5, colDim)
		p.text(cx+78, cy+12, p.fit(c[1], true, 8, colW-84), true, 8, colInk)
	}
	y += gridRowH*3 + 16

	// ── İmalat kalemleri ──────────────────────────────────────────────────
	p.text(mL, y+7, "İMALAT KALEMLERİ", true, 7.5, colNavy)
	y += 12
	cols := []tblCol{
		{"POZ", 44, false}, {"AÇIKLAMA", 150, false}, {"BİRİM", 32, false},
		{"B. FİYAT", 64, true}, {"ÖNCEKİ KÜM.", 60, true}, {"BU DÖNEM", 60, true},
		{"KÜMÜLATİF", 60, true}, {"BU DÖNEM TUTAR", 65, true},
	}
	y = p.tableHeader(y, cols)
	if len(d.Lines) == 0 {
		p.text(mL+7, y+11, "Bu hakedişte imalat kalemi yok.", false, 8, colDim)
		y += rowH
	}
	for i, l := range d.Lines {
		if y+rowH+rowH > pgBottom { // bir sonraki satır + toplam satırı sığmıyorsa
			p.newPage()
			y = p.contHeader(footer, d)
			y = p.tableHeader(y, cols)
		}
		if i%2 == 1 {
			p.fillRect(mL, y, contentW, rowH, colWash)
		}
		p.tableRow(y, cols, []string{
			l.PozNo, l.Description, l.Unit, trNum(l.UnitPrice, 2),
			trQty(l.PrevCumQty), trQty(l.ThisPeriodQty), trQty(l.CumQty), trNum(l.ThisAmount, 2),
		}, false)
		p.hline(mL, mL+contentW, y+rowH, colRule, 0.5)
		y += rowH
	}
	p.fillRect(mL, y, contentW, rowH+1, colWash)
	p.fillRect(mL, y, contentW, 1.5, colNavy)
	p.text(mL+7, y+11.5, "Bu dönem brüt hakediş", true, 8, colInk)
	p.textR(mL+contentW-7, y+11.5, trNum(d.GrossThis, 2), true, 8, colInk)
	y += rowH + 1 + 16

	// ── Kesintiler (sol) + hesap özeti (sağ) ───────────────────────────────
	nDed := len(d.Deductions)
	if nDed == 0 {
		nDed = 1
	}
	sumRows := 5
	if d.VatWithheld > 0 {
		sumRows = 6
	}
	blockH := 12 + math.Max(float64(nDed+1), float64(sumRows))*rowH + 6 + 38
	if y+blockH > pgBottom {
		p.newPage()
		y = p.contHeader(footer, d)
	}
	halfW := (contentW - 16) / 2
	leftX, rightX := mL, mL+halfW+16

	p.text(leftX, y+7, "KESİNTİLER", true, 7.5, colNavy)
	p.text(rightX, y+7, "HESAP ÖZETİ", true, 7.5, colNavy)
	ly, ry := y+12, y+12

	if len(d.Deductions) == 0 {
		p.text(leftX+4, ly+11, "Kesinti yok.", false, 8, colDim)
		p.hline(leftX, leftX+halfW, ly+rowH, colRule, 0.5)
		ly += rowH
	}
	for _, ded := range d.Deductions {
		lbl := dedLabel(ded.Type)
		// Eski kayıtlarda açıklama ASCII ("yıllara sari") olabilir: karşılaştırmayı aksansız yap.
		fl, fd := foldTR(lbl), foldTR(ded.Description)
		switch {
		case fd == "" || fd == fl:
		case strings.HasPrefix(fd, fl):
			// Açıklama etiketle başlıyor: etiketi tekrar yazma, yalnızca artanı ekle.
			if rest := strings.TrimSpace(strings.TrimLeft(string([]rune(ded.Description)[len([]rune(lbl)):]), " ·—-")); rest != "" {
				lbl += " " + rest
			}
		default:
			lbl += " · " + ded.Description
		}
		if ded.RatePct != nil && *ded.RatePct > 0 && !strings.Contains(lbl, "%") {
			lbl += fmt.Sprintf(" (%%%s)", trimPct(*ded.RatePct))
		}
		amt := trNum(ded.Amount, 2)
		p.text(leftX+4, ly+11, p.fit(lbl, false, 8, halfW-12-p.width(amt, false, 8)-8), false, 8, colInk)
		p.textR(leftX+halfW-4, ly+11, amt, false, 8, colInk)
		p.hline(leftX, leftX+halfW, ly+rowH, colRule, 0.5)
		ly += rowH
	}
	p.fillRect(leftX, ly, halfW, rowH, colWash)
	p.text(leftX+4, ly+11, "Toplam kesinti", true, 8, colInk)
	p.textR(leftX+halfW-4, ly+11, trNum(d.TotalDeductions, 2), true, 8, colInk)
	ly += rowH

	type sumRow struct {
		k, label, val string
		tot           bool
	}
	rows := []sumRow{
		{"A", "Kümülatif imalat tutarı", trNum(d.GrossCum, 2), false},
		{"B", "Önceki dönem kümülatifi", trNum(d.GrossPrev, 2), false},
		{"C", "Bu dönem brüt (A − B)", trNum(d.GrossThis, 2), true},
		{"D", fmt.Sprintf("KDV (%%%s)", trimPct(d.VatPct)), trNum(d.VatAmount, 2), false},
	}
	if d.VatWithheld > 0 {
		rows = append(rows, sumRow{"", "Tevkif edilen KDV (ödemeye eklenmez)", "− " + trNum(d.VatWithheld, 2), false})
	}
	rows = append(rows, sumRow{"E", "Toplam kesinti", "− " + trNum(d.TotalDeductions, 2), false})
	for _, r := range rows {
		if r.tot {
			p.fillRect(rightX, ry, halfW, rowH, colWash)
		}
		p.text(rightX+4, ry+11, r.k, true, 8, colNavy)
		p.text(rightX+20, ry+11, r.label, r.tot, 8, colInk)
		p.textR(rightX+halfW-4, ry+11, r.val, r.tot, 8, colInk)
		p.hline(rightX, rightX+halfW, ry+rowH, colRule, 0.5)
		ry += rowH
	}
	ry += 6
	p.fillRect(rightX, ry, halfW, 36, colNavy)
	formula := "C + D − E"
	if d.VatWithheld > 0 {
		formula = "C + tahsil edilen KDV − E"
	}
	p.text(rightX+9, ry+13, trUpper("Yükleniciye ödenecek ("+formula+")"), true, 6.5, hexc(0xc9d4e3))
	p.textR(rightX+halfW-9, ry+29, trNum(d.NetPayable, 2)+" "+cur, true, 13, colWhite)
	ry += 36

	y = math.Max(ly, ry) + 16

	// ── İmza kutuları ─────────────────────────────────────────────────────
	if y+54 > pgBottom {
		p.newPage()
		y = p.contHeader(footer, d)
	}
	sig := []string{"Hazırlayan · Teknik Ofis", "Kontrol · Şantiye Müdürü", "Muhasebe Kontrol", "Onay · Genel Müdür"}
	gap := 8.0
	sw := (contentW - gap*3) / 4
	for i, s := range sig {
		sx := mL + float64(i)*(sw+gap)
		p.strokeRect(sx, y, sw, 50, colRule, 0.75)
		p.text(sx+6, y+12, s, true, 7, colInk)
		p.text(sx+6, y+44, "Ad soyad · imza · tarih", false, 6.5, colDim)
	}

	return p.finish(footer)
}

// statusStyle — durum etiketi ve rozet renkleri.
func statusStyle(s string) (label string, fg, bg rgb) {
	switch s {
	case "Draft":
		return "Taslak", hexc(0x5b6877), hexc(0xe6eaf0)
	case "Submitted":
		return "Gönderildi", hexc(0x1d3d68), hexc(0xdfe8f5)
	case "SiteApproved":
		return "Saha Onaylı", hexc(0xb26a00), hexc(0xfdeed2)
	case "Finalized":
		return "Kesinleşti", hexc(0x1f7a4d), hexc(0xe1f3ea)
	case "Rejected":
		return "Reddedildi", hexc(0x9b2c22), hexc(0xf8e0dd)
	}
	return s, hexc(0x5b6877), hexc(0xe6eaf0)
}

// foldTR — Türkçe harfleri ASCII karşılıklarına indirip küçültür (yalnızca karşılaştırma için).
func foldTR(s string) string {
	return strings.ToLower(strings.NewReplacer(
		"ı", "i", "İ", "i", "I", "i", "ş", "s", "Ş", "s", "ğ", "g", "Ğ", "g",
		"ü", "u", "Ü", "u", "ö", "o", "Ö", "o", "ç", "c", "Ç", "c",
	).Replace(s))
}

func dedLabel(t string) string {
	switch t {
	case "AdvanceOffset":
		return "Avans mahsubu"
	case "Retention":
		return "Teminat kesintisi"
	case "Withholding":
		return "Stopaj (yıllara sarı)"
	case "Tax":
		return "Vergi / stopaj"
	case "OHSPenalty":
		return "İSG ceza kesintisi"
	default:
		return "Diğer kesinti"
	}
}

// ── sayı / tarih biçimi (Türkçe: 1.234,56) ────────────────────────────────

func trNum(v float64, dec int) string {
	neg := v < 0
	s := fmt.Sprintf("%.*f", dec, math.Abs(v))
	ip, fp := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		ip, fp = s[:i], s[i+1:]
	}
	var b strings.Builder
	for i, ch := range ip {
		if i > 0 && (len(ip)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(ch)
	}
	out := b.String()
	if fp != "" {
		out += "," + fp
	}
	if neg && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

// trQty — miktar: en çok 3 ondalık, sondaki sıfırlar atılır.
func trQty(v float64) string {
	s := trNum(v, 3)
	if strings.Contains(s, ",") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ",")
	}
	return s
}

func trimPct(f float64) string {
	return strings.ReplaceAll(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), "."), ".", ",")
}

func trDate(iso string) string {
	if t, err := time.Parse("2006-01-02", iso); err == nil {
		return t.Format("02.01.2006")
	}
	return iso
}

// ── tablo yardımcıları ────────────────────────────────────────────────────

type tblCol struct {
	title string
	w     float64
	right bool
}

func (p *pdfDoc) tableHeader(y float64, cols []tblCol) float64 {
	p.fillRect(mL, y, contentW, 17, colNavy)
	x := mL
	for _, c := range cols {
		if c.right {
			p.textR(x+c.w-6, y+11.5, c.title, true, 6.3, colWhite)
		} else {
			p.text(x+6, y+11.5, c.title, true, 6.3, colWhite)
		}
		x += c.w
	}
	return y + 17
}

func (p *pdfDoc) tableRow(y float64, cols []tblCol, cells []string, bold bool) {
	x := mL
	for i, c := range cols {
		s := p.fit(cells[i], bold, 8, c.w-12)
		if c.right {
			p.textR(x+c.w-6, y+10.5, s, bold, 8, colInk)
		} else {
			p.text(x+6, y+10.5, s, bold, 8, colInk)
		}
		x += c.w
	}
}

// brand — "CONCOORD" yazısı (C harfi vurgu renginde).
func (p *pdfDoc) brand(x, y float64) {
	parts := []struct {
		s string
		c rgb
	}{{"CON", colNavy}, {"C", colAmber}, {"OORD", colNavy}}
	for _, pt := range parts {
		p.text(x, y, pt.s, true, 10, pt.c)
		x += p.width(pt.s, true, 10)
	}
}

// contHeader — devam sayfaları için küçük üst bilgi; içerik başlangıç y'sini döner.
func (p *pdfDoc) contHeader(footer string, d PDFData) float64 {
	p.brand(mL, 40)
	p.textR(pgW-mR, 40, fmt.Sprintf("Hakediş %d · %s", d.PeriodNo, d.Subcontractor), true, 8, colDim)
	p.fillRect(mL, 46, contentW, 1.5, colNavy)
	return contStartY + 8
}

// ── minimal PDF motoru ────────────────────────────────────────────────────

type pdfDoc struct {
	pages        []*bytes.Buffer
	cur          *bytes.Buffer
	reg, boldFnt *ttfFont
}

func newPdfDoc() *pdfDoc {
	r, b := newFontPair()
	return &pdfDoc{reg: r, boldFnt: b}
}

func (p *pdfDoc) font(bold bool) *ttfFont {
	if bold {
		return p.boldFnt
	}
	return p.reg
}

func (p *pdfDoc) newPage() {
	p.cur = &bytes.Buffer{}
	p.pages = append(p.pages, p.cur)
}

func (p *pdfDoc) fillRect(x, y, w, h float64, c rgb) {
	fmt.Fprintf(p.cur, "%.3f %.3f %.3f rg %.2f %.2f %.2f %.2f re f\n", c.r, c.g, c.b, x, pgH-y-h, w, h)
}

func (p *pdfDoc) strokeRect(x, y, w, h float64, c rgb, lw float64) {
	fmt.Fprintf(p.cur, "%.3f %.3f %.3f RG %.2f w %.2f %.2f %.2f %.2f re S\n", c.r, c.g, c.b, lw, x, pgH-y-h, w, h)
}

func (p *pdfDoc) hline(x1, x2, y float64, c rgb, lw float64) {
	fmt.Fprintf(p.cur, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n", c.r, c.g, c.b, lw, x1, pgH-y, x2, pgH-y)
}

func (p *pdfDoc) vline(x, y1, y2 float64, c rgb, lw float64) {
	fmt.Fprintf(p.cur, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n", c.r, c.g, c.b, lw, x, pgH-y1, x, pgH-y2)
}

// text — y üstten ölçülen taban çizgisidir.
func (p *pdfDoc) text(x, y float64, s string, bold bool, size float64, c rgb) {
	font := "F1"
	if bold {
		font = "F2"
	}
	fmt.Fprintf(p.cur, "BT /%s %.2f Tf %.3f %.3f %.3f rg %.2f %.2f Td %s Tj ET\n",
		font, size, c.r, c.g, c.b, x, pgH-y, p.font(bold).hex(s))
}

func (p *pdfDoc) textR(xRight, y float64, s string, bold bool, size float64, c rgb) {
	p.text(xRight-p.width(s, bold, size), y, s, bold, size, c)
}

func (p *pdfDoc) width(s string, bold bool, size float64) float64 {
	return p.font(bold).width(s, size)
}

// fit — metni verilen genişliğe sığacak şekilde sonuna "…" ekleyerek kısaltır.
func (p *pdfDoc) fit(s string, bold bool, size, maxW float64) string {
	if p.width(s, bold, size) <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 1 {
		rs = rs[:len(rs)-1]
		t := strings.TrimRight(string(rs), " ") + "…"
		if p.width(t, bold, size) <= maxW {
			return t
		}
	}
	return "…"
}

func (p *pdfDoc) finish(footer string) []byte {
	n := len(p.pages)
	for i, pg := range p.pages {
		p.cur = pg
		p.hline(mL, pgW-mR, pgH-34, colRule, 0.75)
		p.text(mL, pgH-23, footer, false, 6.8, colDim)
		p.textR(pgW-mR, pgH-23, fmt.Sprintf("Sayfa %d / %d", i+1, n), false, 6.8, colDim)
	}

	// Nesne düzeni: 1=Catalog, 2=Pages, 3..7=Regular yazı tipi, 8..12=Bold, sonra sayfalar.
	var objs []string
	objs = append(objs, "") // 0 kullanılmaz
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>")
	objs = append(objs, "") // 2: Pages, sonra doldurulur
	objs = append(objs, p.reg.objects(3, "LiberationSans")...)
	objs = append(objs, p.boldFnt.objects(8, "LiberationSans-Bold")...)
	var kids []string
	for _, pg := range p.pages {
		content := pg.String()
		contentObj := len(objs)
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
		pageObj := len(objs)
		objs = append(objs, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 8 0 R >> >> /Contents %d 0 R >>",
			pgW, pgH, contentObj))
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObj))
	}
	objs[2] = fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(kids), strings.Join(kids, " "))

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objs))
	for i := 1; i < len(objs); i++ {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i, objs[i])
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs))
	for i := 1; i < len(objs); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objs), xref)
	return buf.Bytes()
}

// trUpper — Türkçe büyük harf kuralı (i → İ, ı → I).
func trUpper(s string) string { return strings.Map(unicode.TurkishCase.ToUpper, s) }
