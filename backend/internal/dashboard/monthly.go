package dashboard

// monthly.go — Aylık Yönetim Raporu (Plan §7 aylık kontrol ritmi + Faz 9).
// weekly_reports (Faz 6) ile birebir aynı desen:
//   1. POST /monthly-reports — ayın verisi SENKRON derlenip snapshot JSONB
//      olarak dondurulur; 'monthly_report_pdf' işi kuyruğa atılır.
//   2. Worker (ProcessMonthlyPDF) snapshot'tan PDF'i çizer, MinIO'ya yükler,
//      kaydı Ready yapar, üreteni bilgilendirir.
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

// JobKindMonthlyPDF — job_queue 'kind' değeri (worker eşleştirmesi).
const JobKindMonthlyPDF = "monthly_report_pdf"

type monthlyJobPayload struct {
	MonthlyReportID string `json:"monthly_report_id"`
}

// ---------------------------------------------------------------------------
// Snapshot modeli — PDF yalnızca bu yapıdan çizilir.
// ---------------------------------------------------------------------------

type MonthlySubPayment struct {
	Subcontractor string  `json:"subcontractor"`
	PeriodNo      int     `json:"period_no"`
	GrossThis     float64 `json:"gross_this"`
	Deductions    float64 `json:"total_deductions"`
	NetPayable    float64 `json:"net_payable"`
}

type MonthlyMilestone struct {
	Name        string   `json:"name"`
	PlannedDate string   `json:"planned_date,omitempty"`
	ActualDate  string   `json:"actual_date,omitempty"`
	WeightPct   *float64 `json:"weight_pct,omitempty"`
	Status      string   `json:"status"`
	Late        bool     `json:"late"`
}

type MonthlySnapshot struct {
	GeneratedAt time.Time `json:"generated_at"`
	ProjectName string    `json:"project_name"`
	ProjectCode string    `json:"project_code"`
	Currency    string    `json:"currency"`
	Kind        string    `json:"kind"`  // Monthly | Quarterly | Custom
	Label       string    `json:"label"` // "2026-03", "2026 Q2", "01.03.2026 - 15.05.2026"
	Year        int       `json:"year"`
	Month       int       `json:"month"`
	PeriodStart string    `json:"period_start"`
	PeriodEnd   string    `json:"period_end"`

	// EVM (Plan §7: PV, EV, AC → SPI, CPI, EAC/ETC).
	EVM EVMResult `json:"evm"`

	// Finansal: ay içinde kesinleşen hakedişler + kesinti dökümü + bütçe sapması.
	FinalizedPayments []MonthlySubPayment `json:"finalized_payments"`
	MonthGross        float64             `json:"month_gross"`
	MonthDeductions   float64             `json:"month_deductions"`
	MonthNet          float64             `json:"month_net"`
	DeductionsByType  map[string]float64  `json:"deductions_by_type"`
	BudgetVariance    float64             `json:"budget_variance"` // BAC − EAC

	// Fiziki: milestone gerçekleşme (Plan §7 aylık fiziki kontrol).
	Milestones []MonthlyMilestone `json:"milestones"`

	// İSG performansı: bulgu/kapama, ceza istatistiği.
	OHS struct {
		FindingsOpened int     `json:"findings_opened"`
		FindingsClosed int     `json:"findings_closed"`
		OpenTotal      int     `json:"open_total"`
		OpenCritical   int     `json:"open_critical"`
		PenaltiesCount int     `json:"penalties_count"`
		PenaltiesTotal float64 `json:"penalties_total"`
	} `json:"ohs"`

	// Gecikme nedenleri (E16): ay içindeki gönderilmiş günlük raporların
	// yapılandırılmış gecikme kayıtları, nedene göre toplanır.
	DelayCauses []MonthlyDelayCause `json:"delay_causes"`

	// Tedarik.
	Procurement struct {
		POsOrdered   int `json:"pos_ordered"`
		POsDelivered int `json:"pos_delivered"`
		POsOverdue   int `json:"pos_overdue"`
		PRsPending   int `json:"prs_pending"`
	} `json:"procurement"`
}

// MonthlyDelayCause — bir gecikme nedeninin ay içindeki toplamı.
type MonthlyDelayCause struct {
	ReasonCode string  `json:"reason_code"`
	Label      string  `json:"label"`
	Hours      float64 `json:"hours"`
	Reports    int     `json:"reports"` // bu nedenin geçtiği farklı rapor sayısı
}

