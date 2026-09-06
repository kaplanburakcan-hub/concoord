package schedule

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/audit"
)

var (
	ErrScheduleNotEmpty = errors.New("proje için zaten bir iş programı var")
	ErrNoSurveyItems    = errors.New("projede keşif kalemi yok")
)

// kategoriPriority — sabit inşaat-mantığı sırası: yapı → cephe/çatı (kabuk) →
// mimari ince işler → tesisat → peyzaj. Bilinmeyen kategoriler Peyzaj'dan
// sonra, Diğer'den önce (alfabetik) sıralanır; Diğer her zaman en sonda.
var kategoriPriority = map[string]int{
	"Betonarme": 1,
	"Cephe":     2,
	"Çatı":      3,
	"Mimari":    4,
	"Mekanik":   5,
	"Elektrik":  6,
	"Peyzaj":    7,
	"Diğer":     9,
}

const unknownKategoriPriority = 8

func kategoriRank(k string) int {
	if p, ok := kategoriPriority[k]; ok {
		return p
	}
	return unknownKategoriPriority
}

// SurveyItem — BuildSurveyPlan'ın saf girdisi (project_survey_items'tan).
type SurveyItem struct {
	ID         uuid.UUID
	Kategori   string
	Tanim      string
	Sira       int
	Miktar     float64
	BirimFiyat float64
	WorkItemID *uuid.UUID // atanmışsa (taşerona bağlıysa) doludur
}

type PlannedItem struct {
	SurveyItemID uuid.UUID  `json:"survey_item_id"`
	Name         string     `json:"name"`
	Weight       float64    `json:"weight"`
	WorkItemID   *uuid.UUID `json:"work_item_id"`
}

type PlannedCategory struct {
	Name   string        `json:"name"`
	Weight float64       `json:"weight"`
	Items  []PlannedItem `json:"items"`
}

// BuildSurveyPlan — saf fonksiyon (DB'siz). Keşif kalemlerini kategoriye
// göre gruplar, kategorileri sabit inşaat-mantığı sırasına, kalemleri
// kategori içinde `sira`ya (eşitse tanım'a) göre sıralar. Ağırlık = miktar ×
// birim_fiyat; kategori ağırlığı kendi kalemlerinin toplamıdır.
func BuildSurveyPlan(items []SurveyItem) []PlannedCategory {
	byKategori := make(map[string][]SurveyItem)
	var kategoriler []string
	for _, it := range items {
		if _, ok := byKategori[it.Kategori]; !ok {
			kategoriler = append(kategoriler, it.Kategori)
		}
		byKategori[it.Kategori] = append(byKategori[it.Kategori], it)
	}

	sort.SliceStable(kategoriler, func(i, j int) bool {
		ri, rj := kategoriRank(kategoriler[i]), kategoriRank(kategoriler[j])
		if ri != rj {
			return ri < rj
		}
		return kategoriler[i] < kategoriler[j]
	})

	out := make([]PlannedCategory, 0, len(kategoriler))
	for _, k := range kategoriler {
		group := byKategori[k]
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Sira != group[j].Sira {
				return group[i].Sira < group[j].Sira
			}
			return group[i].Tanim < group[j].Tanim
		})

		var catWeight float64
		plannedItems := make([]PlannedItem, 0, len(group))
		for _, it := range group {
			w := it.Miktar * it.BirimFiyat
			catWeight += w
			plannedItems = append(plannedItems, PlannedItem{
				SurveyItemID: it.ID, Name: it.Tanim, Weight: w, WorkItemID: it.WorkItemID,
			})
		}
		out = append(out, PlannedCategory{Name: k, Weight: catWeight, Items: plannedItems})
	}
	return out
}

func (h *Handler) loadSurveyItems(ctx context.Context, projectID uuid.UUID) ([]SurveyItem, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT id, kategori, tanim, sira, miktar::float8, birim_fiyat::float8, work_item_id
		FROM project_survey_items WHERE project_id=$1`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SurveyItem{}
	for rows.Next() {
		var it SurveyItem
		if err := rows.Scan(&it.ID, &it.Kategori, &it.Tanim, &it.Sira, &it.Miktar, &it.BirimFiyat, &it.WorkItemID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (h *Handler) scheduleItemCount(ctx context.Context, projectID uuid.UUID) (int, error) {
	var n int
	err := h.pool.QueryRow(ctx,
		`SELECT count(*) FROM schedule_items WHERE project_id=$1 AND deleted_at IS NULL`, projectID,
	).Scan(&n)
	return n, err
}

// PreviewSurveyPlan — hiçbir şey yazmadan önerilen planı döner (frontend
// önizleme modalı için).
func (h *Handler) PreviewSurveyPlan(ctx context.Context, projectID uuid.UUID) ([]PlannedCategory, error) {
	n, err := h.scheduleItemCount(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrScheduleNotEmpty
	}
	items, err := h.loadSurveyItems(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNoSurveyItems
	}
	return BuildSurveyPlan(items), nil
}

// GenerateFromSurvey — planı gerçekten yazar: her kategori bir kök WBS
// kalemi, her keşif kalemi bir alt kalem olur. work_item_id'si olan
// (taşerona atanmış) kalemler progress_source='derived' olarak ve
// schedule_item_pozlar'a doğrudan bağlanarak oluşturulur — ilerlemesi
// gerçek hakedişten türetilir; atanmamışlar 'manual', %0'dan başlar.
// Tek transaction; tüm işlem için TEK bir audit girişi (baseline dondurma
// ile aynı ilke — kalem başına değil, işlem başına bir kayıt).
func (h *Handler) GenerateFromSurvey(ctx context.Context, projectID, actorID uuid.UUID) ([]ItemDTO, error) {
	plan, err := h.PreviewSurveyPlan(ctx, projectID)
	if err != nil {
		return nil, err
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	itemCount := 0
	for catIdx, cat := range plan {
		wbsCode := strconv.Itoa(catIdx + 1)
		var catID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO schedule_items (project_id, parent_id, wbs_code, name, weight, progress_source)
			VALUES ($1, NULL, $2, $3, $4, 'manual')
			RETURNING id`, projectID, wbsCode, cat.Name, cat.Weight,
		).Scan(&catID); err != nil {
			return nil, err
		}

		for itemIdx, it := range cat.Items {
			childWbs := wbsCode + "." + strconv.Itoa(itemIdx+1)
			progressSource := "manual"
			if it.WorkItemID != nil {
				progressSource = "derived"
			}
			var childID uuid.UUID
			if err := tx.QueryRow(ctx, `
				INSERT INTO schedule_items (project_id, parent_id, wbs_code, name, weight, progress_source)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id`, projectID, catID, childWbs, it.Name, it.Weight, progressSource,
			).Scan(&childID); err != nil {
				return nil, err
			}
			itemCount++

			if it.WorkItemID != nil {
				if _, err := tx.Exec(ctx, `
					INSERT INTO schedule_item_pozlar (schedule_item_id, poz_id) VALUES ($1,$2)
					ON CONFLICT DO NOTHING`, childID, *it.WorkItemID,
				); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	meta := audit.MetaFrom(ctx)
	h.rec.Record(ctx, audit.Entry{
		ActorID: actorID.String(), Entity: "schedule_items", EntityID: projectID.String(), Action: audit.ActionInsert,
		After: map[string]any{"kaynak": "proje_kesfi", "kategori_sayisi": len(plan), "kalem_sayisi": itemCount},
		IP:    meta.IP, ReqID: meta.ReqID,
	})

	return h.ListItems(ctx, projectID)
}
