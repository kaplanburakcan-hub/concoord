package projects

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// provinceCentroids — Türkiye'nin 81 ilinin yaklaşık merkez koordinatı.
// Yalnızca kullanıcı tam koordinat girmediğinde, il seçimine dayalı küçük bir
// önizleme haritası üretmek için kullanılır (hassas konumlandırma değil).
var provinceCentroids = map[string][2]float64{
	"Adana": {37.00, 35.32}, "Adıyaman": {37.76, 38.28}, "Afyonkarahisar": {38.76, 30.54},
	"Ağrı": {39.72, 43.05}, "Amasya": {40.65, 35.83}, "Ankara": {39.93, 32.86},
	"Antalya": {36.90, 30.71}, "Artvin": {41.18, 41.82}, "Aydın": {37.85, 27.85},
	"Balıkesir": {39.65, 27.89}, "Bilecik": {40.15, 29.98}, "Bingöl": {38.89, 40.50},
	"Bitlis": {38.40, 42.11}, "Bolu": {40.74, 31.61}, "Burdur": {37.72, 30.29},
	"Bursa": {40.18, 29.06}, "Çanakkale": {40.15, 26.41}, "Çankırı": {40.60, 33.62},
	"Çorum": {40.55, 34.95}, "Denizli": {37.77, 29.09}, "Diyarbakır": {37.91, 40.24},
	"Edirne": {41.68, 26.56}, "Elazığ": {38.68, 39.22}, "Erzincan": {39.75, 39.49},
	"Erzurum": {39.90, 41.27}, "Eskişehir": {39.78, 30.52}, "Gaziantep": {37.07, 37.38},
	"Giresun": {40.92, 38.39}, "Gümüşhane": {40.46, 39.48}, "Hakkari": {37.58, 43.74},
	"Hatay": {36.20, 36.16}, "Isparta": {37.77, 30.55}, "Mersin": {36.81, 34.64},
	"İstanbul": {41.01, 28.98}, "İzmir": {38.42, 27.14}, "Kars": {40.60, 43.09},
	"Kastamonu": {41.38, 33.78}, "Kayseri": {38.73, 35.49}, "Kırklareli": {41.73, 27.22},
	"Kırşehir": {39.15, 34.16}, "Kocaeli": {40.85, 29.88}, "Konya": {37.87, 32.48},
	"Kütahya": {39.42, 29.99}, "Malatya": {38.36, 38.31}, "Manisa": {38.61, 27.43},
	"Kahramanmaraş": {37.58, 36.92}, "Mardin": {37.31, 40.74}, "Muğla": {37.22, 28.36},
	"Muş": {38.74, 41.49}, "Nevşehir": {38.62, 34.72}, "Niğde": {37.97, 34.68},
	"Ordu": {40.98, 37.88}, "Rize": {41.02, 40.52}, "Sakarya": {40.78, 30.40},
	"Samsun": {41.29, 36.33}, "Siirt": {37.93, 41.94}, "Sinop": {42.03, 35.16},
	"Sivas": {39.75, 37.02}, "Tekirdağ": {40.98, 27.51}, "Tokat": {40.31, 36.55},
	"Trabzon": {41.00, 39.72}, "Tunceli": {39.11, 39.54}, "Şanlıurfa": {37.16, 38.79},
	"Uşak": {38.68, 29.41}, "Van": {38.49, 43.38}, "Yozgat": {39.82, 34.80},
	"Zonguldak": {41.46, 31.79}, "Aksaray": {38.37, 34.03}, "Bayburt": {40.26, 40.22},
	"Karaman": {37.18, 33.22}, "Kırıkkale": {39.85, 33.51}, "Batman": {37.88, 41.13},
	"Şırnak": {37.52, 42.46}, "Bartın": {41.63, 32.34}, "Ardahan": {41.11, 42.70},
	"Iğdır": {39.92, 44.04}, "Yalova": {40.65, 29.28}, "Karabük": {41.20, 32.63},
	"Kilis": {36.72, 37.12}, "Osmaniye": {37.07, 36.25}, "Düzce": {40.84, 31.16},
}

