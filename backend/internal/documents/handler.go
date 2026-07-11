// Package documents — Faz 2 doküman motoru: klasör ağacı, dokümanlar, SHA-256
// doğrulamalı versiyonlama ve MinIO nesne depolama entegrasyonu (Plan §5.2).
//
// Güvenlik ilkesi: yükleme ve indirme DAİMA API üzerinden geçer; her istek izin
// (documents.*) + proje kapsamıyla korunur. Böylece "yetkisiz kullanıcı URL'i
// bilse dahi dosyaya erişemez" (Faz 2 kabul kriteri) — depolama anahtarı ve
// nesne uç noktası istemciye asla sızmaz.
package documents

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	"github.com/ipks/ipks/backend/internal/storage"
)

const maxUploadBytes = 100 << 20 // 100 MB

var docCategories = map[string]bool{
	"Contract": true, "Addendum": true, "Submittal": true, "Drawing": true,
	"Delivery": true, "OHS": true, "Other": true,
}

type Handler struct {
	pool  *pgxpool.Pool
	store *storage.Client
	rec   *audit.Recorder
	log   *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, store *storage.Client, rec *audit.Recorder, log *slog.Logger) *Handler {
	return &Handler{pool: pool, store: store, rec: rec, log: log}
}

// ---------------------------------------------------------------------------
// Klasörler
// ---------------------------------------------------------------------------

type folderDTO struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	Name        string     `json:"name"`
	ModuleScope *string    `json:"module_scope,omitempty"`
	RowVersion  int        `json:"row_version"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListFolders — projedeki tüm (yaşayan) klasörleri düz liste olarak döner;
// istemci ağacı parent_id üzerinden kurar.
func (h *Handler) ListFolders(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, project_id, parent_id, name, module_scope, row_version, created_at
		FROM folders WHERE project_id=$1 AND deleted_at IS NULL
		ORDER BY name`, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []folderDTO{}
	for rows.Next() {
		var f folderDTO
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.ParentID, &f.Name, &f.ModuleScope, &f.RowVersion, &f.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, f)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"folders": out})
}

type folderReq struct {
	Name        string  `json:"name"`
	ParentID    *string `json:"parent_id"`
	ModuleScope *string `json:"module_scope"`
	RowVersion  int     `json:"row_version"`
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req folderReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httpx.ValidationFailed(w, r, map[string]string{"name": "zorunlu"})
		return
	}
	parent, ok := h.resolveParent(w, r, pid, req.ParentID)
	if !ok {
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var f folderDTO
	err = tx.QueryRow(r.Context(), `
		INSERT INTO folders (project_id, parent_id, name, module_scope)
		VALUES ($1,$2,$3,$4)
		RETURNING id, project_id, parent_id, name, module_scope, row_version, created_at`,
		pid, parent, req.Name, req.ModuleScope).
		Scan(&f.ID, &f.ProjectID, &f.ParentID, &f.Name, &f.ModuleScope, &f.RowVersion, &f.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu klasörde aynı isim zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "folders", EntityID: f.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"project_id": pid, "name": f.Name}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"folder": f})
}

