package dashboard

// period_extra.go — aylık/çeyrek/özel dönem raporlarının onaylanan v2 düzeni için
// snapshot'a eklenen bölümler: eşikler, "dikkat gerektirenler", çeyrek ay
// özetleri, riskler, kararlar, sonraki dönem hedefleri ve fotoğraflar. Bu
// bölümler isteğe bağlıdır; bir sorgu hata verirse alan boş kalır, rapor yine
// üretilir. Hiçbir değer uydurulmaz: hepsi kayıtlardan veya snapshot'taki
// rakamlardan kural ile türetilir.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/reportphotos"
)

// Thresholds — raporun karşılaştırdığı kontrol eşikleri.
type Thresholds struct {
	CPIMin float64 `json:"cpi_min"`
	SPIMin float64 `json:"spi_min"`
}

// AttentionItem — "dikkat gerektirenler" maddesi. Level: bad | warn | info.
type AttentionItem struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
}

// MonthBrief — çeyrek raporunda bir ayın özeti.
type MonthBrief struct {
	Month      string   `json:"month"` // YYYY-MM
	ProgressD  float64  `json:"progress_delta"`
	SPI        float64  `json:"spi"`
	CPI        float64  `json:"cpi"`
	NetPaid    float64  `json:"net_payments"`
	ManDays    int      `json:"man_days"`
	DelayHours float64  `json:"delay_hours"`
	Opened     int      `json:"findings_opened"`
	Closed     int      `json:"findings_closed"`
	Events     []string `json:"events,omitempty"`
}

// RiskBrief — açık/izlenen risk (olasılık × etki).
type RiskBrief struct {
	No     int    `json:"no"`
	Title  string `json:"title"`
	Prob   int    `json:"probability"`
	Impact int    `json:"impact"`
	Status string `json:"status"`
}

// DecisionBrief — dönemde alınan karar.
type DecisionBrief struct {
	Date string `json:"date"`
	Text string `json:"text"`
}

// NextBrief — sonraki çeyrek hedefleri.
type NextBrief struct {
	Label        string             `json:"label"` // "2026 Q4"
	ProgressFrom float64            `json:"progress_from"`
	ProgressTo   float64            `json:"progress_to"` // planlanan kümülatif %
	Milestones   []MonthlyMilestone `json:"milestones,omitempty"`
}

// PhotoRef — dönem fotoğrafı (baytlar PDF anında depodan okunur).
type PhotoRef struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	TakenAt  string `json:"taken_at"`
	Category string `json:"category,omitempty"`
}

// addExtras — v2 bölümlerini derler.
func (h *Handler) addExtras(ctx context.Context, pid uuid.UUID, sn *MonthlySnapshot, start, end time.Time) {
	if s, err := loadControlSettings(ctx, h.pool, pid); err == nil {
		sn.Thresholds = &Thresholds{CPIMin: s.CPIMin, SPIMin: s.SPIMin}
	}
	sn.Attention = buildAttention(sn)

	limit := 12
	if sn.Kind == KindQuarterly {
		limit = 9
	}
	if refs, err := reportphotos.List(ctx, h.pool, pid, start, end, 60); err == nil {
		for _, r := range pickSpread(refs, limit) {
			sn.Photos = append(sn.Photos, PhotoRef{Key: r.Key, Title: r.Title,
				TakenAt: r.TakenAt.Format(time.RFC3339), Category: r.Category})
		}
	}

	if sn.Kind != KindQuarterly {
		return
	}
	sn.Months = h.quarterMonths(ctx, pid, sn, start, end)
	sn.Risks = h.openRisks(ctx, pid)
	sn.Decisions = h.periodDecisions(ctx, pid, start, end)
	sn.Next = h.nextQuarter(ctx, pid, sn, end)
}

// pickSpread — liste n'den uzunsa dönem boyunca eşit aralıklı n fotoğraf seçer.
func pickSpread(refs []reportphotos.Ref, n int) []reportphotos.Ref {
	if len(refs) <= n {
		return refs
	}
	out := make([]reportphotos.Ref, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, refs[i*len(refs)/n])
	}
	return out
}

// ratio — pay/payda yüzdesi (payda 0 ise 0).
func pct(v, base float64) float64 {
	if base <= 0 {
		return 0
	}
	return v / base * 100
}

// curveAt — YYYY-MM için S-eğrisi noktası.
func curveAt(c []SCurvePoint, month string) (SCurvePoint, bool) {
	for _, p := range c {
		if p.Month == month {
			return p, true
		}
	}
	return SCurvePoint{}, false
}