// delayReasonTR — gecikme nedeni kodunun Türkçe etiketi (rapor ekranıyla aynı).
var delayReasonTR = map[string]string{
	"Hava": "Hava koşulları", "Sahip": "Sahip / müşteri kaynaklı",
	"Tasarim": "Tasarım / proje değişikliği", "Taseron": "Taşeron kaynaklı",
	"Malzeme": "Malzeme tedarik", "Ekipman": "Ekipman arızası",
	"Idari": "İdari / resmi işlem", "Diger": "Diğer",
}

// Dönem türleri (monthly_reports.kind).
const (
	KindMonthly   = "Monthly"
	KindQuarterly = "Quarterly"
	KindCustom    = "Custom"
)

// maxCustomPeriodDays — özel dönem raporunun en uzun aralığı (5 yıl).
const maxCustomPeriodDays = 1827

// periodLabel — dönemin kullanıcıya gösterilen kısa adı.
func periodLabel(kind string, start, end time.Time) string {
	switch kind {
	case KindMonthly:
		return start.Format("2006-01")
	case KindQuarterly:
		return fmt.Sprintf("%d Q%d", start.Year(), (int(start.Month())-1)/3+1)
	}
	return start.Format("02.01.2006") + " - " + end.Format("02.01.2006")
}

// buildMonthlySnapshot — dönemin (start..end, ikisi de dahil) tüm verisini
// derler (Plan Faz 9: EVM + finansal + İSG + tedarik tek raporda).
func (h *Handler) buildMonthlySnapshot(ctx context.Context, pid uuid.UUID, kind string, start, end time.Time) (*MonthlySnapshot, error) {
	endEx := end.AddDate(0, 0, 1) // dışlayıcı üst sınır
	sn := &MonthlySnapshot{
		GeneratedAt: time.Now().UTC(),
		Kind:        kind,
		Label:       periodLabel(kind, start, end),
		Year:        start.Year(), Month: int(start.Month()),
		PeriodStart: start.Format("2006-01-02"),
		PeriodEnd:   end.Format("2006-01-02"),
	}
	if err := h.pool.QueryRow(ctx,
		`SELECT name, code, currency FROM projects WHERE id=$1 AND deleted_at IS NULL`, pid).
		Scan(&sn.ProjectName, &sn.ProjectCode, &sn.Currency); err != nil {
		return nil, fmt.Errorf("proje okunamadı: %w", err)
	}

	// EVM — kümülatifler RAPOR AYININ SONUNA göre kesilir (geriye dönük rapor
	// üretiminde o ayın durumu dondurulur).
	evm, err := LoadEVM(ctx, h.pool, pid, end)
	if err != nil {
		return nil, err
	}
	sn.EVM = *evm
	sn.BudgetVariance = round2(evm.BAC - evm.EAC)

	// Ay içinde kesinleşen hakedişler.
	rows, err := h.pool.Query(ctx, `
		SELECT s.company_name, pp.period_no, pp.gross_this, pp.total_deductions, pp.net_payable
		FROM progress_payments pp
		JOIN subcontractors s ON s.id = pp.subcontractor_id
		WHERE pp.project_id=$1 AND pp.deleted_at IS NULL AND pp.status='Finalized'
		  AND pp.finalized_at >= $2 AND pp.finalized_at < $3
		ORDER BY s.company_name, pp.period_no`,
		pid, start, endEx)
	if err != nil {
		return nil, err
	}
	sn.FinalizedPayments = []MonthlySubPayment{}
	for rows.Next() {
		var p MonthlySubPayment
		if err := rows.Scan(&p.Subcontractor, &p.PeriodNo, &p.GrossThis,
			&p.Deductions, &p.NetPayable); err != nil {
			rows.Close()
			return nil, err
		}
		sn.FinalizedPayments = append(sn.FinalizedPayments, p)
		sn.MonthGross = round2(sn.MonthGross + p.GrossThis)
		sn.MonthDeductions = round2(sn.MonthDeductions + p.Deductions)
		sn.MonthNet = round2(sn.MonthNet + p.NetPayable)
	}
	rows.Close()

	// Kesinti dökümü (tip bazında; ay içinde kesinleşen hakedişlerin kalemleri).
	sn.DeductionsByType = map[string]float64{}
	rows, err = h.pool.Query(ctx, `
		SELECT pd.type, COALESCE(sum(pd.amount),0)
		FROM payment_deductions pd
		JOIN progress_payments pp ON pp.id = pd.progress_payment_id
		WHERE pp.project_id=$1 AND pp.deleted_at IS NULL AND pp.status='Finalized'
		  AND pp.finalized_at >= $2 AND pp.finalized_at < $3
		GROUP BY pd.type`, pid, start, endEx)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t string
		var v float64
		if err := rows.Scan(&t, &v); err != nil {
			rows.Close()
			return nil, err
		}
		sn.DeductionsByType[t] = round2(v)
	}
	rows.Close()

	// Milestone gerçekleşme.
	rows, err = h.pool.Query(ctx, `
		SELECT name, COALESCE(to_char(planned_date,'YYYY-MM-DD'),''),
		       COALESCE(to_char(actual_date,'YYYY-MM-DD'),''), weight_pct, status,
		       (planned_date IS NOT NULL AND planned_date < $2::date AND status <> 'Completed')
		FROM milestones WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY sort_order, planned_date NULLS LAST`, pid, endEx)
	if err != nil {
		return nil, err
	}
	sn.Milestones = []MonthlyMilestone{}
	for rows.Next() {
		var m MonthlyMilestone
		if err := rows.Scan(&m.Name, &m.PlannedDate, &m.ActualDate,
			&m.WeightPct, &m.Status, &m.Late); err != nil {
			rows.Close()
			return nil, err
		}
		sn.Milestones = append(sn.Milestones, m)
	}
	rows.Close()

	// İSG performansı.
	if err := h.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM ohs_findings WHERE project_id=$1 AND deleted_at IS NULL
		     AND created_at >= $2 AND created_at < $3),
		  (SELECT count(*) FROM ohs_findings WHERE project_id=$1 AND deleted_at IS NULL
		     AND closed_at  >= $2 AND closed_at  < $3),
		  (SELECT count(*) FROM ohs_findings WHERE project_id=$1 AND deleted_at IS NULL
		     AND status IN ('Open','InProgress')),
		  (SELECT count(*) FROM ohs_findings WHERE project_id=$1 AND deleted_at IS NULL
		     AND status IN ('Open','InProgress') AND severity='Critical'),
		  (SELECT count(*) FROM ohs_penalties WHERE project_id=$1 AND deleted_at IS NULL
		     AND created_at >= $2 AND created_at < $3),
		  (SELECT COALESCE(sum(amount),0) FROM ohs_penalties WHERE project_id=$1 AND deleted_at IS NULL
		     AND created_at >= $2 AND created_at < $3 AND amount IS NOT NULL)`,
		pid, start, endEx).
		Scan(&sn.OHS.FindingsOpened, &sn.OHS.FindingsClosed, &sn.OHS.OpenTotal,
			&sn.OHS.OpenCritical, &sn.OHS.PenaltiesCount, &sn.OHS.PenaltiesTotal); err != nil {
		return nil, err
	}

	// Tedarik.
	if err := h.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM purchase_orders WHERE project_id=$1 AND deleted_at IS NULL
		     AND created_at >= $2 AND created_at < $3),
		  (SELECT count(*) FROM purchase_orders po WHERE po.project_id=$1 AND po.deleted_at IS NULL
		     AND po.status='Delivered'
		     AND (SELECT max(d.delivered_at) FROM deliveries d
		          WHERE d.po_id=po.id AND d.deleted_at IS NULL) >= $2
		     AND (SELECT max(d.delivered_at) FROM deliveries d
		          WHERE d.po_id=po.id AND d.deleted_at IS NULL) < $3),
		  (SELECT count(*) FROM purchase_orders WHERE project_id=$1 AND deleted_at IS NULL
		     AND status IN ('Ordered','PartiallyDelivered')
		     AND expected_date IS NOT NULL AND expected_date < CURRENT_DATE),
		  (SELECT count(*) FROM purchase_requests WHERE project_id=$1 AND deleted_at IS NULL
		     AND status='Submitted')`,
		pid, start, endEx).
		Scan(&sn.Procurement.POsOrdered, &sn.Procurement.POsDelivered,
			&sn.Procurement.POsOverdue, &sn.Procurement.PRsPending); err != nil {
		return nil, err
	}
	// Gecikme nedenleri: yalnızca gönderilmiş (Submitted) günlük raporlar sayılır.
	drows, err := h.pool.Query(ctx, `
		SELECT elem->>'reason_code', sum((elem->>'hours')::numeric)::float8, count(DISTINCT dr.id)
		FROM daily_reports dr, jsonb_array_elements(dr.delay_causes) elem
		WHERE dr.project_id=$1 AND dr.deleted_at IS NULL AND dr.status='Submitted'
		  AND dr.report_date BETWEEN $2 AND $3
		GROUP BY elem->>'reason_code'
		ORDER BY sum((elem->>'hours')::numeric) DESC`, pid, start, end)
	if err != nil {
		return nil, err
	}
	defer drows.Close()
	for drows.Next() {
		var c MonthlyDelayCause
		if err := drows.Scan(&c.ReasonCode, &c.Hours, &c.Reports); err != nil {
			return nil, err
		}
		c.Label = delayReasonTR[c.ReasonCode]
		if c.Label == "" {
			c.Label = c.ReasonCode
		}
		sn.DelayCauses = append(sn.DelayCauses, c)
	}
	if err := drows.Err(); err != nil {
		return nil, err
	}
	return sn, nil
}