func (h *Handler) UpdateFolder(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	fid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req folderReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// Yeni üst klasör: kendisi olamaz, kendi alt ağacına taşınamaz (döngü koruması).
	var parent *uuid.UUID
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		p, ok := h.resolveParent(w, r, pid, req.ParentID)
		if !ok {
			return
		}
		if p != nil && *p == fid {
			httpx.ValidationFailed(w, r, map[string]string{"parent_id": "klasör kendi altına taşınamaz"})
			return
		}
		if p != nil && h.isDescendant(r, pid, *p, fid) {
			httpx.ValidationFailed(w, r, map[string]string{"parent_id": "döngü: hedef, bu klasörün alt ağacında"})
			return
		}
		parent = p
	}
	name := strings.TrimSpace(req.Name)

	ct, err := h.pool.Exec(r.Context(), `
		UPDATE folders SET
			name = COALESCE(NULLIF($3,''), name),
			parent_id = CASE WHEN $4::boolean THEN $5 ELSE parent_id END,
			row_version = row_version+1
		WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`,
		fid, pid, name, req.ParentID != nil, parent)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict, "Bu klasörde aynı isim zaten var.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Klasör bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "folders", EntityID: fid.String(), Action: audit.ActionUpdate,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	fid, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	// Yalnızca boş klasör silinebilir (alt klasör veya doküman yoksa).
	var children int
	if err := h.pool.QueryRow(r.Context(), `
		SELECT
		  (SELECT count(*) FROM folders   WHERE parent_id=$1 AND deleted_at IS NULL) +
		  (SELECT count(*) FROM documents WHERE folder_id=$1 AND deleted_at IS NULL)`,
		fid).Scan(&children); err != nil {
		httpx.Internal(w, r)
		return
	}
	if children > 0 {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Klasör boş değil; önce içeriğini taşıyın veya silin.", nil)
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE folders SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, fid, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Klasör bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "folders", EntityID: fid.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Dokümanlar
// ---------------------------------------------------------------------------

type documentDTO struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	FolderID      *uuid.UUID `json:"folder_id,omitempty"`
	EntityType    *string    `json:"entity_type,omitempty"`
	EntityID      *uuid.UUID `json:"entity_id,omitempty"`
	Title         string     `json:"title"`
	DocCategory   string     `json:"doc_category"`
	RowVersion    int        `json:"row_version"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	VersionCount  int        `json:"version_count"`
	LatestVersion *int       `json:"latest_version,omitempty"`
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	q := r.URL.Query()
	folderID := q.Get("folder_id")
	entityType := q.Get("entity_type")
	entityID := q.Get("entity_id")
	category := q.Get("category")

	rows, err := h.pool.Query(r.Context(), `
		SELECT d.id, d.project_id, d.folder_id, d.entity_type, d.entity_id, d.title,
		       d.doc_category, d.row_version, d.created_at, d.updated_at,
		       COALESCE(v.cnt,0), v.latest
		FROM documents d
		LEFT JOIN (
			SELECT document_id, count(*) cnt, max(version_no) latest
			FROM document_versions GROUP BY document_id
		) v ON v.document_id = d.id
		WHERE d.project_id=$1 AND d.deleted_at IS NULL
		  AND ($2='' OR d.folder_id=$2::uuid)
		  AND ($3='' OR d.entity_type=$3)
		  AND ($4='' OR d.entity_id=$4::uuid)
		  AND ($5='' OR d.doc_category=$5)
		ORDER BY d.updated_at DESC`,
		pid, folderID, entityType, entityID, category)
	if err != nil {
		h.log.Error("doküman listesi", "err", err)
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	out := []documentDTO{}
	for rows.Next() {
		var d documentDTO
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.FolderID, &d.EntityType, &d.EntityID, &d.Title,
			&d.DocCategory, &d.RowVersion, &d.CreatedAt, &d.UpdatedAt, &d.VersionCount, &d.LatestVersion); err != nil {
			httpx.Internal(w, r)
			return
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"documents": out})
}

type createDocumentReq struct {
	Title       string  `json:"title"`
	FolderID    *string `json:"folder_id"`
	DocCategory *string `json:"doc_category"`
	EntityType  *string `json:"entity_type"`
	EntityID    *string `json:"entity_id"`
}

func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	var req createDocumentReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		httpx.ValidationFailed(w, r, map[string]string{"title": "zorunlu"})
		return
	}
	category := "Other"
	if req.DocCategory != nil && *req.DocCategory != "" {
		category = *req.DocCategory
	}
	if !docCategories[category] {
		httpx.ValidationFailed(w, r, map[string]string{"doc_category": "geçersiz kategori"})
		return
	}
	folder, ok := h.resolveParent(w, r, pid, req.FolderID) // klasörün de projeye ait olduğunu doğrular
	if !ok {
		return
	}
	var entityID *uuid.UUID
	if req.EntityID != nil && strings.TrimSpace(*req.EntityID) != "" {
		e, err := uuid.Parse(strings.TrimSpace(*req.EntityID))
		if err != nil {
			httpx.ValidationFailed(w, r, map[string]string{"entity_id": "geçersiz UUID"})
			return
		}
		entityID = &e
	}
	uid, _ := auth.UserIDFrom(r.Context())

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var d documentDTO
	err = tx.QueryRow(r.Context(), `
		INSERT INTO documents (project_id, folder_id, entity_type, entity_id, title, doc_category, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, project_id, folder_id, entity_type, entity_id, title, doc_category,
		          row_version, created_at, updated_at`,
		pid, folder, req.EntityType, entityID, req.Title, category, uid).
		Scan(&d.ID, &d.ProjectID, &d.FolderID, &d.EntityType, &d.EntityID, &d.Title, &d.DocCategory,
			&d.RowVersion, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Klasör veya proje bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "documents", EntityID: d.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"title": d.Title, "category": d.DocCategory}, IP: m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"document": d})
}

type versionDTO struct {
	ID         uuid.UUID `json:"id"`
	VersionNo  int       `json:"version_no"`
	FileID     uuid.UUID `json:"file_id"`
	OriginalName string  `json:"original_name"`
	Mime       string    `json:"mime"`
	SizeBytes  int64     `json:"size_bytes"`
	SHA256     string    `json:"sha256"`
	Note       *string   `json:"note,omitempty"`
	UploadedBy *uuid.UUID `json:"uploaded_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	did, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var d documentDTO
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, project_id, folder_id, entity_type, entity_id, title, doc_category,
		       row_version, created_at, updated_at
		FROM documents WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, did, pid).
		Scan(&d.ID, &d.ProjectID, &d.FolderID, &d.EntityType, &d.EntityID, &d.Title, &d.DocCategory,
			&d.RowVersion, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Doküman bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT dv.id, dv.version_no, dv.file_id, f.original_name, f.mime, f.size_bytes,
		       dv.sha256, dv.note, dv.uploaded_by, dv.created_at
		FROM document_versions dv JOIN files f ON f.id = dv.file_id
		WHERE dv.document_id=$1 ORDER BY dv.version_no DESC`, did)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer rows.Close()
	versions := []versionDTO{}
	for rows.Next() {
		var v versionDTO
		if err := rows.Scan(&v.ID, &v.VersionNo, &v.FileID, &v.OriginalName, &v.Mime, &v.SizeBytes,
			&v.SHA256, &v.Note, &v.UploadedBy, &v.CreatedAt); err != nil {
			httpx.Internal(w, r)
			return
		}
		versions = append(versions, v)
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"document": d, "versions": versions})
}

type updateDocumentReq struct {
	Title       *string `json:"title"`
	FolderID    *string `json:"folder_id"`
	DocCategory *string `json:"doc_category"`
	RowVersion  int     `json:"row_version"`
}

func (h *Handler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	did, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateDocumentReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.DocCategory != nil && *req.DocCategory != "" && !docCategories[*req.DocCategory] {
		httpx.ValidationFailed(w, r, map[string]string{"doc_category": "geçersiz kategori"})
		return
	}
	var folder *uuid.UUID
	setFolder := false
	if req.FolderID != nil {
		setFolder = true
		if strings.TrimSpace(*req.FolderID) != "" {
			f, ok := h.resolveParent(w, r, pid, req.FolderID)
			if !ok {
				return
			}
			folder = f
		}
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	defer tx.Rollback(r.Context())

	var cur documentDTO
	err = tx.QueryRow(r.Context(), `
		SELECT id, project_id, folder_id, entity_type, entity_id, title, doc_category,
		       row_version, created_at, updated_at
		FROM documents WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL FOR UPDATE`, did, pid).
		Scan(&cur.ID, &cur.ProjectID, &cur.FolderID, &cur.EntityType, &cur.EntityID, &cur.Title, &cur.DocCategory,
			&cur.RowVersion, &cur.CreatedAt, &cur.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Doküman bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if req.RowVersion != 0 && req.RowVersion != cur.RowVersion {
		httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
			"Kayıt başkası tarafından güncellendi.", map[string]int{"current_version": cur.RowVersion})
		return
	}
	title := cur.Title
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		title = strings.TrimSpace(*req.Title)
	}
	category := cur.DocCategory
	if req.DocCategory != nil && *req.DocCategory != "" {
		category = *req.DocCategory
	}
	newFolder := cur.FolderID
	if setFolder {
		newFolder = folder
	}

	var d documentDTO
	err = tx.QueryRow(r.Context(), `
		UPDATE documents SET title=$3, doc_category=$4, folder_id=$5, row_version=row_version+1
		WHERE id=$1 AND project_id=$2
		RETURNING id, project_id, folder_id, entity_type, entity_id, title, doc_category,
		          row_version, created_at, updated_at`,
		did, pid, title, category, newFolder).
		Scan(&d.ID, &d.ProjectID, &d.FolderID, &d.EntityType, &d.EntityID, &d.Title, &d.DocCategory,
			&d.RowVersion, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if isFKViolation(err) {
			httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Klasör bulunamadı.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	m := audit.MetaFrom(r.Context())
	_ = h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "documents", EntityID: did.String(), Action: audit.ActionUpdate,
		Before: map[string]interface{}{"title": cur.Title, "category": cur.DocCategory},
		After:  map[string]interface{}{"title": d.Title, "category": d.DocCategory},
		IP:     m.IP, ReqID: m.ReqID,
	})
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"document": d})
}

func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	did, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ct, err := h.pool.Exec(r.Context(),
		`UPDATE documents SET deleted_at=now() WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL`, did, pid)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Doküman bulunamadı.", nil)
		return
	}
	m := audit.MetaFrom(r.Context())
	h.rec.Record(r.Context(), audit.Entry{
		ActorID: m.ActorID, Entity: "documents", EntityID: did.String(), Action: audit.ActionDelete,
		IP: m.IP, ReqID: m.ReqID,
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Versiyon yükleme (multipart) ve indirme (akış)
// ---------------------------------------------------------------------------

// UploadVersion — yeni doküman versiyonu yükler. Akış sırasında SHA-256 ve boyut
// hesaplanır; nesne MinIO'ya yazıldıktan SONRA DB kaydı atomik olarak eklenir.
// version_no = mevcut en yüksek + 1 (v1, v2, ...); eşzamanlı yüklemede benzersizlik
// kısıtı çakışmayı 409 ile yakalar.
func (h *Handler) UploadVersion(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	did, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	// Doküman projeye ait ve yaşıyor mu?
	var exists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM documents WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL)`,
		did, pid).Scan(&exists); err != nil {
		httpx.Internal(w, r)
		return
	}
	if !exists {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Doküman bulunamadı.", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Dosya çözümlenemedi (boyut sınırı 100 MB).", nil)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"file": "dosya alanı zorunlu"})
		return
	}
	defer file.Close()
	if hdr.Size > maxUploadBytes {
		httpx.Error(w, r, http.StatusRequestEntityTooLarge, httpx.CodeValidation, "Dosya 100 MB sınırını aşıyor.", nil)
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))

	// Faz 10 — yükleme güvenlik geçidi: tip doğrulama + (varsa) antivirüs.
	// Tarayıcının bildirdiği Content-Type'a güvenilmez; koklanmış değer saklanır.
	safeMime, verr := h.store.CheckUpload(file, hdr.Filename)
	if verr != nil {
		httpx.Error(w, r, http.StatusUnsupportedMediaType, httpx.CodeValidation,
			"Dosya kabul edilmedi: "+verr.Error(), nil)
		return
	}

	// SHA-256 + boyut (dosya ReadSeeker; başa sarıp yüklüyoruz).
	hsh := sha256.New()
	size, err := io.Copy(hsh, file)
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	sum := hex.EncodeToString(hsh.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		httpx.Internal(w, r)
		return
	}

	// Sıradaki versiyon numarası.
	var nextNo int
	if err := h.pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(version_no),0)+1 FROM document_versions WHERE document_id=$1`, did).
		Scan(&nextNo); err != nil {
		httpx.Internal(w, r)
		return
	}

	contentType := safeMime
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key := storage.BuildDocumentKey(pid.String(), did.String(), nextNo, hdr.Filename)

	// Önce nesneyi yaz (DB'den önce; başarısızsa yetim kayıt oluşmaz).
	if err := h.store.PutObject(r.Context(), key, contentType, file, size, sum); err != nil {
		h.log.Error("minio yükleme", "err", err, "key", key)
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Dosya deposuna yazılamadı.", nil)
		return
	}

	uid, _ := auth.UserIDFrom(r.Context())
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
		key, hdr.Filename, contentType, size, sum, uid).Scan(&fileID); err != nil {
		httpx.Internal(w, r)
		return
	}
	var v versionDTO
	err = tx.QueryRow(r.Context(), `
		INSERT INTO document_versions (document_id, version_no, file_id, uploaded_by, note, sha256)
		VALUES ($1,$2,$3,$4, NULLIF($5,''), $6)
		RETURNING id, version_no, file_id, uploaded_by, note, sha256, created_at`,
		did, nextNo, fileID, uid, note, sum).
		Scan(&v.ID, &v.VersionNo, &v.FileID, &v.UploadedBy, &v.Note, &v.SHA256, &v.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			httpx.Error(w, r, http.StatusConflict, httpx.CodeConflict,
				"Eşzamanlı versiyon çakışması; tekrar deneyin.", nil)
			return
		}
		httpx.Internal(w, r)
		return
	}
	v.OriginalName = hdr.Filename
	v.Mime = contentType
	v.SizeBytes = size

	// Dokümanı dokun (updated_at tazelensin, liste sıralaması güncellensin).
	_, _ = tx.Exec(r.Context(), `UPDATE documents SET row_version=row_version+1 WHERE id=$1`, did)

	m := audit.MetaFrom(r.Context())
	if err := h.rec.RecordTx(r.Context(), tx, audit.Entry{
		ActorID: m.ActorID, Entity: "document_versions", EntityID: v.ID.String(), Action: audit.ActionInsert,
		After: map[string]interface{}{"document_id": did, "version_no": nextNo, "sha256": sum, "size": size},
		IP:    m.IP, ReqID: m.ReqID,
	}); err != nil {
		httpx.Internal(w, r)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.Internal(w, r)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"version": v})
}

// DownloadVersion — belirli versiyonu izin+proje kontrolünden geçirip MinIO'dan
// akış olarak indirir. Depolama anahtarı asla istemciye dönmez.
func (h *Handler) DownloadVersion(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseID(w, r, "projectID")
	if !ok {
		return
	}
	did, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	no, err := strconv.Atoi(chi.URLParam(r, "versionNo"))
	if err != nil || no < 1 {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz versiyon numarası.", nil)
		return
	}
	var storageKey, originalName, mime string
	err = h.pool.QueryRow(r.Context(), `
		SELECT f.storage_key, f.original_name, f.mime
		FROM document_versions dv
		JOIN files f ON f.id = dv.file_id
		JOIN documents d ON d.id = dv.document_id
		WHERE dv.document_id=$1 AND dv.version_no=$2 AND d.project_id=$3 AND d.deleted_at IS NULL`,
		did, no, pid).Scan(&storageKey, &originalName, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Versiyon bulunamadı.", nil)
		return
	}
	if err != nil {
		httpx.Internal(w, r)
		return
	}
	obj, err := h.store.GetObject(r.Context(), storageKey)
	if err != nil {
		h.log.Error("minio indirme", "err", err, "key", storageKey)
		httpx.Error(w, r, http.StatusBadGateway, httpx.CodeInternal, "Dosya deposundan okunamadı.", nil)
		return
	}
	defer obj.Body.Close()

	if mime == "" {
		mime = obj.ContentType
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeHeader(originalName)+"\"")
	if obj.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, obj.Body)
}

// ---------------------------------------------------------------------------
// yardımcılar
// ---------------------------------------------------------------------------

// resolveParent — opsiyonel parent/folder UUID'sini çözer ve AYNI PROJEYE ait
// olduğunu doğrular. nil dönerse (kök) veya geçerli klasör → ok=true.
func (h *Handler) resolveParent(w http.ResponseWriter, r *http.Request, pid uuid.UUID, raw *string) (*uuid.UUID, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		httpx.ValidationFailed(w, r, map[string]string{"parent_id": "geçersiz UUID"})
		return nil, false
	}
	var exists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM folders WHERE id=$1 AND project_id=$2 AND deleted_at IS NULL)`,
		id, pid).Scan(&exists); err != nil {
		httpx.Internal(w, r)
		return nil, false
	}
	if !exists {
		httpx.ValidationFailed(w, r, map[string]string{"parent_id": "klasör bu projede bulunamadı"})
		return nil, false
	}
	return &id, true
}

// isDescendant — cand, root'un alt ağacında mı? (klasör taşımada döngü koruması)
func (h *Handler) isDescendant(r *http.Request, pid, cand, root uuid.UUID) bool {
	var yes bool
	_ = h.pool.QueryRow(r.Context(), `
		WITH RECURSIVE sub AS (
			SELECT id FROM folders WHERE id=$2 AND project_id=$1 AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM folders f JOIN sub ON f.parent_id = sub.id
			WHERE f.deleted_at IS NULL
		)
		SELECT EXISTS(SELECT 1 FROM sub WHERE id=$3)`, pid, root, cand).Scan(&yes)
	return yes
}

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func parseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeValidation, "Geçersiz kimlik.", map[string]string{param: "geçersiz UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func isUniqueViolation(err error) bool { return sqlState(err) == "23505" }
func isFKViolation(err error) bool     { return sqlState(err) == "23503" }

func sqlState(err error) string {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState()
	}
	return ""
}
