// Package reportpdf — ConCoord günlük, haftalık, aylık ve çeyrek yıl rapor
// PDF'leri. Düzenler onaylanan "Rapor Taslakları" tasarımından türetilmiştir;
// çizim motoru pdfkit'tir. Bu dosya sayfa akışı ve ortak bileşenleri içerir.
//
// KURAL: PDF'e giren her değer çağıranın verdiği veri yapılarından gelir —
// burada hesaplanmış, uydurulmuş sayı yoktur.
package reportpdf

import (
	"fmt"
	"math"
	"strings"
	"time"

	k "github.com/ipks/ipks/backend/internal/pdfkit"
)

const (
	mL       = 30.0
	mR       = 30.0
	cW       = k.PgW - mL - mR // içerik genişliği
	footH    = 19.5
	bottom   = k.PgH - footH - 12 // içerik alt sınırı
	bandH    = 78.0
	slimH    = 48.0
	rowH     = 14.0
	bodySize = 7.4
)

// page — sayfa akışı durumu.
type page struct {
	d        *k.Doc
	y        float64
	kind     string // sağ üst tür etiketi ("GÜNLÜK SAHA RAPORU")
	ctx      string // devam sayfası üst bilgisi ve alt bilgi
	footer   string
	slimSub  string
	numbered bool
}

func newPage(kind, footer, slimSub string) *page {
	return &page{d: k.New(), kind: kind, footer: footer, slimSub: slimSub}
}

// brandOn — koyu zeminde "CONCOORD" (beyaz + amber C), hakediş PDF'iyle aynı.
func brandOn(d *k.Doc, x, y, size float64) {
	parts := []struct {
		s string
		c k.RGB
	}{{"CON", k.White}, {"C", k.Amber}, {"OORD", k.White}}
	for _, pt := range parts {
		d.Text(x, y, pt.s, true, size, pt.c)
		x += d.Width(pt.s, true, size)
	}
}

// bigHeader — ilk sayfa üst bandı. rightTop: küçük amber etiket, rightMain:
// büyük başlık, rightSub: ince alt satır; leftSub: proje satırı.
func (p *page) bigHeader(rightTop, rightMain string, mainSize float64, rightSub, leftSub string) {
	d := p.d
	d.NewPage()
	d.FillRect(0, 0, k.PgW, bandH, k.Navy)
	d.FillRect(0, bandH, k.PgW, 3, k.Amber)
	brandOn(d, mL, 31, 15.8)
	d.Text(mL, 40, "İNŞAAT PROJE YÖNETİM PLATFORMU", false, 5.6, k.OnNavy2)
	d.Text(mL, 63, d.Fit(leftSub, false, 6.8, 270), false, 6.8, k.OnNavy)
	d.TextR(k.PgW-mR, 25, rightTop, true, 6.8, k.Amber)
	d.TextR(k.PgW-mR, 25+mainSize*0.95, rightMain, true, mainSize, k.White)
	d.TextR(k.PgW-mR, 25+mainSize*0.95+13, rightSub, false, 7.5, k.OnNavy)
	p.y = bandH + 3 + 11
}

// slimHeader — devam sayfaları için ince üst bant.
func (p *page) slimHeader() {
	d := p.d
	d.NewPage()
	d.FillRect(0, 0, k.PgW, slimH, k.Navy)
	d.FillRect(0, slimH, k.PgW, 2.5, k.Amber)
	brandOn(d, mL, 27, 12.6)
	d.Text(mL, 35, "İNŞAAT PROJE YÖNETİM PLATFORMU", false, 4.8, k.OnNavy2)
	d.TextR(k.PgW-mR, 22, p.kind+" · EK", true, 6.6, k.Amber)
	d.TextR(k.PgW-mR, 34, p.slimSub, false, 8, k.OnNavy)
	p.y = slimH + 2.5 + 11
}

// need — h puanlık yer yoksa devam sayfası açar.
func (p *page) need(h float64) {
	if p.y+h > bottom {
		p.slimHeader()
	}
}