// ---------------------------------------------------------------------------
// HTTP uçları
// ---------------------------------------------------------------------------

type monthlyDTO struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Year        int       `json:"year"`
	Month       int       `json:"month"`
	PeriodStart string    `json:"period_start"`
	PeriodEnd   string    `json:"period_end"`
	Status      string    `json:"status"`
	Error       *string   `json:"error,omitempty"`
	GeneratedBy string    `json:"generated_by_name"`
	HasPDF      bool      `json:"has_pdf"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *Handler) ListMonthly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT mr.id, mr.kind, mr.year, mr.month, to_char(mr.period_start,'YYYY-MM-DD'),
		       to_char(mr.period_end,'YYYY-MM-DD'), mr.status, mr.error,
		       u.full_name, mr.generated_pdf_file_id IS NOT NULL, mr.created_at
		FROM monthly_reports mr
		JOIN users u ON u.id = mr.generated_by
		WHERE mr.project_id=$1 AND mr.deleted_at IS NULL
		ORDER BY mr.period_start DESC, mr.created_at DESC
		LIMIT 100`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []monthlyDTO{}
	for rows.Next() {
		var d monthlyDTO
		if err := rows.Scan(&d.ID, &d.Kind, &d.Year, &d.Month, &d.PeriodStart, &d.PeriodEnd,
			&d.Status, &d.Error, &d.GeneratedBy, &d.HasPDF, &d.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"monthly_reports": out})
}

