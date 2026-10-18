package payments

// Minimal TrueType okuyucu + PDF CID yazı tipi gömme (yalnızca stdlib).
// Amaç: Türkçe karakterlerin (ş ğ İ ı ...) her PDF okuyucuda aynı çizilmesi
// için Liberation Sans (SIL OFL 1.1, bkz. fonts/LICENSE-Liberation.txt)
// dosyasını PDF'e gömmek. Yalnızca BMP, cmap format 4 desteklenir; bu, bu
// raporun metin ihtiyacı için yeterlidir.

import (
	"bytes"
	"compress/zlib"
	_ "embed"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed fonts/LiberationSans-Regular.ttf
var fontRegularTTF []byte

//go:embed fonts/LiberationSans-Bold.ttf
var fontBoldTTF []byte

type ttfFont struct {
	data       []byte
	unitsPerEm int
	ascent     int
	descent    int
	bbox       [4]int
	adv        []uint16
	segs       []cmapSeg
	cmapData   []byte
	used       map[uint16]rune // PDF'te kullanılan glifler (gid → rune)
}

type cmapSeg struct {
	start, end, delta, rangeOff uint16
	rangeOffPos                 int
}

var (
	fontsOnce             sync.Once
	baseRegular, baseBold *ttfFont
)

func loadBaseFonts() {
	baseRegular = mustParseTTF(fontRegularTTF)
	baseBold = mustParseTTF(fontBoldTTF)
}

// newFontPair — belge başına taze kullanım takibi olan iki yazı tipi döner
// (ayrıştırılmış tablolar paylaşılır, "used" kümesi belgeye özeldir).
func newFontPair() (reg, bold *ttfFont) {
	fontsOnce.Do(loadBaseFonts)
	c := func(f *ttfFont) *ttfFont {
		n := *f
		n.used = map[uint16]rune{}
		return &n
	}
	return c(baseRegular), c(baseBold)
}

func mustParseTTF(b []byte) *ttfFont {
	f, err := parseTTF(b)
	if err != nil {
		panic("gömülü yazı tipi ayrıştırılamadı: " + err.Error())
	}
	return f
}

func u16(b []byte, o int) int { return int(binary.BigEndian.Uint16(b[o:])) }
func i16(b []byte, o int) int { return int(int16(binary.BigEndian.Uint16(b[o:]))) }

func parseTTF(b []byte) (*ttfFont, error) {
	if len(b) < 12 {
		return nil, fmt.Errorf("kısa dosya")
	}
	n := u16(b, 4)
	tab := map[string][2]int{}
	for i := 0; i < n; i++ {
		o := 12 + 16*i
		if o+16 > len(b) {
			return nil, fmt.Errorf("bozuk tablo dizini")
		}
		tab[string(b[o:o+4])] = [2]int{int(binary.BigEndian.Uint32(b[o+8:])), int(binary.BigEndian.Uint32(b[o+12:]))}
	}
	need := func(t string) (int, error) {
		v, ok := tab[t]
		if !ok {
			return 0, fmt.Errorf("%s tablosu yok", t)
		}
		return v[0], nil
	}
	f := &ttfFont{data: b, used: map[uint16]rune{}}
	head, err := need("head")
	if err != nil {
		return nil, err
	}
	f.unitsPerEm = u16(b, head+18)
	f.bbox = [4]int{i16(b, head+36), i16(b, head+38), i16(b, head+40), i16(b, head+42)}
	hhea, err := need("hhea")
	if err != nil {
		return nil, err
	}
	f.ascent, f.descent = i16(b, hhea+4), i16(b, hhea+6)
	nHM := u16(b, hhea+34)
	maxp, err := need("maxp")
	if err != nil {
		return nil, err
	}
	numGlyphs := u16(b, maxp+4)
	hmtx, err := need("hmtx")
	if err != nil {
		return nil, err
	}
	f.adv = make([]uint16, numGlyphs)
	var last uint16
	for g := 0; g < numGlyphs; g++ {
		if g < nHM {
			last = uint16(u16(b, hmtx+4*g))
		}
		f.adv[g] = last
	}
	cm, err := need("cmap")
	if err != nil {
		return nil, err
	}
	nSub := u16(b, cm+2)
	sub := -1
	for i := 0; i < nSub; i++ {
		pid, eid, off := u16(b, cm+4+8*i), u16(b, cm+6+8*i), int(binary.BigEndian.Uint32(b[cm+8+8*i:]))
		if (pid == 3 && eid == 1) || (pid == 0 && sub < 0) {
			sub = cm + off
			if pid == 3 {
				break
			}
		}
	}
	if sub < 0 || u16(b, sub) != 4 {
		return nil, fmt.Errorf("cmap format 4 bulunamadı")
	}
	segX2 := u16(b, sub+6)
	segN := segX2 / 2
	endO, startO := sub+14, sub+16+segX2
	deltaO, rangeO := startO+segX2, startO+2*segX2
	for i := 0; i < segN; i++ {
		f.segs = append(f.segs, cmapSeg{
			end: uint16(u16(b, endO+2*i)), start: uint16(u16(b, startO+2*i)),
			delta: uint16(u16(b, deltaO+2*i)), rangeOff: uint16(u16(b, rangeO+2*i)), rangeOffPos: rangeO + 2*i,
		})
	}
	f.cmapData = b
	return f, nil
}

func (f *ttfFont) gid(r rune) uint16 {
	if r > 0xFFFF {
		return 0
	}
	c := uint16(r)
	for _, s := range f.segs {
		if c > s.end {
			continue
		}
		if c < s.start {
			return 0
		}
		if s.rangeOff == 0 {
			return c + s.delta
		}
		o := s.rangeOffPos + int(s.rangeOff) + 2*int(c-s.start)
		if o+2 > len(f.cmapData) {
			return 0
		}
		g := uint16(u16(f.cmapData, o))
		if g == 0 {
			return 0
		}
		return g + s.delta
	}
	return 0
}

// width — metin genişliği (puan).
func (f *ttfFont) width(s string, size float64) float64 {
	var w float64
	for _, r := range s {
		g := f.gid(r)
		if int(g) < len(f.adv) {
			w += float64(f.adv[g])
		}
	}
	return w * size / float64(f.unitsPerEm)
}

// hex — metni 2 baytlık glif kimlikleri olarak hex dizeye çevirir ve kullanımı kaydeder.
func (f *ttfFont) hex(s string) string {
	var b strings.Builder
	b.WriteByte('<')
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		g := f.gid(r)
		if _, ok := f.used[g]; !ok && g != 0 {
			f.used[g] = r
		}
		fmt.Fprintf(&b, "%04X", g)
	}
	b.WriteByte('>')
	return b.String()
}

