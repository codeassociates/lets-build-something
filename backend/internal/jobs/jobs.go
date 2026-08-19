// Package jobs is a small durable queue kept in Postgres. Reminder emails and
// overdue sweeps run through it, which means no broker and no Redis: the
// database the system already depends on is the only moving part.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/codeassociates/lets-build-something/backend/internal/db"
)

// Job kinds. Each has a handler registered by the worker.
const (
	KindPickupReminder = "pickup_reminder"
	KindReturnReminder = "return_reminder"
	KindOverdueNotice  = "overdue_notice"
	KindBookingConfirm = "booking_confirmation"
	KindReceipt        = "rental_receipt"
	KindSweep          = "daily_sweep"
)

type Job struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	DedupeKey *string         `json:"dedupe_key"`
	RunAt     time.Time       `json:"run_at"`
	Status    string          `json:"status"`
	Attempts  int             `json:"attempts"`
	LastError string          `json:"last_error"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// MaxAttempts bounds retries before a job is parked as failed for a human.
const MaxAttempts = 5

type Queue struct{ pool *db.DB }

func New(pool *db.DB) *Queue { return &Queue{pool: pool} }

type Enqueue struct {
	Kind    string
	Payload any
	RunAt   time.Time
	// DedupeKey makes scheduling idempotent: enqueueing "the pickup reminder for
	// reservation 42" twice leaves one job, however many times the sweep runs.
	DedupeKey string
}

// Add schedules a job. It takes a Querier so scheduling can be part of the same
// transaction that creates the reservation — either both happen or neither does.
func (q *Queue) Add(ctx context.Context, qr db.Querier, e Enqueue) (int64, error) {
	payload := []byte("{}")
	if e.Payload != nil {
		var err error
		payload, err = json.Marshal(e.Payload)
		if err != nil {
			return 0, fmt.Errorf("encoding job payload: %w", err)
		}
	}
	if e.RunAt.IsZero() {
		e.RunAt = time.Now()
	}
	var dedupe *string
	if e.DedupeKey != "" {
		dedupe = &e.DedupeKey
	}

	var id int64
	err := qr.QueryRow(ctx, `
		INSERT INTO jobs (kind, payload, dedupe_key, run_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (dedupe_key) DO UPDATE SET run_at = EXCLUDED.run_at
		WHERE jobs.status = 'pending'
		RETURNING id`, e.Kind, payload, dedupe, e.RunAt).Scan(&id)
	if err != nil {
		// A conflict on an already-completed job is success, not an error: the
		// work it represents has been done.
		if isNoRows(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("enqueuing %s: %w", e.Kind, err)
	}
	return id, nil
}

// Schedule adds a job outside any transaction, for callers with nothing else
// to commit alongside it.
func (q *Queue) Schedule(ctx context.Context, e Enqueue) (int64, error) {
	return q.Add(ctx, q.pool, e)
}

// Claim takes up to n due jobs and marks them running. SKIP LOCKED means
// several workers can poll the same table without ever handing out the same
// job twice, and without blocking each other.
func (q *Queue) Claim(ctx context.Context, n int) ([]Job, error) {
	rows, err := q.pool.Query(ctx, `
		UPDATE jobs SET status = 'running', locked_at = now(), attempts = attempts + 1,
		                updated_at = now()
		WHERE id IN (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_at <= now()
			ORDER BY run_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, kind, payload, dedupe_key, run_at, status, attempts,
		          last_error, created_at, updated_at`, n)
	if err != nil {
		return nil, fmt.Errorf("claiming jobs: %w", err)
	}
	defer rows.Close()

	out := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Kind, &j.Payload, &j.DedupeKey, &j.RunAt, &j.Status,
			&j.Attempts, &j.LastError, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (q *Queue) Complete(ctx context.Context, id int64) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE jobs SET status='done', last_error='', locked_at=NULL, updated_at=now()
		 WHERE id=$1`, id)
	return err
}

// Fail reschedules with exponential backoff until MaxAttempts, then parks the
// job as failed so it shows up in the admin view instead of retrying forever.
func (q *Queue) Fail(ctx context.Context, j Job, cause error) error {
	if j.Attempts >= MaxAttempts {
		_, err := q.pool.Exec(ctx,
			`UPDATE jobs SET status='failed', last_error=$2, locked_at=NULL, updated_at=now()
			 WHERE id=$1`, j.ID, cause.Error())
		return err
	}
	backoff := time.Duration(1<<uint(j.Attempts)) * time.Minute
	_, err := q.pool.Exec(ctx, `
		UPDATE jobs SET status='pending', last_error=$2, run_at=now() + $3::interval,
		                locked_at=NULL, updated_at=now()
		WHERE id=$1`, j.ID, cause.Error(), backoff.String())
	return err
}

// ReleaseStale returns jobs to the queue whose worker died mid-flight.
func (q *Queue) ReleaseStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := q.pool.Exec(ctx, `
		UPDATE jobs SET status='pending', locked_at=NULL, updated_at=now()
		WHERE status='running' AND locked_at < now() - $1::interval`, olderThan.String())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

type Filter struct {
	Status string
	Kind   string
	Limit  int
}

func (q *Queue) List(ctx context.Context, f Filter) ([]Job, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := q.pool.Query(ctx, `
		SELECT id, kind, payload, dedupe_key, run_at, status, attempts, last_error,
		       created_at, updated_at
		FROM jobs
		WHERE ($1 = '' OR status = $1) AND ($2 = '' OR kind = $2)
		ORDER BY run_at DESC LIMIT $3`, f.Status, f.Kind, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	defer rows.Close()

	out := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Kind, &j.Payload, &j.DedupeKey, &j.RunAt, &j.Status,
			&j.Attempts, &j.LastError, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// CancelFor drops pending jobs tied to a reservation — used when a booking is
// cancelled, so no reminder goes out for a rental that will not happen.
func (q *Queue) CancelFor(ctx context.Context, qr db.Querier, reservationID int64) error {
	_, err := qr.Exec(ctx, `
		DELETE FROM jobs
		WHERE status = 'pending' AND payload->>'reservation_id' = $1::text`,
		fmt.Sprint(reservationID))
	return err
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
