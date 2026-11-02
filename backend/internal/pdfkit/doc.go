// Package pdfkit — ConCoord rapor PDF'leri için ortak, yalnızca stdlib'e
// dayanan çizim motoru: A4 sayfa, gömülü Liberation Sans (Türkçe karakterler),
// şekiller, çizgi grafikler ve JPEG/PNG fotoğraf yerleştirme. Koordinatlar
// puandır (1/72 inç); y ekranın üstünden aşağı doğru ölçülür.
package pdfkit

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"strings"
	"unicode"
)

const (
	PgW = 595.0
	PgH = 842.0
)

// RGB — 0..1 aralığında renk.
type RGB struct{ R, G, B float64 }

// Hex — 0xRRGGBB değerinden renk.
func Hex(h uint32) RGB {
	return RGB{float64(h>>16&255) / 255, float64(h>>8&255) / 255, float64(h&255) / 255}
}

// Marka ve rapor paleti (hakediş PDF'iyle aynı lacivert/amber dili).
var (
	Navy    = Hex(0x0b2545)
	Mid     = Hex(0x1d3d68)
	Amber   = Hex(0xe0b400)
	Ink     = Hex(0x14202e)
	Dim     = Hex(0x5b6877)
	Rule    = Hex(0xd5dce5)
	Wash    = Hex(0xf3f6fa)
	White   = Hex(0xffffff)
	Soft    = Hex(0x9fb0c6)
	Steel   = Hex(0x7d96b8)
	Track   = Hex(0xe3e9f1)
	WarnBg  = Hex(0xfbf7e6)
	WarnFg  = Hex(0x8a5200)
	OkBg    = Hex(0xe3f3ea)
	OkFg    = Hex(0x14673a)
	BadBg   = Hex(0xfbe1df)
	BadFg   = Hex(0x9b2318)
	OnNavy  = Hex(0xc9d6e8) // lacivert üzerinde ikincil yazı
	OnNavy2 = Hex(0x9fb2cc)
	PhotoBg = Hex(0xdfe6ef)
)

// Doc — çok sayfalı PDF belgesi.
type Doc struct {
	pages        []*bytes.Buffer
	cur          *bytes.Buffer
	reg, boldFnt *Font
	images       []*imgObj
}

type imgObj struct {
	jpg  []byte
	w, h int
}

// New — boş belge.
func New() *Doc {
	r, b := NewFontPair()
	return &Doc{reg: r, boldFnt: b}
}

// NewPage — yeni sayfa açar ve geçerli sayfa yapar.
func (d *Doc) NewPage() {
	d.cur = &bytes.Buffer{}
	d.pages = append(d.pages, d.cur)
}

// PageCount — açılmış sayfa sayısı.
func (d *Doc) PageCount() int { return len(d.pages) }

// SetPage — sonradan çizim için (alt bilgi gibi) sayfa seçer (0 tabanlı).
func (d *Doc) SetPage(i int) { d.cur = d.pages[i] }

func (d *Doc) font(bold bool) *Font {
	if bold {
		return d.boldFnt
	}
	return d.reg
}

// Has — yazı tipinde bu karakter için glif var mı?
func (d *Doc) Has(r rune) bool { return d.reg.gid(r) != 0 }

// ---------------------------------------------------------------- şekiller

func (d *Doc) FillRect(x, y, w, h float64, c RGB) {
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f rg %.2f %.2f %.2f %.2f re f\n", c.R, c.G, c.B, x, PgH-y-h, w, h)
}

func (d *Doc) StrokeRect(x, y, w, h float64, c RGB, lw float64) {
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f RG %.2f w %.2f %.2f %.2f %.2f re S\n", c.R, c.G, c.B, lw, x, PgH-y-h, w, h)
}

func (d *Doc) HLine(x1, x2, y float64, c RGB, lw float64) {
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n", c.R, c.G, c.B, lw, x1, PgH-y, x2, PgH-y)
}

func (d *Doc) VLine(x, y1, y2 float64, c RGB, lw float64) {
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f RG %.2f w %.2f %.2f m %.2f %.2f l S\n", c.R, c.G, c.B, lw, x, PgH-y1, x, PgH-y2)
}

// Polyline — açık çizgi; dash boşsa düz çizgi.
func (d *Doc) Polyline(pts [][2]float64, c RGB, lw float64, dash ...float64) {
	if len(pts) < 2 {
		return
	}
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f RG %.2f w ", c.R, c.G, c.B, lw)
	if len(dash) > 0 {
		fmt.Fprintf(d.cur, "[%.2f %.2f] 0 d ", dash[0], dash[len(dash)-1])
	}
	for i, p := range pts {
		op := "l"
		if i == 0 {
			op = "m"
		}
		fmt.Fprintf(d.cur, "%.2f %.2f %s ", p[0], PgH-p[1], op)
	}
	d.cur.WriteString("S [] 0 d\n")
}

