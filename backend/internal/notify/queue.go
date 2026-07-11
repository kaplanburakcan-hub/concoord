package notify

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgreSQL tabanlı iş kuyruğu (Plan §2: Redis bağımlılığı yok).
// Dequeue FOR UPDATE SKIP LOCKED kullanır: birden çok worker güvenle çalışır.

type Job struct {
	ID       int64
	Kind     string
	Payload  json.RawMessage
	Attempts int
	MaxAtt   int
}

func Enqueue(ctx context.Context, pool *pgxpool.Pool, kind string, payload json.RawMessage) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO job_queue (kind, payload) VALUES ($1, $2)`, kind, payload)
	return err
}

// Dequeue — hazır tek işi atomik olarak 'running'e çeker. İş yoksa (nil, nil).
func Dequeue(ctx context.Context, pool *pgxpool.Pool) (*Job, error) {
	var j Job
	err := pool.QueryRow(ctx, `
		UPDATE job_queue SET status='running', attempts=attempts+1, updated_at=now()
		WHERE id = (
			SELECT id FROM job_queue
			WHERE status='pending' AND run_at <= now()
			ORDER BY run_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, kind, payload, attempts, max_attempts`).
		Scan(&j.ID, &j.Kind, &j.Payload, &j.Attempts, &j.MaxAtt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func Complete(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	_, err := pool.Exec(ctx,
		`UPDATE job_queue SET status='done', updated_at=now() WHERE id=$1`, id)
	return err
}

// Fail — deneme hakkı kaldıysa üstel geri çekilmeyle yeniden kuyruğa koyar
// (30s, 60s, 120s, ...); kalmadıysa 'failed' olarak işaretler.
func Fail(ctx context.Context, pool *pgxpool.Pool, j *Job, jobErr error) error {
	if j.Attempts >= j.MaxAtt {
		_, err := pool.Exec(ctx, `
			UPDATE job_queue SET status='failed', last_error=$2, updated_at=now()
			WHERE id=$1`, j.ID, jobErr.Error())
		return err
	}
	delaySec := int(Backoff(j.Attempts).Seconds())
	_, err := pool.Exec(ctx, `
		UPDATE job_queue SET status='pending', last_error=$2,
		       run_at=now() + ($3 * interval '1 second'), updated_at=now()
		WHERE id=$1`, j.ID, jobErr.Error(), delaySec)
	return err
}

// Backoff — deneme sayısına göre bekleme süresi. Ayrı fonksiyon: birim testli.
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	sec := 30 * math.Pow(2, float64(attempt-1))
	if sec > 3600 {
		sec = 3600
	}
	return time.Duration(sec) * time.Second
}
