// Package ohs — Faz 8 İSG modülü.
//
// Dört alt alan: checklist şablonları (admin tanımlar, items JSONB — kontrol
// tanımı VERİDİR, Plan §7), mobil denetimler (offline kuyruk uyumlu),
// bulgular (foto+GPS, Open→InProgress→Closed, termin takibi) ve ceza tutanağı
// otomasyonu (anında PDF + bildirim + sonraki hakedişte kesinti önerisi).
//
// Desen Faz 3–7 ile birebir: audit'li yazımlar, workflow_transitions,
// DB seviyesinde değişmezlik kilidi (23001 → 409), satır seviyesi güvenlik
// (taşeron temsilcisi yalnızca kendi firmasının bulgu/cezasını görür).
package ohs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"

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
}

func NewHandler(pool *pgxpool.Pool, eval *rbac.Evaluator, rec *audit.Recorder,
	nt *notify.Service, store *storage.Client, log *slog.Logger) *Handler {
	return &Handler{pool: pool, eval: eval, rec: rec, nt: nt, store: store, log: log}
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.",
			map[string]string{param: "geçersiz UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func sqlState(err error) string {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState()
	}
	return ""
}

func isUniqueViolation(err error) bool { return sqlState(err) == "23505" }
func isLockViolation(err error) bool   { return sqlState(err) == "23001" }
func isFKViolation(err error) bool     { return sqlState(err) == "23503" }

// scopedSub — satır seviyesi güvenlik (Plan §4): taşeron temsilcisi yalnızca
// kendi firmasının kayıtlarını görür; filtre backend'de ZORUNLU uygulanır.
func (h *Handler) scopedSub(ctx context.Context, uid, pid uuid.UUID) (*uuid.UUID, error) {
	var sub *uuid.UUID
	err := h.pool.QueryRow(ctx, `
		SELECT subcontractor_id FROM project_members
		WHERE user_id=$1 AND project_id=$2 AND deleted_at IS NULL
		  AND subcontractor_id IS NOT NULL
		LIMIT 1`, uid, pid).Scan(&sub)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (h *Handler) requireScope(w http.ResponseWriter, r *http.Request, pid uuid.UUID) (uuid.UUID, *uuid.UUID, bool) {
	uid, ok := auth.UserIDFrom(r.Context())
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized, "Kimlik doğrulama gerekli.", nil)
		return uuid.Nil, nil, false
	}
	sub, err := h.scopedSub(r.Context(), uid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return uuid.Nil, nil, false
	}
	return uid, sub, true
}

// notifyByCapability — proje üyeleri arasında ilgili izne (rol varsayılanına)
// sahip olanlara bildirim gönderir; aktör hariç (MAR/satınalma ile aynı desen).
func (h *Handler) notifyByCapability(ctx context.Context, pid uuid.UUID, permCode string, actor uuid.UUID, in notify.Input) {
	rows, err := h.pool.Query(ctx, `
		SELECT pm.user_id, r.code FROM project_members pm
		JOIN roles r ON r.id = pm.role_id
		WHERE pm.project_id=$1 AND pm.deleted_at IS NULL`, pid)
	if err != nil {
		h.log.Error("İSG bildirim hedefleri okunamadı", "err", err)
		return
	}
	defer rows.Close()
	var targets []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		var role string
		if rows.Scan(&uid, &role) != nil {
			continue
		}
		if uid == actor {
			continue
		}
		if rbac.RoleHasDefault(role, permCode) {
			targets = append(targets, uid)
		}
	}
	if len(targets) == 0 {
		return
	}
	in.UserIDs = targets
	h.nt.Send(ctx, in)
}

// notifySubReps — taşeronun bu projedeki temsilcilerine bildirim gönderir.
func (h *Handler) notifySubReps(ctx context.Context, pid, subID uuid.UUID, in notify.Input) {
	rows, err := h.pool.Query(ctx, `
		SELECT user_id FROM project_members
		WHERE project_id=$1 AND subcontractor_id=$2 AND deleted_at IS NULL`, pid, subID)
	if err != nil {
		h.log.Error("taşeron temsilcileri okunamadı", "err", err)
		return
	}
	defer rows.Close()
	var targets []uuid.UUID
	for rows.Next() {
		var uid uuid.UUID
		if rows.Scan(&uid) == nil {
			targets = append(targets, uid)
		}
	}
	if len(targets) == 0 {
		return
	}
	in.UserIDs = targets
	h.nt.Send(ctx, in)
}

