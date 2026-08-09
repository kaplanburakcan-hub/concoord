package reports

// Haftalık İlerleme Raporu (Plan Faz 6):
//
//   1. POST /weekly-reports — API, haftanın verisini SENKRON derleyip snapshot
//      JSONB olarak dondurur (veri o anda sabitlenir; sonradan günlük rapor
//      revize edilse bile PDF/snapshot değişmez) ve 'weekly_report_pdf' işini
//      kuyruğa atar.
//   2. Worker (ProcessWeeklyPDF) snapshot'tan PDF'i çizer, MinIO'ya yükler,
//      files kaydı açar, weekly_reports'u Ready yapar ve üreteni bilgilendirir.
//
// Kabul kriteri karşılığı: "PDF rakamları snapshot'tan doğrulanabiliyor" —
// PDF'e giren HER sayı snapshot'ta vardır; GET ucu snapshot'ı aynen döner.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/storage"
)

// JobKindWeeklyPDF — job_queue 'kind' değeri (worker eşleştirmesi).
const JobKindWeeklyPDF = "weekly_report_pdf"

type weeklyJobPayload struct {
	WeeklyReportID string `json:"weekly_report_id"`
}

// ---------------------------------------------------------------------------
// Snapshot modeli — PDF yalnızca bu yapıdan çizilir.
// ---------------------------------------------------------------------------

type SnapDay struct {
	Date          string         `json:"date"`
	RevisionNo    int            `json:"revision_no"`
	Status        string         `json:"status"`
	TempMin       *float64       `json:"temperature_min,omitempty"`
	TempMax       *float64       `json:"temperature_max,omitempty"`
	WeatherCond   string         `json:"weather_condition,omitempty"`
	Notes         string         `json:"notes,omitempty"`
	ManpowerTotal int            `json:"manpower_total"`
	Manpower      []manpowerDTO  `json:"manpower,omitempty"`
	Equipment     []equipmentDTO `json:"equipment,omitempty"`
	WorkEntries   []workEntryDTO `json:"work_entries,omitempty"`
}

type SnapPayment struct {
	Subcontractor string `json:"subcontractor"`
	PeriodNo      int    `json:"period_no"`
	Status        string `json:"status"`
}

type SnapTotals struct {
	DaysReported   int     `json:"days_reported"`
	ManpowerDays   int     `json:"manpower_person_days"` // Σ headcount (adam-gün)
	EquipmentHours float64 `json:"equipment_hours"`
	WorkEntryCount int     `json:"work_entry_count"`
}

// SnapDeliveryItem — tek teslimat kaleminin özeti.
type SnapDeliveryItem struct {
	MaterialName string  `json:"material_name"`
	Unit         string  `json:"unit"`
	Qty          float64 `json:"qty"`
}

// SnapDelivery — haftada gelen bir irsaliye/teslimat.
type SnapDelivery struct {
	PONo        string             `json:"po_no"`
	Supplier    string             `json:"supplier"`
	NoteNo      string             `json:"delivery_note_no"`
	DeliveredAt string             `json:"delivered_at"`
	Items       []SnapDeliveryItem `json:"items"`
}

// SnapStockItem — hafta sonu stok + haftalık giriş/çıkış özeti.
type SnapStockItem struct {
	MalzemeAdi   string  `json:"malzeme_adi"`
	Kategori     string  `json:"kategori"`
	Birim        string  `json:"birim"`
	MevcutMiktar float64 `json:"mevcut_miktar"`
	MinStok      float64 `json:"min_stok"`
	WeekGiris    float64 `json:"week_giris"`
	WeekCikis    float64 `json:"week_cikis"`
	BelowMin     bool    `json:"below_min"`
}

// SnapPendingPO — teslim edilmemiş veya geciken sipariş.
type SnapPendingPO struct {
	PONo         string  `json:"po_no"`
	Supplier     string  `json:"supplier"`
	ExpectedDate *string `json:"expected_date,omitempty"`
	Overdue      bool    `json:"overdue"`
}