// objects — bu yazı tipi için PDF nesne gövdelerini sırayla üretir:
// [Type0, CIDFontType2, FontDescriptor, FontFile2, ToUnicode]; ilk nesnenin
// numarası base'dir (diğerleri base+1..base+4).
func (f *ttfFont) objects(base int, psName string) []string {
	scale := func(v int) int { return v * 1000 / f.unitsPerEm }

	gids := make([]int, 0, len(f.used))
	for g := range f.used {
		gids = append(gids, int(g))
	}
	sort.Ints(gids)
	var w strings.Builder
	for _, g := range gids {
		fmt.Fprintf(&w, "%d [%d] ", g, scale(int(f.adv[g])))
	}

	var cmap strings.Builder
	cmap.WriteString("/CIDInit /ProcSet findresource begin 12 dict begin begincmap\n" +
		"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n" +
		"/CMapName /Adobe-Identity-UCS def /CMapType 2 def\n1 begincodespacerange <0000> <FFFF> endcodespacerange\n")
	for i := 0; i < len(gids); i += 100 {
		end := i + 100
		if end > len(gids) {
			end = len(gids)
		}
		fmt.Fprintf(&cmap, "%d beginbfchar\n", end-i)
		for _, g := range gids[i:end] {
			fmt.Fprintf(&cmap, "<%04X> <%04X>\n", g, f.used[uint16(g)])
		}
		cmap.WriteString("endbfchar\n")
	}
	cmap.WriteString("endcmap CMapName currentdict /CMap defineresource pop end end")
	cm := cmap.String()

	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	_, _ = zw.Write(f.data)
	_ = zw.Close()

	stemV := 80
	if strings.Contains(psName, "Bold") {
		stemV = 140
	}
	return []string{
		fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>",
			psName, base+1, base+4),
		fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> "+
			"/FontDescriptor %d 0 R /CIDToGIDMap /Identity /DW 556 /W [%s] >>", psName, base+2, w.String()),
		fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags 32 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV %d /FontFile2 %d 0 R >>",
			psName, scale(f.bbox[0]), scale(f.bbox[1]), scale(f.bbox[2]), scale(f.bbox[3]), scale(f.ascent), scale(f.descent), 700, stemV, base+3),
		fmt.Sprintf("<< /Length %d /Length1 %d /Filter /FlateDecode >>\nstream\n%s\nendstream", zbuf.Len(), len(f.data), zbuf.String()),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cm), cm),
	}
}