// --- base64 foto → doküman motoru köprüsü ---------------------------------
//
// Offline kuyruk v1 yalnızca JSON gövde taşır (Faz 6 kararı). Sahada çekilen
// fotoğrafın çevrimdışı kuyruklanabilmesi için bulgu/kanıt fotoğrafı istek
// gövdesinde data-URL (base64) olarak kabul edilir ve TEK istekte Faz 2
// doküman motoruna (documents + document_versions + files + MinIO) yazılır.
// Sınır: 8 MB ham veri (istemci tarafı sıkıştırma önerilir, Plan §10).

const maxPhotoBytes = 8 << 20

var errPhotoTooLarge = errors.New("fotoğraf 8 MB sınırını aşıyor")
var errPhotoInvalid = errors.New("fotoğraf çözümlenemedi (data-URL bekleniyor)")

// savePhotoDocument — data-URL'i çözer, MinIO'ya yazar ve doküman kaydı açar.
// Nesne önce depoya yazılır (Faz 2 deseni: DB'den önce; başarısızsa yetim
// kayıt oluşmaz), doküman + versiyon + file satırları çağıranın tx'i içinde açılır.
func (h *Handler) savePhotoDocument(ctx context.Context, tx pgx.Tx, pid, uploadedBy uuid.UUID,
	title, category, entityType string, entityID uuid.UUID, dataURL, name string) (uuid.UUID, error) {

	mime, raw, err := decodeDataURL(dataURL)
	if err != nil {
		return uuid.Nil, err
	}
	if len(raw) > maxPhotoBytes {
		return uuid.Nil, errPhotoTooLarge
	}
	if name == "" {
		name = "foto.jpg"
	}
	// Faz 10 — yükleme güvenlik geçidi: koklanmış tip + (varsa) antivirüs.
	// İstemcinin data-URL'de bildirdiği MIME'e güvenilmez.
	safeMime, verr := h.store.CheckUpload(bytes.NewReader(raw), name)
	if verr != nil {
		return uuid.Nil, verr
	}
	mime = safeMime
	sum := sha256Hex(raw)

	var docID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO documents (project_id, title, doc_category, entity_type, entity_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		pid, title, category, entityType, entityID, uploadedBy).Scan(&docID); err != nil {
		return uuid.Nil, err
	}
	key := storage.BuildDocumentKey(pid.String(), docID.String(), 1, name)
	if err := h.store.PutObject(ctx, key, mime, bytes.NewReader(raw), int64(len(raw)), sum); err != nil {
		return uuid.Nil, err
	}
	var fileID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO files (storage_key, original_name, mime, size_bytes, sha256, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		key, name, mime, len(raw), sum, uploadedBy).Scan(&fileID); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO document_versions (document_id, version_no, file_id, uploaded_by, note, sha256)
		VALUES ($1,1,$2,$3,'İSG saha fotoğrafı',$4)`,
		docID, fileID, uploadedBy, sum); err != nil {
		return uuid.Nil, err
	}
	return docID, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// decodeDataURL — "data:image/jpeg;base64,..." biçimini çözer.
func decodeDataURL(s string) (mime string, raw []byte, err error) {
	if !strings.HasPrefix(s, "data:") {
		return "", nil, errPhotoInvalid
	}
	rest := s[len("data:"):]
	semi := strings.Index(rest, ";base64,")
	if semi < 0 {
		return "", nil, errPhotoInvalid
	}
	mime = rest[:semi]
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		return "", nil, errPhotoInvalid
	}
	raw, err = base64.StdEncoding.DecodeString(rest[semi+len(";base64,"):])
	if err != nil {
		return "", nil, errPhotoInvalid
	}
	return mime, raw, nil
}