type Snapshot struct {
	GeneratedAt time.Time `json:"generated_at"`
	ProjectName string    `json:"project_name"`
	ProjectCode string    `json:"project_code"`
	WeekNo      int       `json:"week_no"`
	PeriodStart string    `json:"period_start"`
	PeriodEnd   string    `json:"period_end"`

	Days   []SnapDay  `json:"days"`
	Totals SnapTotals `json:"totals"`

	// Teslimat ve depo.
	Deliveries []SnapDelivery  `json:"deliveries"`
	Stock      []SnapStockItem `json:"stock"`

	// Bekleyen aktiviteler.
	PendingPayments []SnapPayment  `json:"pending_payments"`
	PendingPOs      []SnapPendingPO `json:"pending_pos"`
	OpenTasks       int             `json:"open_tasks"`
	TasksDueWeek    int             `json:"tasks_due_this_week"`
	PendingMARs     int             `json:"pending_mars"`

	// İSG özeti Faz 8'de eklenir:
	OHSNote string `json:"ohs_note,omitempty"`
}

// buildSnapshot — haftanın tüm verisini derler. Günler için TARİH BAŞINA EN
// YÜKSEK revizyon alınır (geçerli rapor).
func (h *Handler) buildSnapshot(ctx context.Context, pid uuid.UUID, start, end time.Time) (*Snapshot, error) {
	sn := &Snapshot{
		GeneratedAt:     time.Now().UTC(),
		PeriodStart:     start.Format("2006-01-02"),
		PeriodEnd:       end.Format("2006-01-02"),
		Days:            []SnapDay{},
		Deliveries:      []SnapDelivery{},
		Stock:           []SnapStockItem{},
		PendingPayments: []SnapPayment{},
		PendingPOs:      []SnapPendingPO{},
	}
	_, wk := start.ISOWeek()
	sn.WeekNo = wk

	if err := h.pool.QueryRow(ctx,
		`SELECT name, code FROM projects WHERE id=$1 AND deleted_at IS NULL`, pid).
		Scan(&sn.ProjectName, &sn.ProjectCode); err != nil {
		return nil, fmt.Errorf("proje okunamadı: %w", err)
	}

	// Günlük raporlar (geçerli revizyonlar).
	rows, err := h.pool.Query(ctx, drSelect+`
		WHERE dr.project_id=$1 AND dr.deleted_at IS NULL
		  AND dr.report_date BETWEEN $2::date AND $3::date
		  AND dr.revision_no = (
		      SELECT max(x.revision_no) FROM daily_reports x
		      WHERE x.project_id=dr.project_id AND x.report_date=dr.report_date
		        AND x.deleted_at IS NULL)
		ORDER BY dr.report_date`,
		pid, sn.PeriodStart, sn.PeriodEnd)
	if err != nil {
		return nil, err
	}
	var days []dailyDTO
	for rows.Next() {
		var d dailyDTO
		if err := scanDaily(rows, &d); err != nil {
			rows.Close()
			return nil, err
		}
		days = append(days, d)
	}
	rows.Close()

	for i := range days {
		if err := h.loadLines(ctx, &days[i]); err != nil {
			return nil, err
		}
		d := days[i]
		sd := SnapDay{
			Date: d.ReportDate, RevisionNo: d.RevisionNo, Status: d.Status,
			TempMin: d.TempMin, TempMax: d.TempMax,
			Manpower: d.Manpower, Equipment: d.Equipment, WorkEntries: d.WorkEntries,
		}
		if d.Notes != nil {
			sd.Notes = *d.Notes
		}
		if len(d.Weather) > 0 {
			var wjs struct {
				Condition string `json:"condition"`
			}
			_ = json.Unmarshal(d.Weather, &wjs)
			sd.WeatherCond = wjs.Condition
		}
		for _, m := range d.Manpower {
			sd.ManpowerTotal += m.Headcount
		}
		sn.Days = append(sn.Days, sd)

		sn.Totals.DaysReported++
		sn.Totals.ManpowerDays += sd.ManpowerTotal
		sn.Totals.WorkEntryCount += len(d.WorkEntries)
		for _, e := range d.Equipment {
			if e.WorkingHours != nil {
				sn.Totals.EquipmentHours += *e.WorkingHours
			}
		}
	}

	// Bekleyen hakediş statüleri (finansal TUTAR yok — view_financials ayrımı
	// gereği haftalık raporda yalnızca statü özetlenir; tutarlar aylık EVM
	// raporunda, Faz 9).
	prow, err := h.pool.Query(ctx, `
		SELECT s.company_name, pp.period_no, pp.status
		FROM progress_payments pp
		JOIN subcontractors s ON s.id = pp.subcontractor_id
		WHERE pp.project_id=$1 AND pp.deleted_at IS NULL
		  AND pp.status IN ('Draft','Submitted','SiteApproved')
		ORDER BY s.company_name, pp.period_no`, pid)
	if err != nil {
		return nil, err
	}
	for prow.Next() {
		var p SnapPayment
		if err := prow.Scan(&p.Subcontractor, &p.PeriodNo, &p.Status); err != nil {
			prow.Close()
			return nil, err
		}
		sn.PendingPayments = append(sn.PendingPayments, p)
	}
	prow.Close()

	// Görev özeti.
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status <> 'Done'),
		       count(*) FILTER (WHERE status <> 'Done' AND due_date BETWEEN $2::date AND $3::date)
		FROM tasks WHERE project_id=$1 AND deleted_at IS NULL`,
		pid, sn.PeriodStart, sn.PeriodEnd).Scan(&sn.OpenTasks, &sn.TasksDueWeek); err != nil {
		return nil, err
	}

	// Bekleyen MAR sayısı.
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FROM material_approvals
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND status IN ('Submitted','UnderReview')`, pid).Scan(&sn.PendingMARs); err != nil {
		return nil, err
	}

	// ── Teslimatlar (irsaliye) ─────────────────────────────────────────────────
	drows, err := h.pool.Query(ctx, `
		SELECT o.po_no, o.supplier_name,
		       d.delivery_note_no, TO_CHAR(d.delivered_at,'YYYY-MM-DD'), d.id
		FROM deliveries d
		JOIN purchase_orders o ON o.id = d.po_id
		WHERE o.project_id=$1
		  AND d.delivered_at BETWEEN $2::date AND $3::date
		ORDER BY d.delivered_at, d.created_at`,
		pid, sn.PeriodStart, sn.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("teslimatlar okunamadı: %w", err)
	}
	for drows.Next() {
		var sd SnapDelivery
		var delivID string
		if err := drows.Scan(&sd.PONo, &sd.Supplier, &sd.NoteNo, &sd.DeliveredAt, &delivID); err != nil {
			drows.Close()
			return nil, err
		}
		// Teslimat kalemleri.
		irows, err := h.pool.Query(ctx, `
			SELECT material_name, unit, delivered_qty::float8
			FROM delivery_items WHERE delivery_id=$1 ORDER BY created_at`, delivID)
		if err != nil {
			drows.Close()
			return nil, err
		}
		for irows.Next() {
			var it SnapDeliveryItem
			if err := irows.Scan(&it.MaterialName, &it.Unit, &it.Qty); err != nil {
				irows.Close()
				drows.Close()
				return nil, err
			}
			sd.Items = append(sd.Items, it)
		}
		irows.Close()
		sn.Deliveries = append(sn.Deliveries, sd)
	}
	drows.Close()

	// ── Depo stok ve haftalık hareketler ──────────────────────────────────────
	srows, err := h.pool.Query(ctx, `
		SELECT i.malzeme_adi, i.kategori, i.birim,
		       i.mevcut_miktar::float8, i.min_stok::float8,
		       COALESCE(SUM(m.miktar::float8) FILTER (WHERE m.hareket_turu='giris'),  0),
		       COALESCE(SUM(m.miktar::float8) FILTER (WHERE m.hareket_turu='cikis'),  0)
		FROM site_warehouse_items i
		LEFT JOIN site_warehouse_movements m
		       ON m.item_id = i.id
		      AND m.tarih BETWEEN $2::date AND $3::date
		WHERE i.project_id=$1
		GROUP BY i.id, i.malzeme_adi, i.kategori, i.birim, i.mevcut_miktar, i.min_stok
		ORDER BY i.kategori, i.sira, i.malzeme_adi`,
		pid, sn.PeriodStart, sn.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("stok okunamadı: %w", err)
	}
	for srows.Next() {
		var st SnapStockItem
		if err := srows.Scan(&st.MalzemeAdi, &st.Kategori, &st.Birim,
			&st.MevcutMiktar, &st.MinStok, &st.WeekGiris, &st.WeekCikis); err != nil {
			srows.Close()
			return nil, err
		}
		st.BelowMin = st.MevcutMiktar < st.MinStok
		sn.Stock = append(sn.Stock, st)
	}
	srows.Close()

	// ── Bekleyen / geciken siparişler (PO) ────────────────────────────────────
	prows, err := h.pool.Query(ctx, `
		SELECT po_no, supplier_name,
		       TO_CHAR(expected_date,'YYYY-MM-DD'),
		       (expected_date IS NOT NULL AND expected_date < $2::date)
		FROM purchase_orders
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND status IN ('Ordered','PartiallyDelivered')
		ORDER BY expected_date NULLS LAST, po_no`,
		pid, sn.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("siparişler okunamadı: %w", err)
	}
	for prows.Next() {
		var p SnapPendingPO
		if err := prows.Scan(&p.PONo, &p.Supplier, &p.ExpectedDate, &p.Overdue); err != nil {
			prows.Close()
			return nil, err
		}
		sn.PendingPOs = append(sn.PendingPOs, p)
	}
	prows.Close()

	sn.OHSNote = "İSG özeti Faz 8 ile eklenecektir."
	return sn, nil
}