// FillPoly — kapalı dolu çokgen.
func (d *Doc) FillPoly(pts [][2]float64, c RGB) {
	if len(pts) < 3 {
		return
	}
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f rg ", c.R, c.G, c.B)
	for i, p := range pts {
		op := "l"
		if i == 0 {
			op = "m"
		}
		fmt.Fprintf(d.cur, "%.2f %.2f %s ", p[0], PgH-p[1], op)
	}
	d.cur.WriteString("f\n")
}

// FillCircle — dolu daire.
func (d *Doc) FillCircle(cx, cy, r float64, c RGB) {
	const k = 0.5523
	x, y := cx, PgH-cy
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f rg %.2f %.2f m ", c.R, c.G, c.B, x+r, y)
	fmt.Fprintf(d.cur, "%.2f %.2f %.2f %.2f %.2f %.2f c ", x+r, y+k*r, x+k*r, y+r, x, y+r)
	fmt.Fprintf(d.cur, "%.2f %.2f %.2f %.2f %.2f %.2f c ", x-k*r, y+r, x-r, y+k*r, x-r, y)
	fmt.Fprintf(d.cur, "%.2f %.2f %.2f %.2f %.2f %.2f c ", x-r, y-k*r, x-k*r, y-r, x, y-r)
	fmt.Fprintf(d.cur, "%.2f %.2f %.2f %.2f %.2f %.2f c f\n", x+k*r, y-r, x+r, y-k*r, x+r, y)
}

// RingSeg — halka dilimi (donut): a0..a1 derece, 0° = saat 12 yönü, saat yönünde.
func (d *Doc) RingSeg(cx, cy, rOut, rIn, a0, a1 float64, c RGB) {
	if a1-a0 < 0.05 {
		return
	}
	steps := int(math.Max(6, math.Ceil((a1-a0)/4)))
	var pts [][2]float64
	pt := func(a, r float64) [2]float64 {
		rad := (a - 90) * math.Pi / 180
		return [2]float64{cx + r*math.Cos(rad), cy + r*math.Sin(rad)}
	}
	for i := 0; i <= steps; i++ {
		pts = append(pts, pt(a0+(a1-a0)*float64(i)/float64(steps), rOut))
	}
	for i := steps; i >= 0; i-- {
		pts = append(pts, pt(a0+(a1-a0)*float64(i)/float64(steps), rIn))
	}
	d.FillPoly(pts, c)
}

// ------------------------------------------------------------------ metin

// Text — y üstten ölçülen taban çizgisidir.
func (d *Doc) Text(x, y float64, s string, bold bool, size float64, c RGB) {
	name := "F1"
	if bold {
		name = "F2"
	}
	fmt.Fprintf(d.cur, "BT /%s %.2f Tf %.3f %.3f %.3f rg %.2f %.2f Td %s Tj ET\n",
		name, size, c.R, c.G, c.B, x, PgH-y, d.font(bold).Hex(s))
}

func (d *Doc) TextR(xRight, y float64, s string, bold bool, size float64, c RGB) {
	d.Text(xRight-d.Width(s, bold, size), y, s, bold, size, c)
}

func (d *Doc) TextC(xCenter, y float64, s string, bold bool, size float64, c RGB) {
	d.Text(xCenter-d.Width(s, bold, size)/2, y, s, bold, size, c)
}

func (d *Doc) Width(s string, bold bool, size float64) float64 {
	return d.font(bold).Width(s, size)
}

// Fit — metni verilen genişliğe sığacak şekilde sonuna "…" ekleyerek kısaltır.
func (d *Doc) Fit(s string, bold bool, size, maxW float64) string {
	if d.Width(s, bold, size) <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 1 {
		rs = rs[:len(rs)-1]
		t := strings.TrimRight(string(rs), " ") + "…"
		if d.Width(t, bold, size) <= maxW {
			return t
		}
	}
	return "…"
}

// Wrap — metni kelime sınırında satırlara böler.
func (d *Doc) Wrap(s string, bold bool, size, maxW float64) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		cur := ""
		for _, w := range strings.Fields(para) {
			t := w
			if cur != "" {
				t = cur + " " + w
			}
			if cur != "" && d.Width(t, bold, size) > maxW {
				lines = append(lines, cur)
				cur = w
			} else {
				cur = t
			}
		}
		lines = append(lines, cur)
	}
	return lines
}

// Chip — dolgulu küçük etiket; sağ kenarı döner.
func (d *Doc) Chip(x, y float64, s string, fg, bg RGB) float64 {
	s = TrUpper(s)
	w := d.Width(s, true, 6.4) + 9
	d.FillRect(x, y-7.2, w, 10.4, bg)
	d.Text(x+4.5, y, s, true, 6.4, fg)
	return x + w
}

