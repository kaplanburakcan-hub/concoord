// Package reports — Faz 6: Günlük/Haftalık Saha Raporlama.
//
// Plan §6.6 + §8 (Faz 6):
//   - Mobil öncelikli günlük rapor: hava, personel, ekipman, imalat
//     (work_item bağlantılı). Taslak → Gönderildi akışı.
//   - Geçmiş gün düzeltmesi = REVİZYON: Submitted rapor DB trigger'ıyla
//     kilitlidir; /revise yeni bir Draft revizyonu (revision_no+1) açar ve
//     tüm satırları kopyalar. Geçerli rapor = en yüksek revizyon.
//   - Haftalık PDF worker'da üretilir; veriler üretim anında snapshot JSONB
//     olarak dondurulur (weekly.go).
//
// İzinler: reports.view / reports.create_daily / reports.generate_weekly.
// Finansal alan yoktur; view_financials ayrımı bu modülde gerekmez.
package reports

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/notify"
	"github.com/ipks/ipks/backend/internal/rbac"
	"github.com/ipks/ipks/backend/internal/storage"
)

type Handler struct {
	pool  *pgxpool.Pool
	eval  *rbac.Evaluator
	rec   *audit.Recorder
	nt    *notify.Service
	store *storage.Client
	log   *slog.Logger

	weatherEnabled bool
	weatherAPIURL  string
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder, nt *notify.Service,
	store *storage.Client, log *slog.Logger, weatherEnabled bool, weatherAPIURL string) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, nt: nt, store: store, log: log,
		weatherEnabled: weatherEnabled, weatherAPIURL: weatherAPIURL}
}

// ---------------------------------------------------------------------------
// DTO'lar
// ---------------------------------------------------------------------------

type manpowerDTO struct {
	ID                uuid.UUID  `json:"id,omitempty"`
	SubcontractorID   *uuid.UUID `json:"subcontractor_id,omitempty"`
	SubcontractorName *string    `json:"subcontractor_name,omitempty"`
	Trade             string     `json:"trade"`
	Headcount         int        `json:"headcount"`
}

type equipmentDTO struct {
	ID            uuid.UUID `json:"id,omitempty"`
	EquipmentName string    `json:"equipment_name"`
	Count         int       `json:"count"`
	WorkingHours  *float64  `json:"working_hours,omitempty"`
	IdleReason    *string   `json:"idle_reason,omitempty"`
}

type workEntryDTO struct {
	ID           uuid.UUID  `json:"id,omitempty"`
	WorkItemID   *uuid.UUID `json:"work_item_id,omitempty"`
	WorkItemPoz  *string    `json:"work_item_poz,omitempty"`
	Location     *string    `json:"location,omitempty"`
	Description  string     `json:"description"`
	Qty          *float64   `json:"qty,omitempty"`
	Unit         *string    `json:"unit,omitempty"`
}

type dailyDTO struct {
	ID                uuid.UUID       `json:"id"`
	ProjectID         uuid.UUID       `json:"project_id"`
	ReportDate        string          `json:"report_date"` // YYYY-MM-DD
	RevisionNo        int             `json:"revision_no"`
	ParentReportID    *uuid.UUID      `json:"parent_report_id,omitempty"`
	Weather           json.RawMessage `json:"weather,omitempty"`
	TempMin           *float64        `json:"temperature_min,omitempty"`
	TempMax           *float64        `json:"temperature_max,omitempty"`
	Notes             *string         `json:"notes,omitempty"`
	Status            string          `json:"status"`
	AuthorID          uuid.UUID       `json:"author_id"`
	AuthorName        string          `json:"author_name"`
	SubmittedAt       *time.Time      `json:"submitted_at,omitempty"`
	RowVersion        int             `json:"row_version"`
	CreatedAt         time.Time       `json:"created_at"`
	CoverPhotoFileID  *uuid.UUID      `json:"cover_photo_file_id,omitempty"`

	Manpower    []manpowerDTO  `json:"manpower,omitempty"`
	Equipment   []equipmentDTO `json:"equipment,omitempty"`
	WorkEntries []workEntryDTO `json:"work_entries,omitempty"`
}

