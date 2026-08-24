// Tedarik Planı — Satın Alma altında, mevcut PR/PO onay zincirine
// dokunmayan, zorunlu olmayan bir planlama/takip referansı. Tek satırlık
// manuel ekleme ya da Excel/CSV toplu içe aktarma ile doldurulur; durum
// (Planlandı/Sipariş Verildi/Yolda/Teslim Alındı/Gecikti) uygulama
// içinden elle güncellenir.
package procurement

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
)

type planItemDTO struct {
	ID                  uuid.UUID `json:"id"`
	ProjectID           uuid.UUID `json:"project_id"`
	PozNo               *string   `json:"poz_no,omitempty"`
	Description         string    `json:"description"`
	Category            *string   `json:"category,omitempty"`
	Quantity            *float64  `json:"quantity,omitempty"`
	Unit                *string   `json:"unit,omitempty"`
	SupplierName        *string   `json:"supplier_name,omitempty"`
	PlannedOrderDate    *string   `json:"planned_order_date,omitempty"`
	PlannedDeliveryDate *string   `json:"planned_delivery_date,omitempty"`
	Criticality         *string   `json:"criticality,omitempty"`
	Status              string    `json:"status"`
	Note                *string   `json:"note,omitempty"`
	CreatedByName       string    `json:"created_by_name"`
	RowVersion          int       `json:"row_version"`
}

const planItemSelect = `
	SELECT p.id, p.project_id, p.poz_no, p.description, p.category, p.quantity, p.unit,
	       p.supplier_name, to_char(p.planned_order_date,'YYYY-MM-DD'),
	       to_char(p.planned_delivery_date,'YYYY-MM-DD'), p.criticality, p.status, p.note,
	       u.full_name, p.row_version
	FROM procurement_plan_items p
	JOIN users u ON u.id = p.created_by
`

func scanPlanItem(row pgx.Row, d *planItemDTO) error {
	return row.Scan(&d.ID, &d.ProjectID, &d.PozNo, &d.Description, &d.Category, &d.Quantity, &d.Unit,
		&d.SupplierName, &d.PlannedOrderDate, &d.PlannedDeliveryDate, &d.Criticality, &d.Status, &d.Note,
		&d.CreatedByName, &d.RowVersion)
}

var planStatuses = map[string]bool{
	"Planlandi": true, "SiparisVerildi": true, "Yolda": true, "TeslimAlindi": true, "Gecikti": true,
}
var planCriticalities = map[string]bool{"Kritik": true, "Normal": true}

func (h *Handler) ListPlanItems(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), planItemSelect+`
		WHERE p.project_id=$1 AND p.deleted_at IS NULL
		ORDER BY p.planned_delivery_date NULLS LAST, p.created_at`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []planItemDTO{}
	for rows.Next() {
		var d planItemDTO
		if err := scanPlanItem(rows, &d); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type planItemReq struct {
	PozNo               *string  `json:"poz_no"`
	Description         string   `json:"description"`
	Category            *string  `json:"category"`
	Quantity            *float64 `json:"quantity"`
	Unit                *string  `json:"unit"`
	SupplierName        *string  `json:"supplier_name"`
	PlannedOrderDate    *string  `json:"planned_order_date"`
	PlannedDeliveryDate *string  `json:"planned_delivery_date"`
	Criticality         *string  `json:"criticality"`
	Status              *string  `json:"status"`
	Note                *string  `json:"note"`
	RowVersion          int      `json:"row_version"`
}

func validatePlanItem(req planItemReq) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Description) == "" {
		errs["description"] = "açıklama zorunludur"
	}
	if req.Criticality != nil && *req.Criticality != "" && !planCriticalities[*req.Criticality] {
		errs["criticality"] = "Kritik ya da Normal olmalı"
	}
	if req.Status != nil && *req.Status != "" && !planStatuses[*req.Status] {
		errs["status"] = "geçersiz durum"
	}
	return errs
}

func nilIfEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

