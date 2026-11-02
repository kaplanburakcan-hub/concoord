// Package reportphotos — raporlara eklenecek proje fotoğraflarını bulur ve
// depodan okur. Kaynak, "Fotoğraflar" sayfasının kullandığı documents
// satırlarıdır (entity_type='project_photo'); günlük raporun kapak fotoğrafı
// ayrıca reports paketinde yüklenir.
package reportphotos

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/reportpdf"
	"github.com/ipks/ipks/backend/internal/storage"
)

// TZ — rapor günlerinin sınırı (proje saati).
const TZ = "Europe/Istanbul"

// maxBytes — tek fotoğraf için okuma üst sınırı (yükleme sınırıyla uyumlu).
const maxBytes = 10 << 20

// Ref — bir proje fotoğrafının künyesi (baytlar Load ile okunur).
type Ref struct {
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Category string    `json:"category"`
	TakenAt  time.Time `json:"taken_at"`
}

// List — [from, to] (iki uç dahil, proje saatiyle gün) aralığında yüklenen proje
// fotoğraflarını yeniden eskiye değil, eskiden yeniye sıralı döner. Belgenin en
// son sürümünün dosyası esas alınır. limit <= 0 ise 24.
func List(ctx context.Context, pool *pgxpool.Pool, pid uuid.UUID, from, to time.Time, limit int) ([]Ref, error) {
	if limit <= 0 {
		limit = 24
	}
	rows, err := pool.Query(ctx, `
		SELECT f.storage_key, d.title, d.doc_category, d.created_at
		FROM documents d
		JOIN LATERAL (
			SELECT file_id FROM document_versions
			WHERE document_id = d.id ORDER BY version_no DESC LIMIT 1
		) dv ON true
		JOIN files f ON f.id = dv.file_id
		WHERE d.project_id = $1 AND d.deleted_at IS NULL
		  AND d.entity_type = 'project_photo'
		  AND f.mime LIKE 'image/%'
		  AND (d.created_at AT TIME ZONE $4)::date BETWEEN $2::date AND $3::date
		ORDER BY d.created_at
		LIMIT $5`, pid, from, to, TZ, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ref
	for rows.Next() {
		var r Ref
		if err := rows.Scan(&r.Key, &r.Title, &r.Category, &r.TakenAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Fetch — depodan bir nesnenin baytlarını okur (hata veya aşırı boyutta nil).
func Fetch(ctx context.Context, store *storage.Client, key string) []byte {
	if store == nil || key == "" {
		return nil
	}
	obj, err := store.GetObject(ctx, key)
	if err != nil {
		return nil
	}
	defer obj.Body.Close()
	b, err := io.ReadAll(io.LimitReader(obj.Body, maxBytes+1))
	if err != nil || len(b) > maxBytes {
		return nil
	}
	return b
}

// CategoryTR — fotoğraf türünün kısa Türkçe adı.
func CategoryTR(c string) string {
	switch c {
	case "SahaFotografi":
		return "Saha"
	case "ImalatFotografi":
		return "İmalat"
	case "DenetimFotografi":
		return "Denetim"
	}
	return ""
}

// ToPhotos — künyeleri PDF fotoğraflarına çevirir; baytlar okunamayan kayıt
// atlanmaz, PDF'te "yüklenemedi" yer tutucusu olarak görünür.
func ToPhotos(ctx context.Context, store *storage.Client, refs []Ref, loc *time.Location) []reportpdf.Photo {
	out := make([]reportpdf.Photo, 0, len(refs))
	for _, r := range refs {
		t := r.TakenAt
		if loc != nil {
			t = t.In(loc)
		}
		sub := t.Format("02.01.2006 15:04")
		if c := CategoryTR(r.Category); c != "" {
			sub += " · " + c
		}
		out = append(out, reportpdf.Photo{Data: Fetch(ctx, store, r.Key), Title: r.Title, Sub: sub})
	}
	return out
}
