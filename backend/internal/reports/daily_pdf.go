package reports

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/reportpdf"
	"github.com/ipks/ipks/backend/internal/reportphotos"
)

// delayLabelTR — gecikme nedeni kodunun Türkçe etiketi (rapor ekranıyla aynı).
var delayLabelTR = map[string]string{
	"Hava": "Hava koşulları", "Sahip": "Sahip / müşteri kaynaklı",
	"Tasarim": "Tasarım / proje değişikliği", "Taseron": "Taşeron kaynaklı",
	"Malzeme": "Malzeme tedarik", "Ekipman": "Ekipman arızası",
	"Idari": "İdari / resmi işlem", "Diger": "Diğer",
}

var staffStatusTR = map[string]string{
	"mevcut": "Sahada", "devamsiz": "Devamsız", "izinli": "İzinli", "raporlu": "Raporlu",
}

// maxDailyPhotos — PDF'e girecek en çok fotoğraf (kapak dahil).
const maxDailyPhotos = 13

// buildDailyData — günlük rapor kaydını PDF girdisine çevirir; kapak fotoğrafı ve
// o gün Fotoğraflar sayfasına yüklenen saha fotoğrafları eklenir.
func (h *Handler) buildDailyData(ctx context.Context, pid uuid.UUID, projectName, projectCode string, d dailyDTO) reportpdf.DailyData {
	date, _ := time.Parse("2006-01-02", d.ReportDate)
	out := reportpdf.DailyData{
		ProjectName: projectName, ProjectCode: projectCode,
		Date: date, RevisionNo: d.RevisionNo, Author: d.AuthorName, Status: d.Status,
		SubmittedAt: d.SubmittedAt, IssueFlagged: d.IssueFlagged,
		TempMin: d.TempMin, TempMax: d.TempMax,
	}
	if loc, err := time.LoadLocation(reportphotos.TZ); err == nil && d.SubmittedAt != nil {
		t := d.SubmittedAt.In(loc)
		out.SubmittedAt = &t
	}
	if len(d.Weather) > 0 {
		var w struct {
			Condition string `json:"condition"`
		}
		_ = json.Unmarshal(d.Weather, &w)
		out.WeatherCond = w.Condition
	}
	if d.Notes != nil {
		out.Notes = *d.Notes
	}
	for _, m := range d.Manpower {
		company := "Ana yüklenici"
		if m.SubcontractorName != nil && *m.SubcontractorName != "" {
			company = *m.SubcontractorName
		}
		out.Manpower = append(out.Manpower, reportpdf.DailyManRow{Trade: m.Trade, Company: company, Headcount: m.Headcount})
	}
	for _, s := range d.Staff {
		title := ""
		if s.Unvan != nil {
			title = *s.Unvan
		}
		st := staffStatusTR[s.Durum]
		if st == "" {
			st = s.Durum
		}
		out.Staff = append(out.Staff, reportpdf.DailyStaffRow{Name: s.AdSoyad, Title: title, Group: s.Grup, Status: st})
	}
	for _, e := range d.Equipment {
		idle := ""
		if e.IdleReason != nil {
			idle = *e.IdleReason
		}
		out.Equipment = append(out.Equipment, reportpdf.DailyEquipRow{Name: e.EquipmentName, Count: e.Count, Hours: e.WorkingHours, IdleReason: idle})
	}
	for _, w := range d.WorkEntries {
		row := reportpdf.DailyWorkRow{Description: w.Description, Qty: w.Qty}
		if w.WorkItemPoz != nil {
			row.Poz = *w.WorkItemPoz
		}
		if w.Location != nil {
			row.Location = *w.Location
		}
		if w.Unit != nil {
			row.Unit = *w.Unit
		}
		if w.Discipline != nil {
			row.Discipline = *w.Discipline
		}
		out.Work = append(out.Work, row)
	}
	for _, c := range d.DelayCauses {
		label := delayLabelTR[c.ReasonCode]
		if label == "" {
			label = c.ReasonCode
		}
		out.Delays = append(out.Delays, reportpdf.DailyDelayRow{Label: label, Hours: c.Hours, Note: c.Note})
	}
	for _, c := range d.CashExpenses {
		receipt := ""
		if c.ReceiptNo != nil {
			receipt = *c.ReceiptNo
		}
		out.Cash = append(out.Cash, reportpdf.DailyCashRow{Description: c.Description, Receipt: receipt, Amount: c.Amount})
	}

	// Fotoğraflar: kapak + o gün yüklenen proje fotoğrafları.
	if d.CoverPhotoFileID != nil {
		var key string
		if err := h.pool.QueryRow(ctx, `SELECT storage_key FROM files WHERE id=$1`, *d.CoverPhotoFileID).Scan(&key); err == nil {
			if b := reportphotos.Fetch(ctx, h.store, key); b != nil {
				out.Photos = append(out.Photos, reportpdf.Photo{Data: b, Title: "Kapak fotoğrafı", Sub: strings.TrimSpace(reportpdf.DMY(date)), Tag: "Kapak", Featured: true})
			}
		}
	}
	if refs, err := reportphotos.List(ctx, h.pool, pid, date, date, maxDailyPhotos-len(out.Photos)); err == nil {
		loc, _ := time.LoadLocation(reportphotos.TZ)
		out.Photos = append(out.Photos, reportphotos.ToPhotos(ctx, h.store, refs, loc)...)
		if len(out.Photos) > 0 && !out.Photos[0].Featured {
			out.Photos[0].Featured = true
		}
	}
	return out
}
