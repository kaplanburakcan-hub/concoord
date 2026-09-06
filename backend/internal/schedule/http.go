package schedule

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ipks/ipks/backend/internal/httpx"
)

// writeErr — repository katmanının sentinel hatalarını (items.go,
// dependencies.go) HTTP durum koduna çevirir. Bilinmeyen hata = 500.
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Kayıt bulunamadı.", nil)
	case errors.Is(err, ErrConflict):
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Kayıt sizden önce değişmiş, sayfayı yenileyip tekrar deneyin.", nil)
	case errors.Is(err, ErrHasChildren):
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "Alt kalemleri olan bir kalem silinemez.", nil)
	case errors.Is(err, ErrCrossProject):
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "Kayıt başka bir projeye ait.", nil)
	case errors.Is(err, ErrCycle):
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "Bu bağımlılık döngüsel bir zincir oluşturur.", nil)
	case errors.Is(err, ErrScheduleNotEmpty):
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu proje için zaten bir iş programı var; öneri yalnızca boş bir iş programında kullanılabilir.", nil)
	case errors.Is(err, ErrNoSurveyItems):
		httpx.Error(w, r, http.StatusUnprocessableEntity, httpx.CodeValidation, "Projede henüz keşif kalemi yok.", nil)
	default:
		httpx.Internal(w, r)
	}
}

// GET /projects/{projectID}/schedule — WBS ağacı + istek anında hesaplanmış ilerleme.
func (h *Handler) ListSchedule(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	items, err := h.ListItems(r.Context(), pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /projects/{projectID}/schedule/items
func (h *Handler) CreateItemHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req ItemReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := req.Validate(); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	it, err := h.CreateItem(r.Context(), pid, uid, req)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"item": it})
}

type updateItemReq struct {
	ItemReq
	RowVersion int `json:"row_version"`
}

// PATCH /schedule/items/{itemId}
func (h *Handler) UpdateItemHTTP(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseID(w, r, "itemId")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req updateItemReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if f := req.Validate(); len(f) > 0 {
		httpx.ValidationFailed(w, r, f)
		return
	}
	it, err := h.UpdateItem(r.Context(), itemID, uid, req.RowVersion, req.ItemReq)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"item": it})
}

// DELETE /schedule/items/{itemId}
func (h *Handler) DeleteItemHTTP(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseID(w, r, "itemId")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := h.DeleteItem(r.Context(), itemID, uid); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type pozLinkReq struct {
	PozID  uuid.UUID `json:"poz_id"`
	Action string    `json:"action"` // "link" | "unlink"
}

// POST /schedule/items/{itemId}/pozlar
func (h *Handler) LinkPozHTTP(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseID(w, r, "itemId")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req pozLinkReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.PozID == uuid.Nil {
		httpx.ValidationFailed(w, r, map[string]string{"poz_id": "Zorunlu."})
		return
	}
	var err error
	if req.Action == "unlink" {
		err = h.UnlinkPoz(r.Context(), itemID, req.PozID, uid)
	} else {
		err = h.LinkPoz(r.Context(), itemID, req.PozID, uid)
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /projects/{projectID}/schedule/available-pozlar
func (h *Handler) ListAvailablePozlarHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	pozlar, err := h.ListAvailablePozlar(r.Context(), pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"pozlar": pozlar})
}

type dependencyReq struct {
	PredecessorID uuid.UUID `json:"predecessor_id"`
	SuccessorID   uuid.UUID `json:"successor_id"`
	DepType       string    `json:"dep_type"`
	LagDays       int       `json:"lag_days"`
}

// POST /projects/{projectID}/schedule/dependencies
func (h *Handler) CreateDependencyHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req dependencyReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.PredecessorID == uuid.Nil || req.SuccessorID == uuid.Nil {
		httpx.ValidationFailed(w, r, map[string]string{"predecessor_id": "predecessor_id ve successor_id zorunlu."})
		return
	}
	dep, err := h.CreateDependency(r.Context(), pid, uid, req.PredecessorID, req.SuccessorID, req.DepType, req.LagDays)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"dependency": dep})
}

// GET /projects/{projectID}/schedule/dependencies
func (h *Handler) ListDependenciesHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	deps, err := h.ListDependencies(r.Context(), pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"dependencies": deps})
}

// DELETE /schedule/dependencies/{predId}/{succId}
func (h *Handler) DeleteDependencyHTTP(w http.ResponseWriter, r *http.Request) {
	predID, ok := parseID(w, r, "predId")
	if !ok {
		return
	}
	succID, ok := parseID(w, r, "succId")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := h.DeleteDependency(r.Context(), uid, predID, succID); err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type baselineReq struct {
	Note *string `json:"note"`
}

// POST /projects/{projectID}/schedule/baseline
func (h *Handler) FreezeBaselineHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	var req baselineReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	b, err := h.FreezeBaseline(r.Context(), pid, uid, req.Note)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"baseline": b})
}

// GET /projects/{projectID}/schedule/baselines
func (h *Handler) ListBaselinesHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	list, err := h.ListBaselines(r.Context(), pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"baselines": list})
}

// GET /schedule/baselines/{baselineId} — donmuş anlık görüntünün tamamı
// (Gantt'ta gölge çubuk karşılaştırması için; DÜZELTME: spec'te ayrı
// listelenmemişti ama frontend'in "dondurulmuş revizyonu seçince göster"
// ihtiyacı için gerekli).
func (h *Handler) GetBaselineHTTP(w http.ResponseWriter, r *http.Request) {
	bid, ok := parseID(w, r, "baselineId")
	if !ok {
		return
	}
	b, err := h.GetBaseline(r.Context(), bid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"baseline": b})
}

// GET /projects/{projectID}/schedule/survey-preview
func (h *Handler) SurveyPreviewHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	plan, err := h.PreviewSurveyPlan(r.Context(), pid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"categories": plan})
}

// POST /projects/{projectID}/schedule/generate-from-survey
func (h *Handler) GenerateFromSurveyHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	uid, ok := requireUser(w, r)
	if !ok {
		return
	}
	items, err := h.GenerateFromSurvey(r.Context(), pid, uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"items": items})
}

// GET /projects/{projectID}/schedule/s-curve?from=&to=&bucket=week|month
func (h *Handler) SCurveHTTP(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	bucket := r.URL.Query().Get("bucket")
	if bucket != "week" {
		bucket = "month"
	}

	parseDate := func(s string, fallback time.Time) time.Time {
		if s == "" {
			return fallback
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return fallback
		}
		return t
	}
	now := time.Now().UTC()
	from := parseDate(r.URL.Query().Get("from"), now.AddDate(-1, 0, 0))
	to := parseDate(r.URL.Query().Get("to"), now)

	points, err := h.ComputeSCurve(r.Context(), pid, from, to, bucket)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"points": points})
}