const locationMapCategory = "KonumHaritasi"

// captureLocationMapOnce — proje için daha önce yakalanmış bir konum
// haritası yoksa, il merkezine (koordinat yoksa) ya da tam koordinata göre
// OpenStreetMap üzerinden ücretsiz bir statik harita görüntüsü çeker ve
// documents motoruna (kategori "KonumHaritasi") kaydeder. Yalnızca BİR kez
// çalışır — bir sonraki çağrıda mevcut kayıt bulunup atlanır. Arka planda
// çalıştığı için hatalar yalnızca loglanır, kullanıcıya yansımaz.
func (h *Handler) captureLocationMapOnce(projectID uuid.UUID, il *string, lat, lon *float64, uid uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var exists bool
	if err := h.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM documents
			WHERE entity_type='project' AND entity_id=$1 AND doc_category=$2 AND deleted_at IS NULL)`,
		projectID, locationMapCategory).Scan(&exists); err != nil {
		h.log.Error("konum haritası kontrolü", "err", err, "project_id", projectID)
		return
	}
	if exists {
		return
	}

	var effLat, effLon float64
	zoom := 8
	if lat != nil && lon != nil {
		effLat, effLon, zoom = *lat, *lon, 14
	} else if il != nil {
		c, ok := provinceCentroids[strings.TrimSpace(*il)]
		if !ok {
			return // bilinmeyen il adı — harita üretmeden çık
		}
		effLat, effLon = c[0], c[1]
	} else {
		return
	}

	data, err := fetchLocationMapPNG(ctx, effLat, effLon, zoom)
	if err != nil {
		h.log.Error("konum haritası oluşturulamadı", "err", err)
		return
	}

	sum := sha256.Sum256(data)
	shaHex := hex.EncodeToString(sum[:])
	key := fmt.Sprintf("project/%s/location-map.png", projectID.String())

	if err := h.store.PutObject(ctx, key, "image/png", strings.NewReader(string(data)), int64(len(data)), shaHex); err != nil {
		h.log.Error("konum haritası depoya yazılamadı", "err", err, "key", key)
		return
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("konum haritası kaydı başlatılamadı", "err", err)
		return
	}
	defer tx.Rollback(ctx)

	var uidPtr *uuid.UUID
	if uid != uuid.Nil {
		uidPtr = &uid
	}

	var docID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO documents (project_id, folder_id, entity_type, entity_id, title, doc_category, created_by)
		VALUES ($1, NULL, 'project', $1, 'Konum Haritası', $2, $3)
		RETURNING id`,
		projectID, locationMapCategory, uidPtr).Scan(&docID); err != nil {
		h.log.Error("konum haritası doküman kaydı oluşturulamadı", "err", err)
		return
	}
	var fileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO files (storage_key, original_name, mime, size_bytes, sha256, uploaded_by)
		VALUES ($1,'konum-haritasi.png','image/png',$2,$3,$4) RETURNING id`,
		key, int64(len(data)), shaHex, uidPtr).Scan(&fileID); err != nil {
		h.log.Error("konum haritası dosya kaydı oluşturulamadı", "err", err)
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO document_versions (document_id, version_no, file_id, uploaded_by, sha256)
		VALUES ($1, 1, $2, $3, $4)`,
		docID, fileID, uidPtr, shaHex); err != nil {
		h.log.Error("konum haritası versiyon kaydı oluşturulamadı", "err", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		h.log.Error("konum haritası kaydı tamamlanamadı", "err", err)
	}
}

const tileSize = 256