type dailyInput struct {
	ReportDate string          `json:"report_date"`
	Weather    json.RawMessage `json:"weather"`
	TempMin    *float64        `json:"temperature_min"`
	TempMax    *float64        `json:"temperature_max"`
	Notes      string          `json:"notes"`
	Manpower   []manpowerDTO   `json:"manpower"`
	Equipment  []equipmentDTO  `json:"equipment"`
	WorkEntries []workEntryDTO `json:"work_entries"`
}

// ---------------------------------------------------------------------------
// Ortak yardımcılar
// ---------------------------------------------------------------------------

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.",
			map[string]string{param: "geçersiz UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, false
	}
	return uid, true
}

func isLockViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23001"
	}
	return false
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

const drSelect = `
	SELECT dr.id, dr.project_id, to_char(dr.report_date,'YYYY-MM-DD'), dr.revision_no,
	       dr.parent_report_id, dr.weather, dr.temperature_min, dr.temperature_max,
	       dr.notes, dr.status, dr.author_id, u.full_name, dr.submitted_at,
	       dr.row_version, dr.created_at, dr.cover_photo_file_id
	FROM daily_reports dr
	JOIN users u ON u.id = dr.author_id`

func scanDaily(row pgx.Row, d *dailyDTO) error {
	return row.Scan(&d.ID, &d.ProjectID, &d.ReportDate, &d.RevisionNo, &d.ParentReportID,
		&d.Weather, &d.TempMin, &d.TempMax, &d.Notes, &d.Status,
		&d.AuthorID, &d.AuthorName, &d.SubmittedAt, &d.RowVersion, &d.CreatedAt, &d.CoverPhotoFileID)
}

// loadLines — bir raporun satırlarını doldurur.
func (h *Handler) loadLines(ctx context.Context, d *dailyDTO) error {
	rows, err := h.pool.Query(ctx, `
		SELECT dm.id, dm.subcontractor_id, s.company_name, dm.trade, dm.headcount
		FROM daily_manpower dm
		LEFT JOIN subcontractors s ON s.id = dm.subcontractor_id
		WHERE dm.daily_report_id=$1 AND dm.deleted_at IS NULL
		ORDER BY dm.created_at`, d.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var m manpowerDTO
		if err := rows.Scan(&m.ID, &m.SubcontractorID, &m.SubcontractorName, &m.Trade, &m.Headcount); err != nil {
			rows.Close()
			return err
		}
		d.Manpower = append(d.Manpower, m)
	}
	rows.Close()

	rows, err = h.pool.Query(ctx, `
		SELECT id, equipment_name, count, working_hours, idle_reason
		FROM daily_equipment
		WHERE daily_report_id=$1 AND deleted_at IS NULL
		ORDER BY created_at`, d.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var e equipmentDTO
		if err := rows.Scan(&e.ID, &e.EquipmentName, &e.Count, &e.WorkingHours, &e.IdleReason); err != nil {
			rows.Close()
			return err
		}
		d.Equipment = append(d.Equipment, e)
	}
	rows.Close()

	rows, err = h.pool.Query(ctx, `
		SELECT dw.id, dw.work_item_id, wi.poz_no, dw.location, dw.description, dw.qty, dw.unit
		FROM daily_work_entries dw
		LEFT JOIN work_items wi ON wi.id = dw.work_item_id
		WHERE dw.daily_report_id=$1 AND dw.deleted_at IS NULL
		ORDER BY dw.created_at`, d.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var we workEntryDTO
		if err := rows.Scan(&we.ID, &we.WorkItemID, &we.WorkItemPoz, &we.Location, &we.Description, &we.Qty, &we.Unit); err != nil {
			rows.Close()
			return err
		}
		d.WorkEntries = append(d.WorkEntries, we)
	}
	rows.Close()
	return nil
}

