package costcodes

import (
	"io"
	"net/http"
	"strings"

	"github.com/ipks/ipks/backend/internal/audit"
	"github.com/ipks/ipks/backend/internal/auth"
	"github.com/ipks/ipks/backend/internal/httpx"
	"github.com/ipks/ipks/backend/internal/tabular"
)

// Maliyet kodu bütçeleri toplu içe aktarma.
// Sütun düzeni (ilk satır başlık ise atlanır): A=kod, B=bütçe_tutarı.
// Kod projede yoksa satır atlanır ve yanıtta listelenir (yeni kod oluşturulmaz).

const maxBudgetImportBytes = 10 << 20

// maxListed — yanıtta geri döndürülen en fazla hatalı/bilinmeyen satır sayısı.
const maxListed = 50

type importRow struct {
	Line   int    `json:"line"`
	Code   string `json:"code"`
	Reason string `json:"reason,omitempty"`
}

func (h *Handler) ImportBudgets(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Oturum gerekli.", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBudgetImportBytes+(1<<20))
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
	data, err := io.ReadAll(io.LimitReader(file, maxBudgetImportBytes))
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	rows, err := tabular.Read(hdr.Filename, data)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya okunamadı: "+err.Error(), nil)
		return
	}

	// Projedeki mevcut kodlar: kod → id
	codes := map[string]string{}
	crows, err := h.pool.Query(r.Context(), `SELECT code, id::text FROM cost_codes WHERE project_id=$1`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	for crows.Next() {
		var code, id string
		if err := crows.Scan(&code, &id); err == nil {
			codes[code] = id
		}
	}
	crows.Close()

	type pending struct {
		id     string
		amount float64
	}
	var writes []pending
	var unknown, invalid []importRow
	total := 0
	for i, cells := range rows {
		if tabular.IsEmpty(cells) {
			continue
		}
		code := tabular.Cell(cells, 0)
		raw := tabular.Cell(cells, 1)
		// Başlık satırı: kod sütunu "kod" içerir ve tutar sayısal değil
		if i == 0 && strings.Contains(strings.ToLower(code), "kod") {
			if _, numOK := tabular.ParseNum(raw); !numOK {
				continue
			}
		}
		total++
		line := i + 1
		if code == "" {
			invalid = append(invalid, importRow{Line: line, Reason: "kod boş"})
			continue
		}
		amount, ok := tabular.ParseNum(raw)
		if !ok || amount < 0 || amount > 999999999999 {
			invalid = append(invalid, importRow{Line: line, Code: code, Reason: "tutar geçersiz"})
			continue
		}
		id, found := codes[code]
		if !found {
			unknown = append(unknown, importRow{Line: line, Code: code, Reason: "bu projede böyle bir maliyet kodu yok"})
			continue
		}
		writes = append(writes, pending{id: id, amount: amount})
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())
	for _, p := range writes {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO cost_code_budgets (cost_code_id, project_id, amount, updated_by)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (cost_code_id) DO UPDATE
			  SET amount=EXCLUDED.amount, updated_by=EXCLUDED.updated_by, updated_at=now()`,
			p.id, pid, p.amount, uid); err != nil {
			httpx.Internal(w, r)
			return
		}
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "cost_code_budgets", EntityID: pid.String(), Action: audit.ActionUpdate,
		After: map[string]interface{}{"import": "bulk", "rows": len(writes), "unknown": len(unknown), "invalid": len(invalid)},
		IP:    m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"processed":     len(writes),
		"total":         total,
		"unknown_codes": capList(unknown),
		"invalid_rows":  capList(invalid),
	})
}

func capList(rows []importRow) []importRow {
	if len(rows) > maxListed {
		return rows[:maxListed]
	}
	return rows
}