// finish — her sayfaya alt bant ve sayfa numarası basar, PDF baytlarını döner.
func (p *page) finish(generated time.Time) []byte {
	d := p.d
	n := d.PageCount()
	for i := 0; i < n; i++ {
		d.SetPage(i)
		d.FillRect(0, k.PgH-footH, k.PgW, footH, k.Navy)
		d.Text(mL, k.PgH-7.2, p.footer, false, 6, k.OnNavy2)
		d.TextR(k.PgW-mR, k.PgH-7.2,
			fmt.Sprintf("Üretim: %s · Sayfa %d / %d", generated.Format("02.01.2006 15:04"), i+1, n),
			false, 6, k.OnNavy2)
	}
	return d.Finish()
}

// ---------------------------------------------------------------- bileşenler

// section — bölüm başlığı (küçük, kalın, büyük harf, lacivert).
func (p *page) section(title, note string) {
	p.need(26)
	p.d.Text(mL, p.y+7, k.TrUpper(title), true, 7.2, k.Navy)
	if note != "" {
		p.d.TextR(k.PgW-mR, p.y+7, note, false, 6.4, k.Dim)
	}
	p.y += 12
}

// col — tablo sütunu.
type col struct {
	Title string
	W     float64
	Right bool
}

// colsFill — genişliklerin toplamını total'a esnetir (son hariç sabit, "-1" = kalan).
func fillCols(cols []col, total float64) []col {
	used, flex := 0.0, -1
	for i, c := range cols {
		if c.W < 0 {
			flex = i
		} else {
			used += c.W
		}
	}
	out := append([]col(nil), cols...)
	if flex >= 0 {
		out[flex].W = total - used
	}
	return out
}

func (p *page) tableHead(x0 float64, cols []col) {
	d := p.d
	x := x0
	for _, c := range cols {
		if c.Right {
			d.TextR(x+c.W-4, p.y+8, k.TrUpper(c.Title), true, 5.9, k.Dim)
		} else {
			d.Text(x+4, p.y+8, k.TrUpper(c.Title), true, 5.9, k.Dim)
		}
		x += c.W
	}
	w := 0.0
	for _, c := range cols {
		w += c.W
	}
	d.HLine(x0, x0+w, p.y+11.5, k.Navy, 1.1)
	p.y += 12.5
}

// table — tam genişlikte tablo; satırlar sayfaya sığmazsa devam sayfasına geçer.
func (p *page) table(cols []col, rows [][]string, boldFirst bool) {
	p.tableAt(mL, cols, rows, boldFirst)
}

func (p *page) tableAt(x0 float64, cols []col, rows [][]string, boldFirst bool) {
	d := p.d
	p.need(12.5 + rowH*2)
	p.tableHead(x0, cols)
	total := 0.0
	for _, c := range cols {
		total += c.W
	}
	for _, r := range rows {
		if p.y+rowH > bottom {
			p.slimHeader()
			p.tableHead(x0, cols)
		}
		x := x0
		for i, c := range cols {
			s := ""
			if i < len(r) {
				s = r[i]
			}
			bold := boldFirst && i == 0
			s = d.Fit(s, bold, bodySize, c.W-8)
			if c.Right {
				d.TextR(x+c.W-4, p.y+9.4, s, bold, bodySize, k.Ink)
			} else {
				d.Text(x+4, p.y+9.4, s, bold, bodySize, k.Ink)
			}
			x += c.W
		}
		d.HLine(x0, x0+total, p.y+rowH, k.Rule, 0.5)
		p.y += rowH
	}
}

// kpi — KPI şeridi hücresi.
type kpi struct {
	Label, Value, Sub string
	Accent            bool // amber hücre (genellikle sonuncu)
	Warn              bool // değer sarı
}

// kpiStrip — tam genişlikte KPI şeridi.
func (p *page) kpiStrip(cells []kpi, h float64) {
	p.need(h + 6)
	p.kpiStripAt(mL, cW, cells, h)
	p.y += h + 10
}