// ---------------------------------------------------------------------------
// HTTP uçları
// ---------------------------------------------------------------------------

type weeklyDTO struct {
	ID          uuid.UUID  `json:"id"`
	WeekNo      int        `json:"week_no"`
	PeriodStart string     `json:"period_start"`
	PeriodEnd   string     `json:"period_end"`
	Status      string     `json:"status"`
	Error       *string    `json:"error,omitempty"`
	GeneratedBy string     `json:"generated_by_name"`
	HasPDF      bool       `json:"has_pdf"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListWeekly — üretilen haftalık raporlar (snapshot hariç; hafif liste).
func (h *Handler) ListWeekly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT wr.id, wr.week_no, to_char(wr.period_start,'YYYY-MM-DD'),
		       to_char(wr.period_end,'YYYY-MM-DD'), wr.status, wr.error,
		       u.full_name, wr.generated_pdf_file_id IS NOT NULL, wr.created_at
		FROM weekly_reports wr
		JOIN users u ON u.id = wr.generated_by
		WHERE wr.project_id=$1 AND wr.deleted_at IS NULL
		ORDER BY wr.period_start DESC, wr.created_at DESC
		LIMIT 100`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []weeklyDTO{}
	for rows.Next() {
		var d weeklyDTO
		if err := rows.Scan(&d.ID, &d.WeekNo, &d.PeriodStart, &d.PeriodEnd,
			&d.Status, &d.Error, &d.GeneratedBy, &d.HasPDF, &d.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"weekly_reports": out})
}