// fetchLocationMapPNG — OpenStreetMap'in standart tile sunucusundan (bilinen,
// gerçekten çözümlenen tek uç nokta) verilen noktanın etrafındaki 3×3 tile'ı
// çekip birleştirir, tam koordinatın üzerine kırmızı bir işaretçi çizer ve
// 640×400'e kırpar. "Statik harita" servisleri (staticmap.openstreetmap.de
// vb.) çoğu zaman artık çözümlenmiyor/kaldırılmış; bu yüzden dış bir sarmalayıcı
// servise değil, doğrudan tile sunucusuna + standart kütüphane image paketine
// dayanılıyor.
func fetchLocationMapPNG(ctx context.Context, lat, lon float64, zoom int) ([]byte, error) {
	n := math.Exp2(float64(zoom))
	xf := (lon + 180.0) / 360.0 * n
	latRad := lat * math.Pi / 180
	yf := (1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * n

	centerX := int(math.Floor(xf))
	centerY := int(math.Floor(yf))
	nInt := int(n)

	canvas := image.NewRGBA(image.Rect(0, 0, tileSize*3, tileSize*3))
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			tx := ((centerX+dx)%nInt + nInt) % nInt // antimeridyende sarma
			ty := centerY + dy
			tile := fetchOSMTile(ctx, zoom, tx, ty)
			dstX, dstY := (dx+1)*tileSize, (dy+1)*tileSize
			draw.Draw(canvas, image.Rect(dstX, dstY, dstX+tileSize, dstY+tileSize), tile, image.Point{}, draw.Src)
		}
	}

	// Kanvasın sol-üst köşesi dünya piksel uzayında (centerX-1, centerY-1)
	// tile'ının başlangıcına denk gelir; işaretçi bu referansa göre konumlanır.
	markerX := xf*tileSize - float64(centerX-1)*tileSize
	markerY := yf*tileSize - float64(centerY-1)*tileSize
	drawMarker(canvas, int(markerX), int(markerY))

	cropW, cropH := 640, 400
	x0 := clampInt(int(markerX)-cropW/2, 0, tileSize*3-cropW)
	y0 := clampInt(int(markerY)-cropH/2, 0, tileSize*3-cropH)
	cropped := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	draw.Draw(cropped, cropped.Bounds(), canvas, image.Point{X: x0, Y: y0}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return nil, fmt.Errorf("harita png kodlanamadı: %w", err)
	}
	return buf.Bytes(), nil
}

// fetchOSMTile — tek bir OSM raster tile'ı çeker; başarısızsa (ör. geçersiz
// y aralığı, ağ hatası) boş beyaz bir tile döner — tek bir tile'ın eksik
// olması tüm haritayı iptal ettirmez.
func fetchOSMTile(ctx context.Context, z, x, y int) image.Image {
	blank := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	draw.Draw(blank, blank.Bounds(), &image.Uniform{C: color.RGBA{0xe8, 0xec, 0xf0, 0xff}}, image.Point{}, draw.Src)
	if y < 0 {
		return blank
	}
	url := fmt.Sprintf("https://tile.openstreetmap.org/%d/%d/%d.png", z, x, y)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return blank
	}
	// OSM tile kullanım politikası tanımlayıcı bir User-Agent ister.
	req.Header.Set("User-Agent", "ConCoord-IPKS/1.0 (dahili şantiye yönetim sistemi; proje konum önizlemesi)")
	res, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return blank
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return blank
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return blank
	}
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		return blank
	}
	return img
}

// drawMarker — (cx, cy) merkezli, beyaz kenarlıklı dolu kırmızı bir daire çizer.
func drawMarker(img *image.RGBA, cx, cy int) {
	const r = 7
	red := color.RGBA{0xd6, 0x33, 0x2b, 0xff}
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	for dy := -r - 2; dy <= r+2; dy++ {
		for dx := -r - 2; dx <= r+2; dx++ {
			dist2 := dx*dx + dy*dy
			px, py := cx+dx, cy+dy
			if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
				continue
			}
			switch {
			case dist2 <= r*r:
				img.Set(px, py, red)
			case dist2 <= (r+2)*(r+2):
				img.Set(px, py, white)
			}
		}
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
