// Package audit — merkezi denetim izi (audit trail) iskeleti.
//
// İlke (Plan §5.1): Audit kaydı repository katmanında ZORUNLU olarak alınır;
// geliştirici unutamaz. Faz 0'da altyapı kurulur (tablo + Recorder + HTTP
// context'i); Faz 1+ modülleri her yazma işleminde Recorder.Record çağırır.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ipks/ipks/backend/internal/httpx"
)

type Action string

const (
	ActionInsert Action = "INSERT"
	ActionUpdate Action = "UPDATE"
	ActionDelete Action = "DELETE" // soft delete dahil
)

type Entry struct {
	ActorID  string // boş olabilir (sistem/worker işleri)
	Entity   string // örn. "progress_payments"
	EntityID string
	Action   Action
	Before   interface{} // JSONB'ye serileştirilir
	After    interface{}
	IP       string
	ReqID    string
}

type Recorder struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewRecorder(pool *pgxpool.Pool, log *slog.Logger) *Recorder {
	return &Recorder{pool: pool, log: log}
}

// Record — denetim kaydını yazar. Ana transaction'ı bozmamak için hata
// yutulmaz ama isteği de düşürmez: loglanır ve metrik konusu olur.
// (Faz 1'de aynı tx içinde çalışan RecordTx varyantı eklenir.)
func (r *Recorder) Record(ctx context.Context, e Entry) {
	before, _ := json.Marshal(e.Before)
	after, _ := json.Marshal(e.After)
	if e.Before == nil {
		before = nil
	}
	if e.After == nil {
		after = nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, entity, entity_id, action, before, after, ip, request_id)
		VALUES (NULLIF($1,'')::uuid, $2, NULLIF($3,'')::uuid, $4, $5, $6, NULLIF($7,''), NULLIF($8,''))`,
		e.ActorID, e.Entity, e.EntityID, string(e.Action), before, after, e.IP, e.ReqID)
	if err != nil {
		r.log.Error("audit yazılamadı", "err", err, "entity", e.Entity, "action", e.Action)
	}
}

// Querier — pgxpool.Pool ve pgx.Tx'in ortak yüzeyi (aynı-tx audit için).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// RecordTx — audit kaydını ÇAĞIRANIN transaction'ında yazar. Repository, iş
// değişikliğiyle audit kaydının atomik olmasını istediğinde bunu kullanır:
// iş yazımı geri alınırsa audit de geri alınır (tutarlılık). Hata YUTULMAZ —
// çağırana döner ki transaction bütünlüğü korunsun.
func (r *Recorder) RecordTx(ctx context.Context, q Querier, e Entry) error {
	before, _ := json.Marshal(e.Before)
	after, _ := json.Marshal(e.After)
	if e.Before == nil {
		before = nil
	}
	if e.After == nil {
		after = nil
	}
	_, err := q.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, entity, entity_id, action, before, after, ip, request_id)
		VALUES (NULLIF($1,'')::uuid, $2, NULLIF($3,'')::uuid, $4, $5, $6, NULLIF($7,''), NULLIF($8,''))`,
		e.ActorID, e.Entity, e.EntityID, string(e.Action), before, after, e.IP, e.ReqID)
	return err
}

type ctxKey int

const ctxKeyMeta ctxKey = iota

type Meta struct {
	ActorID string
	IP      string
	ReqID   string
}

// MetaFrom — audit meta'sını döner. Aktör, isteğin İLERLEYEN aşamasında (auth
// middleware'i route seviyesinde çalıştıktan sonra) context'e yazıldığından
// burada context'ten TAZE okunur; global middleware sırasında henüz boştu.
func MetaFrom(ctx context.Context) Meta {
	m, _ := ctx.Value(ctxKeyMeta).(Meta)
	if actor := httpx.ActorIDFrom(ctx); actor != "" {
		m.ActorID = actor // auth middleware'inin doldurduğu güncel aktör
	}
	return m
}

// Middleware — istekten IP/request-id bilgisini toplayıp context'e koyar.
// Aktör (JWT sub) auth middleware'i tarafından route seviyesinde eklenir ve
// MetaFrom tarafından okuma anında birleştirilir.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := Meta{
			IP:    clientIP(r),
			ReqID: httpx.RequestIDFrom(r.Context()),
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyMeta, m)))
	})
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return v
	}
	return r.RemoteAddr
}
