package documents

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// moduleRecordDTO — diğer modüllerde (sözleşme, malzeme onayı, irsaliye, İSG
// denetimi, saha tutanağı) bulunan ve Dokümanlar kategorileriyle eşleşen
// kayıtlar. Bunlar documents tablosunda bir satır değildir; yalnızca listede
// gösterilir ve kaynak modüle bağlanır. Dosyası olan kayıtlar dokümanlar
// bölümünden indirilir.
type moduleRecordDTO struct {
	Source      string    `json:"source"`
	SourceLabel string    `json:"source_label"`
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	DocCategory string    `json:"doc_category"`
	Status      string    `json:"status,omitempty"`
	Date        time.Time `json:"date"`
	Link        string    `json:"link"`
}

// moduleRecords — istenen kategoriye (boşsa tümü) düşen modül kayıtlarını döner.
func (h *Handler) moduleRecords(ctx context.Context, pid uuid.UUID, category string) ([]moduleRecordDTO, error) {
	out := []moduleRecordDTO{}
	want := func(cat string) bool { return category == "" || category == cat }

	if want("Contract") {
		rows, err := h.pool.Query(ctx, `
			SELECT id, created_at FROM project_main_contracts WHERE project_id=$1`, pid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r moduleRecordDTO
			if err := rows.Scan(&r.ID, &r.Date); err != nil {
				rows.Close()
				return nil, err
			}
			r.Source, r.SourceLabel, r.DocCategory = "main_contract", "Ana Sözleşme", "Contract"
			r.Title, r.Link = "Ana Sözleşme", "/proje/ana-sozlesme"
			out = append(out, r)
		}
		rows.Close()
	}

	if want("Contract") || want("Addendum") {
		types := []string{}
		if want("Contract") {
			types = append(types, "Main", "Sub")
		}
		if want("Addendum") {
			types = append(types, "Addendum")
		}
		rows, err := h.pool.Query(ctx, `
			SELECT c.id, c.type, c.contract_no, COALESCE(s.company_name, ''), c.created_at
			FROM contracts c
			LEFT JOIN subcontractors s ON s.id = c.subcontractor_id
			WHERE c.project_id=$1 AND c.deleted_at IS NULL AND c.type = ANY($2)
			ORDER BY c.created_at DESC`, pid, types)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r moduleRecordDTO
			var typ, no, company string
			if err := rows.Scan(&r.ID, &typ, &no, &company, &r.Date); err != nil {
				rows.Close()
				return nil, err
			}
			r.Title = no
			if company != "" {
				r.Title = no + " — " + company
			}
			r.Source, r.SourceLabel, r.Link = "contract", "Taşeron Sözleşmesi", "/taseronlar"
			r.DocCategory = "Contract"
			if typ == "Addendum" {
				r.DocCategory = "Addendum"
				r.SourceLabel = "Zeyilname"
			}
			out = append(out, r)
		}
		rows.Close()
	}

	if want("Submittal") {
		rows, err := h.pool.Query(ctx, `
			SELECT id, mar_no, material_name, status, created_at
			FROM material_approvals
			WHERE project_id=$1 AND deleted_at IS NULL
			ORDER BY created_at DESC`, pid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r moduleRecordDTO
			var no, name string
			if err := rows.Scan(&r.ID, &no, &name, &r.Status, &r.Date); err != nil {
				rows.Close()
				return nil, err
			}
			r.Title = no + " — " + name
			r.Source, r.SourceLabel, r.DocCategory = "mar", "Malzeme Onayı", "Submittal"
			r.Link = "/malzeme-onaylari"
			out = append(out, r)
		}
		rows.Close()
	}

	if want("Delivery") {
		rows, err := h.pool.Query(ctx, `
			SELECT d.id, d.delivery_note_no, po.supplier_name, d.delivered_at
			FROM deliveries d
			JOIN purchase_orders po ON po.id = d.po_id
			WHERE po.project_id=$1 AND d.deleted_at IS NULL
			ORDER BY d.delivered_at DESC`, pid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r moduleRecordDTO
			var no, supplier string
			if err := rows.Scan(&r.ID, &no, &supplier, &r.Date); err != nil {
				rows.Close()
				return nil, err
			}
			r.Title = "İrsaliye " + no + " — " + supplier
			r.Source, r.SourceLabel, r.DocCategory = "delivery", "Mal Kabul", "Delivery"
			r.Link = "/satinalma/siparisler"
			out = append(out, r)
		}
		rows.Close()
	}

	if want("OHS") {
		rows, err := h.pool.Query(ctx, `
			SELECT i.id, t.name, i.inspected_at
			FROM ohs_inspections i
			JOIN ohs_checklist_templates t ON t.id = i.template_id
			WHERE i.project_id=$1 AND i.deleted_at IS NULL
			ORDER BY i.inspected_at DESC`, pid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r moduleRecordDTO
			var tmpl string
			if err := rows.Scan(&r.ID, &tmpl, &r.Date); err != nil {
				rows.Close()
				return nil, err
			}
			r.Title = "İSG denetimi — " + tmpl
			r.Source, r.SourceLabel, r.DocCategory = "ohs_inspection", "İSG Denetimi", "OHS"
			r.Link = "/isg/denetimler"
			out = append(out, r)
		}
		rows.Close()
	}

	if want("Other") {
		rows, err := h.pool.Query(ctx, `
			SELECT id, tip, baslik, tarih
			FROM saha_tutanaklari
			WHERE project_id=$1 AND deleted_at IS NULL
			ORDER BY tarih DESC`, pid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var r moduleRecordDTO
			var tip string
			if err := rows.Scan(&r.ID, &tip, &r.Title, &r.Date); err != nil {
				rows.Close()
				return nil, err
			}
			r.Source, r.SourceLabel, r.DocCategory = "saha_tutanagi", "Saha Tutanağı", "Other"
			r.Status = tip
			r.Link = "/saha/tutanaklar"
			out = append(out, r)
		}
		rows.Close()
	}

	return out, nil
}
