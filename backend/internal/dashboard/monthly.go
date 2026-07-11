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

	// Tedarik.
	Procurement struct {
		POsOrdered   int `json:"pos_ordered"`
		POsDelivered int `json:"pos_delivered"`
		POsOverdue   int `json:"pos_overdue"`
		PRsPending   int `json:"prs_pending"`
	} `json:"procurement"`
}

// buildMonthlySnapshot — ayın tüm verisini derler (Plan Faz 9: EVM + finansal
// + İSG + tedarik tek raporda).
func (h *Handler) buildMonthlySnapshot(ctx context.Context, pid uuid.UUID, year, month int) (*MonthlySnapshot, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	sn := &MonthlySnapshot{
		GeneratedAt: time.Now().UTC(),
		Year:        year, Month: month,
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
		pid, start, start.AddDate(0, 1, 0))
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
		GROUP BY pd.type`, pid, start, start.AddDate(0, 1, 0))
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
		ORDER BY sort_order, planned_date NULLS LAST`, pid, end.AddDate(0, 0, 1))
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
		pid, start, start.AddDate(0, 1, 0)).
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
		pid, start, start.AddDate(0, 1, 0)).
		Scan(&sn.Procurement.POsOrdered, &sn.Procurement.POsDelivered,
			&sn.Procurement.POsOverdue, &sn.Procurement.PRsPending); err != nil {
		return nil, err
	}
	return sn, nil
}

// ---------------------------------------------------------------------------
// HTTP uçları
// ---------------------------------------------------------------------------

type monthlyDTO struct {
	ID          uuid.UUID `json:"id"`
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
		SELECT mr.id, mr.year, mr.month, to_char(mr.period_start,'YYYY-MM-DD'),
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
		if err := rows.Scan(&d.ID, &d.Year, &d.Month, &d.PeriodStart, &d.PeriodEnd,
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
		SELECT mr.id, mr.year, mr.month, to_char(mr.period_start,'YYYY-MM-DD'),
		       to_char(mr.period_end,'YYYY-MM-DD'), mr.status, mr.error,
		       u.full_name, mr.generated_pdf_file_id IS NOT NULL, mr.created_at, mr.snapshot
		FROM monthly_reports mr
		JOIN users u ON u.id = mr.generated_by
		WHERE mr.id=$1 AND mr.project_id=$2 AND mr.deleted_at IS NULL`, id, pid).
		Scan(&d.ID, &d.Year, &d.Month, &d.PeriodStart, &d.PeriodEnd, &d.Status,
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
		Month string `json:"month"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	t, err := time.Parse("2006-01", strings.TrimSpace(in.Month))
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"month": "geçerli bir ay (YYYY-MM) girin"})
		return
	}

	sn, err := h.buildMonthlySnapshot(r.Context(), pid, t.Year(), int(t.Month()))
	if err != nil {
		h.log.Error("aylık snapshot derlenemedi", "err", err, "project", pid)
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
		INSERT INTO monthly_reports (project_id, year, month, period_start, period_end,
		                             status, snapshot, generated_by)
		VALUES ($1,$2,$3,$4::date,$5::date,'Pending',$6,$7)
		RETURNING id`,
		pid, sn.Year, sn.Month, sn.PeriodStart, sn.PeriodEnd, snapJSON, uid).Scan(&id)
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
		After: map[string]interface{}{"year": sn.Year, "month": sn.Month,
			"spi": sn.EVM.SPI, "cpi": sn.EVM.CPI},
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusAccepted, map[string]interface{}{
		"id": id, "status": "Pending", "year": sn.Year, "month": sn.Month,
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

	fname := fmt.Sprintf("aylik-yonetim-raporu-%s-%04d-%02d.pdf", sn.ProjectCode, sn.Year, sn.Month)
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
		Title:   fmt.Sprintf("Aylık yönetim raporu hazır: %s / %04d-%02d", sn.ProjectCode, sn.Year, sn.Month),
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
		Title:   "Aylık yönetim raporu üretilemedi",
		Body:    reason,
		EntityType: "monthly_reports", EntityID: &id, ProjectID: &pid,
	})
}
