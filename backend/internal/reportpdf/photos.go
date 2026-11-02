package reportpdf

import (
	k "github.com/ipks/ipks/backend/internal/pdfkit"
)

// Photo — rapora eklenecek bir fotoğraf (Data: JPEG/PNG/GIF baytları).
type Photo struct {
	Data     []byte
	Title    string // kalın başlık (iş / konum / dosya adı)
	Sub      string // ikinci satır (tarih, çekim sahibi)
	Tag      string // köşe etiketi ("KAPAK", "SORUN", "PZT")
	Featured bool   // öne çıkan (büyük) fotoğraf
}

// photoBox — fotoğrafı kutuya çizer; çözülemezse yer tutucu bırakır.
func (p *page) photoBox(x, y, w, h float64, ph Photo) {
	d := p.d
	d.FillRect(x, y, w, h, k.PhotoBg)
	if len(ph.Data) == 0 || !d.Image(x, y, w, h, ph.Data) {
		d.TextC(x+w/2, y+h/2+2, "Fotoğraf yüklenemedi", false, 7, k.Steel)
	}
	d.StrokeRect(x, y, w, h, k.Hex(0xc4cfdd), 0.6)
	if ph.Tag != "" {
		p.d.Chip(x+5, y+13, ph.Tag, k.White, k.Navy)
	}
}

func (p *page) photoCaption(x, y, w float64, ph Photo) {
	d := p.d
	if ph.Title != "" {
		d.Text(x, y+8, d.Fit(ph.Title, true, 7, w), true, 7, k.Navy)
	}
	if ph.Sub != "" {
		d.Text(x, y+17, d.Fit(ph.Sub, false, 6.4, w), false, 6.4, k.Dim)
	}
}

// photoFeatured — tam genişlikte büyük fotoğraf (+ başlık satırı).
func (p *page) photoFeatured(ph Photo, h float64) {
	p.need(h + 24)
	p.photoBox(mL, p.y, cW, h, ph)
	p.photoCaption(mL, p.y+h+3, cW, ph)
	p.y += h + 26
}

// photoGrid — cols sütunlu ızgara; sayfaya sığmayan satırlar devam sayfasına geçer.
func (p *page) photoGrid(photos []Photo, cols int, imgH float64) {
	if len(photos) == 0 {
		return
	}
	const gap = 9.0
	w := (cW - gap*float64(cols-1)) / float64(cols)
	rowTotal := imgH + 26
	for i := 0; i < len(photos); i += cols {
		p.need(rowTotal)
		for j := 0; j < cols && i+j < len(photos); j++ {
			x := mL + float64(j)*(w+gap)
			p.photoBox(x, p.y, w, imgH, photos[i+j])
			p.photoCaption(x, p.y+imgH+3, w, photos[i+j])
		}
		p.y += rowTotal
	}
}

// emptyNote — veri yokken bölümün altına gri açıklama yazar.
func (p *page) emptyNote(s string) {
	p.need(14)
	p.d.Text(mL+4, p.y+9, s, false, bodySize, k.Dim)
	p.y += 14
}