// Check — küçük onay işareti (yazı tipinde ✓ olmadığı için çizilir).
func (d *Doc) Check(x, y float64, c RGB) {
	d.Polyline([][2]float64{{x, y - 3}, {x + 2.4, y}, {x + 7, y - 6.5}}, c, 1.3)
}

// Arrow — küçük sağa ok.
func (d *Doc) Arrow(x, y float64, c RGB) {
	d.Polyline([][2]float64{{x, y - 3.2}, {x + 7, y - 3.2}}, c, 1.2)
	d.Polyline([][2]float64{{x + 4.6, y - 5.8}, {x + 7.2, y - 3.2}, {x + 4.6, y - 0.6}}, c, 1.2)
}

// TrUpper — Türkçe büyük harf kuralı (i → İ, ı → I).
func TrUpper(s string) string { return strings.Map(unicode.TurkishCase.ToUpper, s) }

// ------------------------------------------------------------- fotoğraflar

// PhotoMaxEdge — gömülen fotoğrafın uzun kenarı (piksel) üst sınırı.
const PhotoMaxEdge = 1100

// Image — fotoğrafı kutuya "cover" biçiminde (kırparak) yerleştirir. Çözümlenemeyen
// veride false döner ve hiçbir şey çizmez; çağıran yer tutucu çizebilir.
func (d *Doc) Image(x, y, w, h float64, data []byte) bool {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}
	img = shrink(img, PhotoMaxEdge)
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(rgba, b, img, b.Min, draw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 78}); err != nil {
		return false
	}
	im := &imgObj{jpg: buf.Bytes(), w: b.Dx(), h: b.Dy()}
	d.images = append(d.images, im)
	n := len(d.images)

	scale := math.Max(w/float64(im.w), h/float64(im.h))
	dw, dh := float64(im.w)*scale, float64(im.h)*scale
	dx, dy := x+(w-dw)/2, y+(h-dh)/2
	fmt.Fprintf(d.cur, "q %.2f %.2f %.2f %.2f re W n %.2f 0 0 %.2f %.2f %.2f cm /Im%d Do Q\n",
		x, PgH-y-h, w, h, dw, dh, dx, PgH-dy-dh, n)
	return true
}

// shrink — uzun kenar max pikseli aşıyorsa kutu süzgeciyle küçültür.
func shrink(src image.Image, max int) image.Image {
	b := src.Bounds()
	long := b.Dx()
	if b.Dy() > long {
		long = b.Dy()
	}
	if long <= max {
		return src
	}
	f := float64(max) / float64(long)
	nw, nh := int(float64(b.Dx())*f), int(float64(b.Dy())*f)
	if nw < 1 || nh < 1 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy0 := b.Min.Y + int(float64(y)/f)
		sy1 := b.Min.Y + int(float64(y+1)/f)
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := 0; x < nw; x++ {
			sx0 := b.Min.X + int(float64(x)/f)
			sx1 := b.Min.X + int(float64(x+1)/f)
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, bl, a, n uint32
			for sy := sy0; sy < sy1 && sy < b.Max.Y; sy++ {
				for sx := sx0; sx < sx1 && sx < b.Max.X; sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r += cr
					g += cg
					bl += cb
					a += ca
					n++
				}
			}
			if n == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), uint8(a / n >> 8)})
		}
	}
	return dst
}

// ------------------------------------------------------------------ çıktı

// Finish — PDF baytlarını üretir.
func (d *Doc) Finish() []byte {
	var objs []string
	objs = append(objs, "") // 0 kullanılmaz
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>")
	objs = append(objs, "")                                             // 2: Pages
	objs = append(objs, d.reg.Objects(3, "LiberationSans")...)          // 3..7
	objs = append(objs, d.boldFnt.Objects(8, "LiberationSans-Bold")...) // 8..12

	var xo strings.Builder
	for i, im := range d.images {
		num := len(objs)
		objs = append(objs, fmt.Sprintf(
			"<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n%s\nendstream",
			im.w, im.h, len(im.jpg), im.jpg))
		fmt.Fprintf(&xo, "/Im%d %d 0 R ", i+1, num)
	}
	res := "<< /Font << /F1 3 0 R /F2 8 0 R >>"
	if xo.Len() > 0 {
		res += " /XObject << " + xo.String() + ">>"
	}
	res += " >>"

	var kids []string
	for _, pg := range d.pages {
		content := pg.String()
		contentObj := len(objs)
		objs = append(objs, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
		pageObj := len(objs)
		objs = append(objs, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources %s /Contents %d 0 R >>",
			PgW, PgH, res, contentObj))
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