// GetWeekly — snapshot dahil tekil kayıt (PDF doğrulaması için).
func (h *Handler) GetWeekly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var d weeklyDTO
	var snapshot json.RawMessage
	err := h.pool.QueryRow(r.Context(), `
		SELECT wr.id, wr.week_no, to_char(wr.period_start,'YYYY-MM-DD'),
		       to_char(wr.period_end,'YYYY-MM-DD'), wr.status, wr.error,
		       u.full_name, wr.generated_pdf_file_id IS NOT NULL, wr.created_at, wr.snapshot
		FROM weekly_reports wr
		JOIN users u ON u.id = wr.generated_by
		WHERE wr.id=$1 AND wr.project_id=$2 AND wr.deleted_at IS NULL`, id, pid).
		Scan(&d.ID, &d.WeekNo, &d.PeriodStart, &d.PeriodEnd, &d.Status, &d.Error,
			&d.GeneratedBy, &d.HasPDF, &d.CreatedAt, &snapshot)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Haftalık rapor bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"weekly_report": d,
		"snapshot":      snapshot,
	})
}

// GenerateWeekly — snapshot'ı derler, kaydı Pending açar, PDF işini kuyruğa
// atar. Girdi: {"period_start":"YYYY-MM-DD"} (haftanın herhangi bir günü de
// olabilir; Pazartesi'ye yuvarlanır).
func (h *Handler) GenerateWeekly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		PeriodStart string `json:"period_start"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	day, err := time.Parse("2006-01-02", strings.TrimSpace(in.PeriodStart))
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"period_start": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	// ISO haftası: Pazartesi..Pazar'a yuvarla.
	wd := int(day.Weekday())
	if wd == 0 {
		wd = 7
	}
	start := day.AddDate(0, 0, -(wd - 1))
	end := start.AddDate(0, 0, 6)

	sn, err := h.buildSnapshot(r.Context(), pid, start, end)
	if err != nil {
		h.log.Error("haftalık snapshot derlenemedi", "err", err, "project", pid)
		httpx.Internal(w, r)
		return
	}
	snapJSON, err := json.Marshal(sn)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO weekly_reports (project_id, week_no, period_start, period_end,
		                            status, snapshot, generated_by)
		VALUES ($1,$2,$3::date,$4::date,'Ready',$5,$6)
		RETURNING id`,
		pid, sn.WeekNo, sn.PeriodStart, sn.PeriodEnd, snapJSON, uid).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	payload, _ := json.Marshal(weeklyJobPayload{WeeklyReportID: id.String()})
	if err := notify.Enqueue(r.Context(), h.pool, JobKindWeeklyPDF, payload); err != nil {
		h.log.Error("haftalık PDF işi kuyruğa atılamadı", "err", err, "weekly", id)
	}

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "weekly_reports", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"week_no": sn.WeekNo, "period_start": sn.PeriodStart,
			"days_reported": sn.Totals.DaysReported},
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id": id, "status": "Ready",
		"week_no": sn.WeekNo, "period_start": sn.PeriodStart, "period_end": sn.PeriodEnd,
	})
}

