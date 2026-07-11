package materials

import (
	"encoding/csv"
	"net/http"
	"time"
)

// ExportRegister — MAR kayıt defteri dışa aktarımı (Plan Faz 5).
// CSV; Excel uyumu için UTF-8 BOM ve ';' ayırıcı (TR bölge ayarı).
// Kapsam filtreleri (taşeron / client kısıtı) listedekiyle birebir aynıdır:
// kullanıcı dışa aktarımda göremeyeceği satırı da göremez.
func (h *Handler) ExportRegister(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	sc, ok := h.requireScope(w, r, pid)
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), marSelect+`
		WHERE m.project_id=$1 AND m.deleted_at IS NULL
		  AND ($2::uuid IS NULL OR m.subcontractor_id=$2)
		  AND ($3 = false OR m.status <> 'Submitted')
		ORDER BY m.mar_no`, pid, sc.sub, sc.clientLimited)
	if err != nil {
		http.Error(w, "kayıt defteri okunamadı", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="mar-kayit-defteri.csv"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM: Excel'de Türkçe karakterler

	cw := csv.NewWriter(w)
	cw.Comma = ';'
	_ = cw.Write([]string{"MAR No", "Malzeme", "Şartname Ref", "Üretici", "Taşeron",
		"Durum", "Sunan", "Sunum Tarihi", "Karar Veren", "Karar Tarihi", "Karar Notu", "Ek Sayısı"})

	fmtT := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.Format("02.01.2006 15:04")
	}
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	for rows.Next() {
		var m marDTO
		if err := scanMAR(rows, &m); err != nil {
			continue
		}
		created := m.CreatedAt
		_ = cw.Write([]string{
			m.MARNo, m.MaterialName, deref(m.SpecRef), deref(m.Manufacturer),
			deref(m.SubcontractorName), statusLabel(m.Status),
			m.CreatedByName, created.Format("02.01.2006 15:04"),
			deref(m.DecidedByName), fmtT(m.DecidedAt), deref(m.DecisionNote),
			itoa(m.AttachmentCount),
		})
	}
	cw.Flush()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