// insertLines — satırları tek tx içinde yazar (create / draft-update / revise ortak).
func insertLines(ctx context.Context, tx pgx.Tx, reportID uuid.UUID, in dailyInput) error {
	for _, m := range in.Manpower {
		if _, err := tx.Exec(ctx, `
			INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount)
			VALUES ($1,$2,$3,$4)`, reportID, m.SubcontractorID, strings.TrimSpace(m.Trade), m.Headcount); err != nil {
			return err
		}
	}
	for _, e := range in.Equipment {
		if _, err := tx.Exec(ctx, `
			INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason)
			VALUES ($1,$2,$3,$4,NULLIF(btrim(COALESCE($5,'')),''))`,
			reportID, strings.TrimSpace(e.EquipmentName), e.Count, e.WorkingHours, e.IdleReason); err != nil {
			return err
		}
	}
	for _, we := range in.WorkEntries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit)
			VALUES ($1,$2,NULLIF(btrim(COALESCE($3,'')),''),$4,$5,NULLIF(btrim(COALESCE($6,'')),''))`,
			reportID, we.WorkItemID, we.Location, strings.TrimSpace(we.Description), we.Qty, we.Unit); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Günlük rapor uçları
// ---------------------------------------------------------------------------

// ListDaily — geçerli revizyonları (tarih başına en yükseği) listeler.
// ?from=YYYY-MM-DD&to=YYYY-MM-DD isteğe bağlı aralık filtresi.
func (h *Handler) ListDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))

	rows, err := h.pool.Query(r.Context(), drSelect+`
		WHERE dr.project_id=$1 AND dr.deleted_at IS NULL
		  AND dr.revision_no = (
		      SELECT max(x.revision_no) FROM daily_reports x
		      WHERE x.project_id=dr.project_id AND x.report_date=dr.report_date
		        AND x.deleted_at IS NULL)
		  AND ($2 = '' OR dr.report_date >= $2::date)
		  AND ($3 = '' OR dr.report_date <= $3::date)
		ORDER BY dr.report_date DESC
		LIMIT 200`, pid, from, to)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()

	out := []dailyDTO{}
	for rows.Next() {
		var d dailyDTO
		if err := scanDaily(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"daily_reports": out})
}

// GetDaily — başlık + tüm satırlar.
func (h *Handler) GetDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var d dailyDTO
	err := scanDaily(h.pool.QueryRow(r.Context(), drSelect+`
		WHERE dr.id=$1 AND dr.project_id=$2 AND dr.deleted_at IS NULL`, id, pid), &d)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Günlük rapor bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := h.loadLines(r.Context(), &d); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"daily_report": d})
}

// CreateDaily — yeni taslak (revision_no=1). Aynı tarihte kayıt varsa 409.
func (h *Handler) CreateDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in dailyInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if fields := validateDaily(in); len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var id uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO daily_reports (project_id, report_date, weather, temperature_min,
		                           temperature_max, notes, author_id)
		VALUES ($1,$2::date,$3,$4,$5,NULLIF(btrim($6),''),$7)
		RETURNING id`,
		pid, in.ReportDate, nullJSON(in.Weather), in.TempMin, in.TempMax, in.Notes, uid).Scan(&id)
	if isUniqueViolation(err) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Bu tarih için rapor zaten var. Gönderilmiş raporu düzeltmek için revizyon açın.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := insertLines(r.Context(), tx, id, in); err != nil {
		httpx.Internal(w, r)
		return
	}

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "daily_reports", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"report_date": in.ReportDate, "status": "Draft"},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