// GetMonthly — snapshot dahil (PDF rakamları buradan doğrulanır).
func (h *Handler) GetMonthly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var d monthlyDTO
	var snapshot json.RawMessage
	err := h.pool.QueryRow(r.Context(), `
		SELECT mr.id, mr.kind, mr.year, mr.month, to_char(mr.period_start,'YYYY-MM-DD'),
		       to_char(mr.period_end,'YYYY-MM-DD'), mr.status, mr.error,
		       u.full_name, mr.generated_pdf_file_id IS NOT NULL, mr.created_at, mr.snapshot
		FROM monthly_reports mr
		JOIN users u ON u.id = mr.generated_by
		WHERE mr.id=$1 AND mr.project_id=$2 AND mr.deleted_at IS NULL`, id, pid).
		Scan(&d.ID, &d.Kind, &d.Year, &d.Month, &d.PeriodStart, &d.PeriodEnd, &d.Status,
			&d.Error, &d.GeneratedBy, &d.HasPDF, &d.CreatedAt, &snapshot)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Aylık rapor bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"monthly_report": d,
		"snapshot":       snapshot,
	})
}

// GenerateMonthly — snapshot'ı derler, kaydı Pending açar, PDF işini kuyruğa
// atar. Girdi: {"month":"YYYY-MM"}.
func (h *Handler) GenerateMonthly(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Kind    string `json:"kind"`    // monthly | quarterly | custom (boş = monthly)
		Month   string `json:"month"`   // monthly: YYYY-MM
		Year    int    `json:"year"`    // quarterly
		Quarter int    `json:"quarter"` // quarterly: 1..4
		Start   string `json:"start"`   // custom: YYYY-MM-DD
		End     string `json:"end"`     // custom: YYYY-MM-DD
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	var kind string
	var start, end time.Time
	switch strings.ToLower(strings.TrimSpace(in.Kind)) {
	case "", "monthly":
		kind = KindMonthly
		t, err := time.Parse("2006-01", strings.TrimSpace(in.Month))
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"month": "geçerli bir ay (YYYY-MM) girin"})
			return
		}
		start = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 1, -1)
	case "quarterly":
		kind = KindQuarterly
		errs := map[string]string{}
		if in.Year < 2000 || in.Year > 2100 {
			errs["year"] = "geçerli bir yıl girin"
		}
		if in.Quarter < 1 || in.Quarter > 4 {
			errs["quarter"] = "çeyrek 1 ile 4 arasında olmalı"
		}
		if len(errs) > 0 {
			httpx.ValidationFailed(w, r, errs)
			return
		}
		start = time.Date(in.Year, time.Month((in.Quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, 3, -1)
	case "custom":
		kind = KindCustom
		errs := map[string]string{}
		s, e1 := time.Parse("2006-01-02", strings.TrimSpace(in.Start))
		e, e2 := time.Parse("2006-01-02", strings.TrimSpace(in.End))
		if e1 != nil {
			errs["start"] = "geçerli bir başlangıç tarihi girin"
		}
		if e2 != nil {
			errs["end"] = "geçerli bir bitiş tarihi girin"
		}
		if len(errs) == 0 && e.Before(s) {
			errs["end"] = "bitiş tarihi başlangıçtan önce olamaz"
		}
		if len(errs) == 0 && int(e.Sub(s).Hours()/24) >= maxCustomPeriodDays {
			errs["end"] = "özel dönem en fazla 5 yıl olabilir"
		}
		if len(errs) > 0 {
			httpx.ValidationFailed(w, r, errs)
			return
		}
		start, end = s, e
	default:
		httpx.ValidationFailed(w, r, map[string]string{"kind": "rapor türü monthly, quarterly veya custom olmalı"})
		return
	}

	sn, err := h.buildMonthlySnapshot(r.Context(), pid, kind, start, end)
	if err != nil {
		h.log.Error("dönem snapshot'ı derlenemedi", "err", err, "project", pid)
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
		INSERT INTO monthly_reports (project_id, kind, year, month, period_start, period_end,
		                             status, snapshot, generated_by)
		VALUES ($1,$2,$3,$4,$5::date,$6::date,'Pending',$7,$8)
		RETURNING id`,
		pid, kind, sn.Year, sn.Month, sn.PeriodStart, sn.PeriodEnd, snapJSON, uid).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	payload, _ := json.Marshal(monthlyJobPayload{MonthlyReportID: id.String()})
	if err := notify.Enqueue(r.Context(), h.pool, JobKindMonthlyPDF, payload); err != nil {
		h.log.Error("aylık PDF işi kuyruğa atılamadı", "err", err, "monthly", id)
	}

	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "monthly_reports", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"kind": kind, "period": sn.Label,
			"spi": sn.EVM.SPI, "cpi": sn.EVM.CPI},
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusAccepted, map[string]interface{}{
		"id": id, "status": "Pending", "kind": kind, "label": sn.Label,
	})
}

// DownloadMonthly — üretilen PDF'i MinIO'dan akıtır.
func (h *Handler) DownloadMonthly(w http.ResponseWriter, r *http.Request) {
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
		FROM monthly_reports mr
		JOIN files f ON f.id = mr.generated_pdf_file_id
		WHERE mr.id=$1 AND mr.project_id=$2 AND mr.deleted_at IS NULL
		  AND mr.status='Ready'`, id, pid).Scan(&key, &name)
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
		h.log.Error("aylık PDF okunamadı", "err", err, "key", key)
		httpx.Internal(w, r)
		return
	}
	defer obj.Body.Close()
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	_, _ = io.Copy(w, obj.Body)
}