// buildAttention — snapshot'tan "dikkat gerektirenler" listesi (en fazla 4).
func buildAttention(sn *MonthlySnapshot) []AttentionItem {
	var out []AttentionItem
	spiMin, cpiMin := 0.90, 0.90
	if sn.Thresholds != nil {
		spiMin, cpiMin = sn.Thresholds.SPIMin, sn.Thresholds.CPIMin
	}
	e := sn.EVM
	if e.CPI > 0 && e.CPI < cpiMin {
		out = append(out, AttentionItem{"bad", "CPI eşiğin altında",
			fmt.Sprintf("CPI %s, eşik %s. Maliyet performansı gözden geçirilmeli.", trf(e.CPI, 2), trf(cpiMin, 2))})
	}
	if e.SPI > 0 && e.SPI < spiMin {
		out = append(out, AttentionItem{"bad", "SPI eşiğin altında",
			fmt.Sprintf("SPI %s, eşik %s. Süre performansı için telafi planı gerekli.", trf(e.SPI, 2), trf(spiMin, 2))})
	}
	for _, m := range sn.Milestones {
		if m.Late {
			d := m.PlannedDate
			if d != "" {
				d = " (plan " + isoDMY(d) + ")"
			}
			out = append(out, AttentionItem{"warn", m.Name + " geç", "Kilometre taşı planlanan tarihte tamamlanmadı" + d + "."})
			if len(out) >= 3 {
				break
			}
		}
	}
	if e.SPI >= spiMin && e.SPI > 0 && e.SPI < 1 {
		out = append(out, AttentionItem{"warn", "SPI 1,00'ın altında", spiTrendText(sn)})
	}
	if sn.OHS.OpenCritical > 0 {
		out = append(out, AttentionItem{"info", fmt.Sprintf("%d açık kritik İSG bulgusu", sn.OHS.OpenCritical),
			"Kritik bulgular kapatılana dek İSG performansı izlenmeli."})
	}
	if sn.Procurement.POsOverdue > 0 {
		out = append(out, AttentionItem{"info", fmt.Sprintf("%d geciken sipariş", sn.Procurement.POsOverdue),
			"Beklenen teslim tarihi geçen siparişler tedarikçiyle takip edilmeli."})
	}
	if sn.BudgetVariance < 0 {
		out = append(out, AttentionItem{"bad", "Tamamlanma tahmini bütçeyi aşıyor",
			fmt.Sprintf("EAC bütçenin %s %s üzerinde.", fmtMil(-sn.BudgetVariance), sn.Currency)})
	}
	sort.SliceStable(out, func(i, j int) bool { return levelRank(out[i].Level) < levelRank(out[j].Level) })
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func levelRank(l string) int {
	switch l {
	case "bad":
		return 0
	case "warn":
		return 1
	}
	return 2
}

// spiTrendText — son aylardaki SPI seyri (S-eğrisinden).
func spiTrendText(sn *MonthlySnapshot) string {
	var pts []string
	last := sn.EVM.SCurve
	var vals []SCurvePoint
	for _, p := range last {
		if p.Month <= sn.EVM.AsOfMonth && p.PV > 0 {
			vals = append(vals, p)
		}
	}
	if len(vals) > 4 {
		vals = vals[len(vals)-4:]
	}
	for _, p := range vals {
		pts = append(pts, fmt.Sprintf("%s %s", monthNameTR(p.Month), trf(SPI(p.EV, p.PV), 2)))
	}
	if len(pts) < 2 {
		return "Plana göre geride; ilerleme hızı artırılmalı."
	}
	s := pts[0]
	for _, x := range pts[1:] {
		s += " → " + x
	}
	first, lastV := SPI(vals[0].EV, vals[0].PV), SPI(vals[len(vals)-1].EV, vals[len(vals)-1].PV)
	switch {
	case lastV > first:
		return s + ". Toparlanıyor; açığı kapatmak için kaynak artırımı değerlendirilmeli."
	case lastV < first:
		return s + ". Düşüş eğiliminde; telafi planı gerekli."
	}
	return s + "."
}

func fmtMil(v float64) string { return trf(v/1e6, 2) + " M" }

// trf — Türkçe ondalık biçimi (virgül).
func trf(v float64, dec int) string { return strings.ReplaceAll(fmt.Sprintf("%.*f", dec, v), ".", ",") }

func isoDMY(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02.01.2006")
}

var trMonthNames = []string{"Ocak", "Şubat", "Mart", "Nisan", "Mayıs", "Haziran", "Temmuz", "Ağustos", "Eylül", "Ekim", "Kasım", "Aralık"}

// monthNameTR — "2026-09" → "Eylül".
func monthNameTR(ym string) string {
	t, err := time.Parse("2006-01", ym)
	if err != nil {
		return ym
	}
	return trMonthNames[int(t.Month())-1]
}

// quarterMonths — çeyreğin her ayı için özet (SPI/CPI, ilerleme, hakediş, gecikme,
// İSG, personel, olaylar).
func (h *Handler) quarterMonths(ctx context.Context, pid uuid.UUID, sn *MonthlySnapshot, start, end time.Time) []MonthBrief {
	var out []MonthBrief
	bac := sn.EVM.BAC
	idx := map[string]*MonthBrief{}
	prevEV := 0.0
	if p, ok := curveAt(sn.EVM.SCurve, start.AddDate(0, -1, 0).Format("2006-01")); ok {
		prevEV = p.EV
	}
	for m := start; !m.After(end); m = m.AddDate(0, 1, 0) {
		key := m.Format("2006-01")
		b := MonthBrief{Month: key}
		if p, ok := curveAt(sn.EVM.SCurve, key); ok {
			b.SPI, b.CPI = SPI(p.EV, p.PV), CPI(p.EV, p.AC)
			b.ProgressD = round2(pct(p.EV-prevEV, bac))
			prevEV = p.EV
		}
		out = append(out, b)
	}
	for i := range out {
		idx[out[i].Month] = &out[i]
	}

	// Hakediş (kesinleşen, net).
	if rows, err := h.pool.Query(ctx, `
		SELECT to_char(finalized_at,'YYYY-MM'), COALESCE(sum(net_payable),0)::float8
		FROM progress_payments
		WHERE project_id=$1 AND deleted_at IS NULL AND status='Finalized'
		  AND finalized_at >= $2 AND finalized_at < $3 GROUP BY 1`, pid, start, end.AddDate(0, 0, 1)); err == nil {
		for rows.Next() {
			var m string
			var v float64
			if rows.Scan(&m, &v) == nil && idx[m] != nil {
				idx[m].NetPaid = round2(v)
			}
		}
		rows.Close()
	}
	// Personel (adam-gün) ve gecikme saati — gönderilmiş günlük raporlar.
	if rows, err := h.pool.Query(ctx, `
		SELECT to_char(dr.report_date,'YYYY-MM'), COALESCE(sum(dm.headcount),0)
		FROM daily_reports dr JOIN daily_manpower dm ON dm.daily_report_id = dr.id AND dm.deleted_at IS NULL
		WHERE dr.project_id=$1 AND dr.deleted_at IS NULL AND dr.status='Submitted'
		  AND dr.report_date BETWEEN $2 AND $3 GROUP BY 1`, pid, start, end); err == nil {
		for rows.Next() {
			var m string
			var v int
			if rows.Scan(&m, &v) == nil && idx[m] != nil {
				idx[m].ManDays = v
			}
		}
		rows.Close()
	}
	if rows, err := h.pool.Query(ctx, `
		SELECT to_char(dr.report_date,'YYYY-MM'), COALESCE(sum((elem->>'hours')::numeric),0)::float8
		FROM daily_reports dr, jsonb_array_elements(dr.delay_causes) elem
		WHERE dr.project_id=$1 AND dr.deleted_at IS NULL AND dr.status='Submitted'
		  AND dr.report_date BETWEEN $2 AND $3 GROUP BY 1`, pid, start, end); err == nil {
		for rows.Next() {
			var m string
			var v float64
			if rows.Scan(&m, &v) == nil && idx[m] != nil {
				idx[m].DelayHours = round2(v)
			}
		}
		rows.Close()
	}
	// İSG bulguları.
	if rows, err := h.pool.Query(ctx, `
		SELECT m, sum(o), sum(c) FROM (
		  SELECT to_char(created_at,'YYYY-MM') m, 1 o, 0 c FROM ohs_findings
		   WHERE project_id=$1 AND deleted_at IS NULL AND created_at >= $2 AND created_at < $3
		  UNION ALL
		  SELECT to_char(closed_at,'YYYY-MM'), 0, 1 FROM ohs_findings
		   WHERE project_id=$1 AND deleted_at IS NULL AND closed_at >= $2 AND closed_at < $3) x
		GROUP BY m`, pid, start, end.AddDate(0, 0, 1)); err == nil {
		for rows.Next() {
			var m string
			var o, c int
			if rows.Scan(&m, &o, &c) == nil && idx[m] != nil {
				idx[m].Opened, idx[m].Closed = o, c
			}
		}
		rows.Close()
	}
	// Olaylar: tamamlanan kilometre taşı, kesinleşen hakediş, en büyük gecikme, İSG.
	for i := range out {
		b := &out[i]
		for _, ms := range sn.Milestones {
			if len(ms.ActualDate) >= 7 && ms.ActualDate[:7] == b.Month && ms.Status == "Completed" {
				b.Events = append(b.Events, ms.Name+" tamamlandı")
			}
		}
		if b.NetPaid > 0 {
			b.Events = append(b.Events, fmt.Sprintf("Kesinleşen hakediş net %s", fmtMil(b.NetPaid)))
		}
		if b.DelayHours > 0 {
			b.Events = append(b.Events, trf(b.DelayHours, 1)+" saat gecikme kaydedildi")
		}
		if b.Opened+b.Closed > 0 {
			b.Events = append(b.Events, fmt.Sprintf("İSG: %d bulgu açıldı, %d kapatıldı", b.Opened, b.Closed))
		}
		if len(b.Events) > 4 {
			b.Events = b.Events[:4]
		}
	}
	return out
}

// openRisks — açık/izlenen riskler (skor sırasıyla, en çok 8).
func (h *Handler) openRisks(ctx context.Context, pid uuid.UUID) []RiskBrief {
	rows, err := h.pool.Query(ctx, `
		SELECT risk_no, title, probability, impact, status FROM project_risks
		WHERE project_id=$1 AND deleted_at IS NULL AND status <> 'Kapandı'
		ORDER BY probability*impact DESC, risk_no LIMIT 8`, pid)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []RiskBrief
	for rows.Next() {
		var r RiskBrief
		if rows.Scan(&r.No, &r.Title, &r.Prob, &r.Impact, &r.Status) == nil {
			out = append(out, r)
		}
	}
	return out
}

// periodDecisions — dönemde alınan kararlar (en çok 5, yeniden eskiye).
func (h *Handler) periodDecisions(ctx context.Context, pid uuid.UUID, start, end time.Time) []DecisionBrief {
	rows, err := h.pool.Query(ctx, `
		SELECT to_char(decided_on,'YYYY-MM-DD'), decision_text FROM project_decisions
		WHERE project_id=$1 AND decided_on BETWEEN $2 AND $3
		ORDER BY decided_on DESC, created_at DESC LIMIT 5`, pid, start, end)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DecisionBrief
	for rows.Next() {
		var d DecisionBrief
		if rows.Scan(&d.Date, &d.Text) == nil {
			out = append(out, d)
		}
	}
	return out
}

// nextQuarter — sonraki çeyrek: planlanan kümülatif ilerleme (PV / BAC) ve o
// çeyrekte planlı kilometre taşları.
func (h *Handler) nextQuarter(ctx context.Context, pid uuid.UUID, sn *MonthlySnapshot, end time.Time) *NextBrief {
	ns := end.AddDate(0, 0, 1)
	ne := ns.AddDate(0, 3, -1)
	nb := &NextBrief{Label: fmt.Sprintf("%d Q%d", ns.Year(), (int(ns.Month())-1)/3+1)}
	if p, ok := curveAt(sn.EVM.SCurve, end.Format("2006-01")); ok {
		nb.ProgressFrom = round2(pct(p.EV, sn.EVM.BAC))
	}
	if p, ok := curveAt(sn.EVM.SCurve, ne.Format("2006-01")); ok {
		nb.ProgressTo = round2(pct(p.PV, sn.EVM.BAC))
	}
	if nb.ProgressTo == 0 && nb.ProgressFrom == 0 {
		return nil
	}
	rows, err := h.pool.Query(ctx, `
		SELECT name, to_char(planned_date,'YYYY-MM-DD'), weight_pct, status
		FROM milestones
		WHERE project_id=$1 AND deleted_at IS NULL AND status <> 'Completed'
		  AND planned_date BETWEEN $2 AND $3
		ORDER BY planned_date LIMIT 4`, pid, ns, ne)
	if err == nil {
		for rows.Next() {
			var m MonthlyMilestone
			if rows.Scan(&m.Name, &m.PlannedDate, &m.WeightPct, &m.Status) == nil {
				nb.Milestones = append(nb.Milestones, m)
			}
		}
		rows.Close()
	}
	return nb
}