// UpdateDaily — TASLAK günceller (başlık + satırlar tam değişim).
// Submitted rapor DB kilidine takılır → 409.
func (h *Handler) UpdateDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var in dailyInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if fields := validateDaily(in); len(fields) > 0 {
		httpx.ValidationFailed(w, r, fields)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	ct, err := tx.Exec(r.Context(), `
		UPDATE daily_reports
		SET weather=$3, temperature_min=$4, temperature_max=$5,
		    notes=NULLIF(btrim($6),''), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		id, pid, nullJSON(in.Weather), in.TempMin, in.TempMax, in.Notes)
	if isLockViolation(err) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Gönderilmiş rapor değiştirilemez; düzeltme için revizyon açın.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Günlük rapor bulunamadı.", nil)
		return
	}

	// Satırlar: tam değişim (taslakta basit ve güvenli; audit başlıkta tutulur).
	for _, tbl := range []string{"daily_manpower", "daily_equipment", "daily_work_entries"} {
		if _, err := tx.Exec(r.Context(), `DELETE FROM `+tbl+` WHERE daily_report_id=$1`, id); err != nil {
			if isLockViolation(err) {
				httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
					"Gönderilmiş rapor değiştirilemez; düzeltme için revizyon açın.", nil)
				return
			}
			httpx.Internal(w, r)
			return
		}
	}
	if err := insertLines(r.Context(), tx, id, in); err != nil {
		httpx.Internal(w, r)
		return
	}

	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "daily_reports", EntityID: id.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"manpower": len(in.Manpower), "equipment": len(in.Equipment),
			"work_entries": len(in.WorkEntries)},
		IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		if isLockViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Gönderilmiş rapor değiştirilemez.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SubmitDaily — Draft → Submitted. Geçiş workflow_transitions'a yazılır.
func (h *Handler) SubmitDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	err = tx.QueryRow(r.Context(), `
		SELECT status FROM daily_reports
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, pid).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Günlük rapor bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if status != "Draft" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Rapor zaten gönderilmiş.", nil)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		UPDATE daily_reports
		SET status='Submitted', submitted_at=now(), submitted_by=$3, row_version=row_version+1
		WHERE id=$1 AND project_id=$2`, id, pid, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	if _, err := tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id)
		VALUES ('daily_report', $1, 'Draft', 'Submitted', $2)`, id, uid); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "daily_reports", EntityID: id.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"status": "Draft"},
		After:  map[string]interface{}{"status": "Submitted"},
		IP:     m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "Submitted"})
}