// DownloadWeekly — üretilen PDF'i MinIO'dan akıtır.
func (h *Handler) DownloadWeekly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var key, name string
	err := h.pool.QueryRow(r.Context(), `
		SELECT f.storage_key, f.original_name
		FROM weekly_reports wr
		JOIN files f ON f.id = wr.generated_pdf_file_id
		WHERE wr.id=$1 AND wr.project_id=$2 AND wr.deleted_at IS NULL
		  AND wr.status='Ready'`, id, pid).Scan(&key, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound,
			"PDF henüz hazır değil ya da rapor bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	obj, err := h.store.GetObject(r.Context(), key)
	if err != nil {
		h.log.Error("haftalık PDF okunamadı", "err", err, "key", key)
		httpx.Internal(w, r)
		return
	}
	defer obj.Body.Close()
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	_, _ = io.Copy(w, obj.Body)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Worker tarafı — PDF üretimi (cmd/worker bu fonksiyonu çağırır)
// ---------------------------------------------------------------------------

// ProcessWeeklyPDF — snapshot'tan PDF üretir, depoya yükler, kaydı Ready yapar.
// Dönen error worker'ın yeniden deneme mekanizmasını tetikler; kalıcı hatalar
// (kayıt yok, snapshot bozuk) nil döner ve kayıt Failed işaretlenir.
func ProcessWeeklyPDF(ctx context.Context, pool *pgxpool.Pool, store *storage.Client,
	nt *notify.Service, log *slog.Logger, payload json.RawMessage) error {

	var p weeklyJobPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		log.Warn("haftalık PDF işi: bozuk payload, atlanıyor", "err", err)
		return nil
	}
	id, err := uuid.Parse(p.WeeklyReportID)
	if err != nil {
		log.Warn("haftalık PDF işi: geçersiz id, atlanıyor", "id", p.WeeklyReportID)
		return nil
	}

	var pid, genBy uuid.UUID
	var snapJSON []byte
	var hasPDF bool
	err = pool.QueryRow(ctx, `
		SELECT project_id, generated_by, snapshot,
		       (generated_pdf_file_id IS NOT NULL)
		FROM weekly_reports
		WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&pid, &genBy, &snapJSON, &hasPDF)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Warn("haftalık PDF işi: kayıt yok, atlanıyor", "id", id)
		return nil
	}
	if err != nil {
		return err // geçici DB hatası → yeniden dene
	}
	if hasPDF {
		return nil // idempotentlik: PDF zaten üretilmiş
	}

	var sn Snapshot
	if err := json.Unmarshal(snapJSON, &sn); err != nil {
		markWeeklyFailed(ctx, pool, nt, log, id, pid, genBy, "snapshot çözümlenemedi")
		return nil
	}

	pdf := BuildWeeklyPDF(sn)
	key := fmt.Sprintf("project/%s/reports/weekly/%s.pdf", pid, id)
	sha := sha256Hex(pdf)
	if err := store.PutObject(ctx, key, "application/pdf",
		bytes.NewReader(pdf), int64(len(pdf)), sha); err != nil {
		return fmt.Errorf("PDF depoya yüklenemedi: %w", err) // geçici → yeniden dene
	}

	fname := fmt.Sprintf("haftalik-rapor-%s-H%02d.pdf", sn.ProjectCode, sn.WeekNo)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var fileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO files (storage_key, original_name, mime, size_bytes, uploaded_by)
		VALUES ($1,$2,'application/pdf',$3,$4) RETURNING id`,
		key, fname, len(pdf), genBy).Scan(&fileID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE weekly_reports
		SET status='Ready', generated_pdf_file_id=$2, error=NULL, row_version=row_version+1
		WHERE id=$1`, id, fileID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	nt.Send(ctx, notify.Input{
		UserIDs: []uuid.UUID{genBy},
		Type:    notify.TypeWeeklyReportReady,
		Title:   fmt.Sprintf("Haftalık rapor hazır: %s / Hafta %d", sn.ProjectCode, sn.WeekNo),
		Body:    fmt.Sprintf("%s – %s dönemi PDF raporu indirilebilir.", sn.PeriodStart, sn.PeriodEnd),
		EntityType: "weekly_reports", EntityID: &id, ProjectID: &pid,
	})
	log.Info("haftalık PDF üretildi", "weekly", id, "bytes", len(pdf))
	return nil
}

func markWeeklyFailed(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service,
	log *slog.Logger, id, pid, genBy uuid.UUID, reason string) {
	if _, err := pool.Exec(ctx, `
		UPDATE weekly_reports SET status='Failed', error=$2, row_version=row_version+1
		WHERE id=$1`, id, reason); err != nil {
		log.Error("haftalık rapor Failed işaretlenemedi", "id", id, "err", err)
	}
	nt.Send(ctx, notify.Input{
		UserIDs: []uuid.UUID{genBy},
		Type:    notify.TypeWeeklyReportFailed,
		Title:   "Haftalık rapor üretilemedi",
		Body:    reason,
		EntityType: "weekly_reports", EntityID: &id, ProjectID: &pid,
	})
}