func pdfFilePrefix(kind string) string {
	switch kind {
	case KindQuarterly:
		return "ceyrek-yonetim-raporu"
	case KindCustom:
		return "donem-yonetim-raporu"
	}
	return "aylik-yonetim-raporu"
}

// kindTitleTR — bildirim başlıklarında kullanılan Türkçe rapor adı.
func kindTitleTR(kind string) string {
	switch kind {
	case KindQuarterly:
		return "Çeyrek yıl yönetim raporu"
	case KindCustom:
		return "Özel dönem yönetim raporu"
	}
	return "Aylık yönetim raporu"
}

func fileSafe(s string) string { return strings.NewReplacer(" ", "", ":", "-", "/", "-").Replace(s) }

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Worker tarafı — PDF üretimi (cmd/worker bu fonksiyonu çağırır)
// ---------------------------------------------------------------------------

// ProcessMonthlyPDF — snapshot'tan PDF üretir, depoya yükler, kaydı Ready yapar.
// Dönen error yeniden denemeyi tetikler; kalıcı hatalar nil döner (Failed).
func ProcessMonthlyPDF(ctx context.Context, pool *pgxpool.Pool, store *storage.Client,
	nt *notify.Service, log *slog.Logger, payload json.RawMessage) error {

	var p monthlyJobPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		log.Warn("aylık PDF işi: bozuk payload, atlanıyor", "err", err)
		return nil
	}
	id, err := uuid.Parse(p.MonthlyReportID)
	if err != nil {
		log.Warn("aylık PDF işi: geçersiz id, atlanıyor", "id", p.MonthlyReportID)
		return nil
	}

	var pid, genBy uuid.UUID
	var snapJSON []byte
	var status string
	err = pool.QueryRow(ctx, `
		SELECT project_id, generated_by, snapshot, status FROM monthly_reports
		WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&pid, &genBy, &snapJSON, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Warn("aylık PDF işi: kayıt yok, atlanıyor", "id", id)
		return nil
	}
	if err != nil {
		return err // geçici DB hatası → yeniden dene
	}
	if status == "Ready" {
		return nil // idempotentlik
	}

	var sn MonthlySnapshot
	if err := json.Unmarshal(snapJSON, &sn); err != nil {
		markMonthlyFailed(ctx, pool, nt, log, id, pid, genBy, "snapshot çözümlenemedi")
		return nil
	}

	pdf := BuildMonthlyPDF(sn)
	key := fmt.Sprintf("project/%s/reports/monthly/%s.pdf", pid, id)
	sha := sha256Hex(pdf)
	if err := store.PutObject(ctx, key, "application/pdf",
		bytes.NewReader(pdf), int64(len(pdf)), sha); err != nil {
		return fmt.Errorf("PDF depoya yüklenemedi: %w", err)
	}

	fname := fmt.Sprintf("%s-%s-%s.pdf", pdfFilePrefix(sn.Kind), sn.ProjectCode, fileSafe(sn.PeriodStart+"_"+sn.PeriodEnd))
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var fileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO files (storage_key, original_name, mime, size_bytes, sha256, uploaded_by)
		VALUES ($1,$2,'application/pdf',$3,$4,$5) RETURNING id`,
		key, fname, len(pdf), sha, genBy).Scan(&fileID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE monthly_reports
		SET status='Ready', generated_pdf_file_id=$2, error=NULL, row_version=row_version+1
		WHERE id=$1`, id, fileID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	nt.Send(ctx, notify.Input{
		UserIDs: []uuid.UUID{genBy},
		Type:    notify.TypeMonthlyReportReady,
		Title:   fmt.Sprintf("%s hazır: %s / %s", kindTitleTR(sn.Kind), sn.ProjectCode, sn.Label),
		Body:    fmt.Sprintf("SPI %.3f · CPI %.3f — PDF indirilebilir.", sn.EVM.SPI, sn.EVM.CPI),
		EntityType: "monthly_reports", EntityID: &id, ProjectID: &pid,
	})
	log.Info("aylık PDF üretildi", "monthly", id, "bytes", len(pdf))
	return nil
}

func markMonthlyFailed(ctx context.Context, pool *pgxpool.Pool, nt *notify.Service,
	log *slog.Logger, id, pid, genBy uuid.UUID, reason string) {
	if _, err := pool.Exec(ctx, `
		UPDATE monthly_reports SET status='Failed', error=$2, row_version=row_version+1
		WHERE id=$1`, id, reason); err != nil {
		log.Error("aylık rapor Failed işaretlenemedi", "id", id, "err", err)
	}
	nt.Send(ctx, notify.Input{
		UserIDs: []uuid.UUID{genBy},
		Type:    notify.TypeMonthlyReportFailed,
		Title:   "Yönetim raporu üretilemedi",
		Body:    reason,
		EntityType: "monthly_reports", EntityID: &id, ProjectID: &pid,
	})
}