// ReviseDaily — Submitted raporun düzeltmesi: yeni Draft revizyonu açar,
// başlık ve tüm satırları kopyalar (Plan Faz 6: "geçmiş gün düzeltmesi = revizyon").
func (h *Handler) ReviseDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var status, reportDate string
	err = tx.QueryRow(r.Context(), `
		SELECT status, to_char(report_date,'YYYY-MM-DD') FROM daily_reports
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, pid).
		Scan(&status, &reportDate)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Günlük rapor bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if status != "Submitted" {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Yalnızca gönderilmiş rapor için revizyon açılır; taslak doğrudan düzenlenir.", nil)
		return
	}

	// Aynı tarih için açık (Draft) bir revizyon varsa ikincisi açılmaz —
	// iki paralel taslak, hangisinin geçerli olacağı belirsizliği doğurur.
	var openDrafts int
	if err := tx.QueryRow(r.Context(), `
		SELECT count(*) FROM daily_reports
		WHERE project_id=$1 AND report_date=$2::date
		  AND status='Draft' AND deleted_at IS NULL`, pid, reportDate).Scan(&openDrafts); err != nil {
		httpx.Internal(w, r)
		return
	}
	if openDrafts > 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Bu tarih için açık bir taslak revizyon zaten var; önce onu tamamlayın.", nil)
		return
	}

	var newID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO daily_reports (project_id, report_date, revision_no, parent_report_id,
		                           weather, temperature_min, temperature_max, notes, author_id)
		SELECT project_id, report_date,
		       (SELECT max(revision_no)+1 FROM daily_reports x
		        WHERE x.project_id=$2 AND x.report_date=$3::date AND x.deleted_at IS NULL),
		       id, weather, temperature_min, temperature_max, notes, $4
		FROM daily_reports WHERE id=$1
		RETURNING id`, id, pid, reportDate, uid).Scan(&newID)
	if isUniqueViolation(err) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Bu tarih için açık bir revizyon zaten var.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	// Satır kopyalama.
	for _, q := range []string{
		`INSERT INTO daily_manpower (daily_report_id, subcontractor_id, trade, headcount)
		 SELECT $2, subcontractor_id, trade, headcount
		 FROM daily_manpower WHERE daily_report_id=$1 AND deleted_at IS NULL`,
		`INSERT INTO daily_equipment (daily_report_id, equipment_name, count, working_hours, idle_reason)
		 SELECT $2, equipment_name, count, working_hours, idle_reason
		 FROM daily_equipment WHERE daily_report_id=$1 AND deleted_at IS NULL`,
		`INSERT INTO daily_work_entries (daily_report_id, work_item_id, location, description, qty, unit)
		 SELECT $2, work_item_id, location, description, qty, unit
		 FROM daily_work_entries WHERE daily_report_id=$1 AND deleted_at IS NULL`,
	} {
		if _, err := tx.Exec(r.Context(), q, id, newID); err != nil {
			httpx.Internal(w, r)
			return
		}
	}

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO workflow_transitions (entity, entity_id, from_status, to_status, actor_id, note)
		VALUES ('daily_report', $1, 'Submitted', 'Draft', $2, 'revizyon: '||$3::text)`,
		newID, uid, id.String()); err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "daily_reports", EntityID: newID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"report_date": reportDate, "parent_report_id": id.String(), "status": "Draft"},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"id": newID})
}

// DeleteDaily — yalnızca taslak; Submitted DB kilidine takılır.
func (h *Handler) DeleteDaily(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE daily_reports SET deleted_at=now(), row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid)
	if isLockViolation(err) {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Gönderilmiş rapor silinemez.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Günlük rapor bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "daily_reports", EntityID: id.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// SetCoverPhoto — günlük rapora kapak fotoğrafı ekler / günceller.
// POST /projects/{projectID}/daily-reports/{id}/cover-photo
// Body: { "photo": "data:image/jpeg;base64,..." }
func (h *Handler) SetCoverPhoto(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}

	var body struct {
		Photo string `json:"photo"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if body.Photo == "" {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Fotoğraf verisi boş.", nil)
		return
	}

	// Veri URL'sini çöz.
	if !strings.HasPrefix(body.Photo, "data:") {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz fotoğraf formatı.", nil)
		return
	}
	rest := body.Photo[len("data:"):]
	semi := strings.Index(rest, ";base64,")
	if semi < 0 {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz fotoğraf formatı.", nil)
		return
	}
	mimeType := rest[:semi]
	if !strings.HasPrefix(mimeType, "image/") {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Yalnızca resim dosyaları kabul edilir.", nil)
		return
	}
	raw, err := base64.StdEncoding.DecodeString(rest[semi+len(";base64,"):])
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Fotoğraf verisi çözümlenemedi.", nil)
		return
	}
	const maxSize = 8 << 20 // 8 MB
	if len(raw) > maxSize {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Fotoğraf 8 MB sınırını aşıyor.", nil)
		return
	}

	safeMime, verr := h.store.CheckUpload(bytes.NewReader(raw), "cover.jpg")
	if verr != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya güvenlik kontrolü başarısız.", nil)
		return
	}
	mimeType = safeMime

	sum := sha256Hex(raw)
	ext := "jpg"
	if strings.Contains(mimeType, "png") {
		ext = "png"
	} else if strings.Contains(mimeType, "webp") {
		ext = "webp"
	}
	key := fmt.Sprintf("project/%s/daily-reports/%s/cover.%s", pid, id, ext)

	if err := h.store.PutObject(r.Context(), key, mimeType, bytes.NewReader(raw), int64(len(raw)), sum); err != nil {
		h.log.Error("kapak fotoğrafı yüklenemedi", "err", err)
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Fotoğraf depolama alanına yüklenemedi.", nil)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var fileID uuid.UUID
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO files (storage_key, original_name, mime, size_bytes, sha256, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		key, "cover."+ext, mimeType, len(raw), sum, uid).Scan(&fileID); err != nil {
		httpx.Internal(w, r)
		return
	}
	ct, err := tx.Exec(r.Context(), `
		UPDATE daily_reports SET cover_photo_file_id=$3, row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid, fileID)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Günlük rapor bulunamadı.", nil)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"cover_photo_file_id": fileID.String()})
}

// GetCoverPhoto — kapak fotoğrafını doğrudan akışla döner.
// GET /projects/{projectID}/daily-reports/{id}/cover-photo
func (h *Handler) GetCoverPhoto(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}

	var key, mimeType string
	var size int64
	err := h.pool.QueryRow(r.Context(), `
		SELECT f.storage_key, f.mime, f.size_bytes
		FROM daily_reports dr
		JOIN files f ON f.id = dr.cover_photo_file_id
		WHERE dr.id=$1 AND dr.project_id=$2 AND dr.deleted_at IS NULL`, id, pid).
		Scan(&key, &mimeType, &size)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kapak fotoğrafı bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}

	obj, err := h.store.GetObject(r.Context(), key)
	if err != nil {
		h.log.Error("kapak fotoğrafı okunamadı", "err", err, "key", key)
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Dosya deposundan okunamadı.", nil)
		return
	}
	defer obj.Body.Close()

	if mimeType == "" {
		mimeType = obj.ContentType
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj.Body)
}

func nullJSON(raw json.RawMessage) interface{} {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil
	}
	return raw
}

// ── DailyContext — günlük rapor yardımcı verileri ─────────────────────────────
// GET /projects/{projectID}/daily-report-context?date=YYYY-MM-DD
// Rapor formuna eklenen Depo-Stok özeti ve Bekleyen Aktiviteler için gerekli
// verileri tek istekte döner. reports.view yetkisi yeterli.

type DailyContextWarehouse struct {
	MalzemeAdi string  `json:"malzeme_adi"`
	Kategori   string  `json:"kategori"`
	Birim      string  `json:"birim"`
	Giris      float64 `json:"giris"`
	Cikis      float64 `json:"cikis"`
	NetDelta   float64 `json:"net_delta"`
}

type DailyContextResponse struct {
	WarehouseDelta []DailyContextWarehouse `json:"warehouse_delta"`
	PendingMARs    int                     `json:"pending_mars"`
	PendingPOs     int                     `json:"pending_pos"`
	OpenTasks      int                     `json:"open_tasks"`
}

func (h *Handler) DailyContext(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	dateStr := strings.TrimSpace(r.URL.Query().Get("date"))
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"date": "geçerli bir tarih (YYYY-MM-DD) girin"})
		return
	}
	ctx := r.Context()
	var out DailyContextResponse

	// Depo hareketleri özeti (sadece o güne ait giris/cikis).
	rows, err := h.pool.Query(ctx, `
		SELECT i.malzeme_adi, i.kategori, i.birim,
		       COALESCE(SUM(m.miktar::float8) FILTER (WHERE m.hareket_turu='giris'),  0),
		       COALESCE(SUM(m.miktar::float8) FILTER (WHERE m.hareket_turu='cikis'),  0)
		FROM site_warehouse_items i
		JOIN site_warehouse_movements m ON m.item_id=i.id AND m.tarih=$2::date
		WHERE i.project_id=$1
		GROUP BY i.id, i.malzeme_adi, i.kategori, i.birim
		HAVING SUM(m.miktar::float8) > 0
		ORDER BY i.kategori, i.malzeme_adi`, pid, dateStr)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	out.WarehouseDelta = []DailyContextWarehouse{}
	for rows.Next() {
		var wh DailyContextWarehouse
		if err := rows.Scan(&wh.MalzemeAdi, &wh.Kategori, &wh.Birim, &wh.Giris, &wh.Cikis); err != nil {
			rows.Close()
			httpx.Internal(w, r)
			return
		}
		wh.NetDelta = wh.Giris - wh.Cikis
		out.WarehouseDelta = append(out.WarehouseDelta, wh)
	}
	rows.Close()

	// Bekleyen MAR sayısı.
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FROM material_approvals
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND status IN ('Submitted','UnderReview')`, pid).Scan(&out.PendingMARs); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Bekleyen/açık PO sayısı.
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FROM purchase_orders
		WHERE project_id=$1 AND deleted_at IS NULL
		  AND status IN ('Ordered','PartiallyDelivered')`, pid).Scan(&out.PendingPOs); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Açık görev sayısı.
	if err := h.pool.QueryRow(ctx, `
		SELECT count(*) FROM tasks
		WHERE project_id=$1 AND deleted_at IS NULL AND status <> 'Done'`, pid).Scan(&out.OpenTasks); err != nil {
		httpx.Internal(w, r)
		return
	}

	httpx.JSON(w, http.StatusOK, out)
}