// kpiStripAt — lacivert-mavi şerit; Accent hücre amber dolgulu. y'yi ilerletmez.
func (p *page) kpiStripAt(x0, w float64, cells []kpi, h float64) {
	d := p.d
	d.FillRect(x0, p.y, w, h, k.Mid)
	cw := w / float64(len(cells))
	for i, c := range cells {
		x := x0 + float64(i)*cw
		fg, dim := k.White, k.Hex(0xb9c9e0)
		if c.Accent {
			d.FillRect(x, p.y, cw, h, k.Amber)
			fg, dim = k.Navy, k.Navy
		} else if i > 0 {
			d.VLine(x, p.y+4, p.y+h-4, k.Hex(0x3a5a85), 0.6)
		}
		vc := fg
		if c.Warn && !c.Accent {
			vc = k.Hex(0xffd966)
		}
		d.Text(x+8, p.y+11, k.TrUpper(c.Label), true, 5.7, dim)
		size := 15.5
		for size > 9 && d.Width(c.Value, true, size) > cw-14 {
			size -= 1
		}
		d.Text(x+8, p.y+11+size+0.5, c.Value, true, size, vc)
		if c.Sub != "" {
			d.Text(x+8, p.y+h-4, d.Fit(c.Sub, c.Accent, 6.1, cw-12), c.Accent, 6.1, dim)
		}
	}
}

// cardBox — açık zeminli kart çerçevesi çizer.
func (p *page) cardBox(x, y, w, h float64) {
	p.d.FillRect(x, y, w, h, k.Wash)
	p.d.StrokeRect(x, y, w, h, k.Rule, 0.6)
}

// noteBox — amber sol çizgili uyarı/not kutusu; yüksekliği döner.
func (p *page) noteBox(x, y, w float64, head, body string) float64 {
	d := p.d
	lines := d.Wrap(body, false, 7, w-16)
	h := 8.0 + float64(len(lines))*9.2
	if head != "" {
		h += 9.5
	}
	d.FillRect(x, y, w, h, k.WarnBg)
	d.FillRect(x, y, 2.2, h, k.Amber)
	ty := y + 10
	if head != "" {
		d.Text(x+9, ty, k.TrUpper(head), true, 5.9, k.WarnFg)
		ty += 9.5
	}
	for _, ln := range lines {
		d.Text(x+9, ty, ln, false, 7, k.Ink)
		ty += 9.2
	}
	return h
}

// bar — yatay oran çubuğu (iz + dolgu).
func (p *page) bar(x, y, w, h, frac float64, c k.RGB) {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	p.d.FillRect(x, y, w, h, k.Track)
	if frac > 0 {
		p.d.FillRect(x, y, w*frac, h, c)
	}
}

// ------------------------------------------------------------ biçim yardımcıları

// Num — Türkçe sayı biçimi: binlik nokta, ondalık virgül.
func Num(v float64, dec int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%.*f", dec, v)
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i+1:]
	}
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if frac != "" {
		out += "," + frac
	}
	if neg {
		out = "−" + out
	}
	return out
}

// Qty — miktar: tam sayıysa ondalıksız, değilse en çok 2 hane.
func Qty(v float64) string {
	if v == float64(int64(v)) {
		return Num(v, 0)
	}
	s := Num(v, 2)
	return strings.TrimRight(strings.TrimRight(s, "0"), ",")
}

// Mil — milyon biçimi ("47,13 M").
func Mil(v float64) string { return Num(v/1e6, 2) + " M" }

var trMonths = []string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"}
var trDays = []string{"Pazar", "Pazartesi", "Salı", "Çarşamba", "Perşembe", "Cuma", "Cumartesi"}

// TRMonth — ay adı (1..12).
func TRMonth(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return trMonths[m-1]
}

// TRDay — gün adı.
func TRDay(t time.Time) string { return trDays[int(t.Weekday())] }

// DMY — 02.01.2006.
func DMY(t time.Time) string { return t.Format("02.01.2006") }

// ISO2DMY — "2026-10-14" → "14.10.2026"; çözülemezse girdiyi döner.
func ISO2DMY(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return DMY(t)
}

func cosf(a float64) float64 { return math.Cos(a) }
func sinf(a float64) float64 { return math.Sin(a) }