func (h *Handler) CreatePlanItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	var req planItemReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if errs := validatePlanItem(req); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	var id uuid.UUID
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO procurement_plan_items
			(project_id, poz_no, description, category, quantity, unit, supplier_name,
			 planned_order_date, planned_delivery_date, criticality, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::date,NULLIF($9,'')::date,$10,$11,$12)
		RETURNING id`,
		pid, nilIfEmpty(req.PozNo), strings.TrimSpace(req.Description), nilIfEmpty(req.Category),
		req.Quantity, nilIfEmpty(req.Unit), nilIfEmpty(req.SupplierName),
		strDeref(req.PlannedOrderDate), strDeref(req.PlannedDeliveryDate),
		nilIfEmpty(req.Criticality), nilIfEmpty(req.Note), uid,
	).Scan(&id)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "procurement_plan_items", EntityID: id.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"description": req.Description}, IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": id})
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *Handler) UpdatePlanItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req planItemReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if errs := validatePlanItem(req); len(errs) > 0 {
		httpx.ValidationFailed(w, r, errs)
		return
	}
	status := req.Status
	if status == nil || *status == "" {
		s := "Planlandi"
		status = &s
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE procurement_plan_items SET
			poz_no=$3, description=$4, category=$5, quantity=$6, unit=$7, supplier_name=$8,
			planned_order_date=NULLIF($9,'')::date, planned_delivery_date=NULLIF($10,'')::date,
			criticality=$11, status=$12, note=$13, row_version=row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL
		  AND ($14=0 OR row_version=$14)`,
		id, pid, nilIfEmpty(req.PozNo), strings.TrimSpace(req.Description), nilIfEmpty(req.Category),
		req.Quantity, nilIfEmpty(req.Unit), nilIfEmpty(req.SupplierName),
		strDeref(req.PlannedOrderDate), strDeref(req.PlannedDeliveryDate),
		nilIfEmpty(req.Criticality), *status, nilIfEmpty(req.Note), req.RowVersion)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt bulunamadı ya da başkası tarafından güncellendi.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "procurement_plan_items", EntityID: id.String(), Action: audit.ActionUpdate,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) DeletePlanItem(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(), `
		UPDATE procurement_plan_items SET deleted_at=now()
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, id, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kayıt bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "procurement_plan_items", EntityID: id.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Excel/CSV toplu içe aktarma — payments/import.go ile aynı desen (harici
// kütüphane yok, ADR-0003): .xlsx ham zip+XML, .csv encoding/csv. Beklenen
// sütun düzeni (başlık satırı atlanır): A=poz_no, B=açıklama(zorunlu),
// C=kategori, D=miktar, E=birim, F=tedarikçi, G=planlanan sipariş tarihi,
// H=planlanan teslim tarihi, I=kritiklik, J=not.
// ---------------------------------------------------------------------------

const maxPlanImportBytes = 10 << 20

type importedPlanRow struct {
	PozNo, Description, Category, Unit, Supplier, OrderDate, DeliveryDate, Criticality, Note string
	Quantity                                                                                 float64
}

func (h *Handler) ImportPlanItems(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPlanImportBytes+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya çözümlenemedi (sınır 10 MB).", nil)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"file": "dosya alanı zorunlu (.xlsx veya .csv)"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPlanImportBytes))
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	rowsIn, err := parsePlanImport(hdr.Filename, data)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya okunamadı: "+err.Error(), nil)
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var inserted, skipped int
	for _, row := range rowsIn {
		desc := strings.TrimSpace(row.Description)
		if desc == "" {
			skipped++
			continue
		}
		crit := strings.TrimSpace(row.Criticality)
		if !planCriticalities[crit] {
			crit = ""
		}
		var qty *float64
		if row.Quantity != 0 {
			q := row.Quantity
			qty = &q
		}
		_, err := tx.Exec(r.Context(), `
			INSERT INTO procurement_plan_items
				(project_id, poz_no, description, category, quantity, unit, supplier_name,
				 planned_order_date, planned_delivery_date, criticality, note, created_by)
			VALUES ($1,NULLIF($2,''),$3,NULLIF($4,''),$5,NULLIF($6,''),NULLIF($7,''),
			        NULLIF($8,'')::date,NULLIF($9,'')::date,NULLIF($10,''),NULLIF($11,''),$12)`,
			pid, strings.TrimSpace(row.PozNo), desc, strings.TrimSpace(row.Category), qty,
			strings.TrimSpace(row.Unit), strings.TrimSpace(row.Supplier),
			normalizeDate(row.OrderDate), normalizeDate(row.DeliveryDate), crit,
			strings.TrimSpace(row.Note), uid)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Satır yazılamadı: "+err.Error(), nil)
			return
		}
		inserted++
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "procurement_plan_items", EntityID: pid.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"import": "bulk", "rows": inserted, "skipped": skipped}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"processed": inserted, "skipped": skipped, "total": len(rowsIn)})
}

// normalizeDate — "GG.AA.YYYY" ya da "YYYY-AA-GG" kabul eder, YYYY-MM-DD döner.
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02")
	}
	if t, err := time.Parse("02.01.2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	return ""
}

func parsePlanImport(filename string, data []byte) ([]importedPlanRow, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return parsePlanXLSX(data)
	case strings.HasSuffix(lower, ".csv"):
		return parsePlanCSV(data)
	default:
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return parsePlanXLSX(data)
		}
		return parsePlanCSV(data)
	}
}

func parsePlanCSV(data []byte) ([]importedPlanRow, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv çözümlenemedi: %w", err)
	}
	var out []importedPlanRow
	for i, rec := range recs {
		if i == 0 && planLooksLikeHeader(rec) {
			continue
		}
		if planAllEmpty(rec) {
			continue
		}
		out = append(out, planRowFromCells(rec))
	}
	return out, nil
}

type planXLSXSST struct {
	Items []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}
type planXLSXSheet struct {
	Rows []struct {
		Cells []struct {
			R  string `xml:"r,attr"`
			T  string `xml:"t,attr"`
			V  string `xml:"v"`
			IS struct {
				T string `xml:"t"`
			} `xml:"is"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

const planImportCols = 10 // A..J

func parsePlanXLSX(data []byte) ([]importedPlanRow, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xlsx (zip) açılamadı: %w", err)
	}
	var shared []string
	var sheetXML []byte
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			b, err := planReadZip(f)
			if err != nil {
				return nil, err
			}
			var sst planXLSXSST
			if err := xml.Unmarshal(b, &sst); err != nil {
				return nil, fmt.Errorf("sharedStrings çözümlenemedi: %w", err)
			}
			for _, si := range sst.Items {
				if si.T != "" {
					shared = append(shared, si.T)
					continue
				}
				var sb strings.Builder
				for _, rr := range si.R {
					sb.WriteString(rr.T)
				}
				shared = append(shared, sb.String())
			}
		case f.Name == "xl/worksheets/sheet1.xml":
			sheetXML, err = planReadZip(f)
			if err != nil {
				return nil, err
			}
		}
	}
	if sheetXML == nil {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
				if sheetXML, err = planReadZip(f); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	if sheetXML == nil {
		return nil, fmt.Errorf("xlsx içinde çalışma sayfası bulunamadı")
	}
	var sheet planXLSXSheet
	if err := xml.Unmarshal(sheetXML, &sheet); err != nil {
		return nil, fmt.Errorf("sayfa çözümlenemedi: %w", err)
	}
	var out []importedPlanRow
	for i, row := range sheet.Rows {
		cells := make([]string, planImportCols)
		for _, c := range row.Cells {
			col := planColIndex(c.R)
			if col < 0 || col >= planImportCols {
				continue
			}
			var val string
			switch c.T {
			case "s":
				if idx, err := strconv.Atoi(strings.TrimSpace(c.V)); err == nil && idx >= 0 && idx < len(shared) {
					val = shared[idx]
				}
			case "inlineStr":
				val = c.IS.T
			default:
				val = c.V
			}
			cells[col] = strings.TrimSpace(val)
		}
		if i == 0 && planLooksLikeHeader(cells) {
			continue
		}
		if planAllEmpty(cells) {
			continue
		}
		out = append(out, planRowFromCells(cells))
	}
	return out, nil
}

func planReadZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func planColIndex(ref string) int {
	letters := ""
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			letters += string(ch)
		} else if ch >= 'a' && ch <= 'z' {
			letters += strings.ToUpper(string(ch))
		} else {
			break
		}
	}
	if letters == "" {
		return -1
	}
	n := 0
	for _, ch := range letters {
		n = n*26 + int(ch-'A'+1)
	}
	return n - 1
}

func planRowFromCells(cells []string) importedPlanRow {
	get := func(i int) string {
		if i < len(cells) {
			return strings.TrimSpace(cells[i])
		}
		return ""
	}
	return importedPlanRow{
		PozNo: get(0), Description: get(1), Category: get(2), Quantity: planParseNum(get(3)),
		Unit: get(4), Supplier: get(5), OrderDate: get(6), DeliveryDate: get(7),
		Criticality: get(8), Note: get(9),
	}
}

func planParseNum(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func planLooksLikeHeader(cells []string) bool {
	joined := strings.ToLower(strings.Join(cells, " "))
	return strings.Contains(joined, "poz") || strings.Contains(joined, "açıklama") ||
		strings.Contains(joined, "aciklama") || strings.Contains(joined, "tanım")
}

func planAllEmpty(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
