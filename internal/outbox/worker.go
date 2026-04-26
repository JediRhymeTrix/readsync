// internal/outbox/worker.go
//
// Outbox worker: exponential backoff, per-target serial execution.

package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/readsync/readsync/internal/model"
)

const (
	MaxAttempts    = 10
	BaseDelay      = 5 * time.Second
	MaxDelay       = 2 * time.Hour
	JitterFraction = 0.2
	TickInterval   = 2 * time.Second
)

// Executor is the interface adapters must implement.
type Executor interface {
	Execute(ctx context.Context, job *model.OutboxJob) error
}

// Store is the persistence interface the worker uses.
type Store interface {
	ClaimJob(ctx context.Context, target model.Source) (*model.OutboxJob, error)
	MarkSucceeded(ctx context.Context, jobID int64) error
	MarkFailed(ctx context.Context, jobID int64, errMsg string, nextRetry time.Time, dead bool) error
}

// Worker manages outbox processing for one adapter target.
type Worker struct {
	target   model.Source
	store    Store
	executor Executor
}

// NewWorker creates a new outbox worker.
func NewWorker(target model.Source, store Store, executor Executor) *Worker {
	return &Worker{target: target, store: store, executor: executor}
}

// Run processes jobs until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processOne(ctx)
		}
	}
}

func (w *Worker) processOne(ctx context.Context) {
	job, err := w.store.ClaimJob(ctx, w.target)
	if err != nil || job == nil {
		return
	}
	execErr := w.executor.Execute(ctx, job)
	if execErr == nil {
		_ = w.store.MarkSucceeded(ctx, job.ID)
		return
	}
	attempts := job.Attempts + 1
	dead := attempts >= MaxAttempts
	next := NextRetry(attempts)
	_ = w.store.MarkFailed(ctx, job.ID, execErr.Error(), next, dead)
}

// NextRetry computes the next retry time for the given attempt number.
func NextRetry(attempt int) time.Time {
	var delay time.Duration
	if attempt <= 1 {
		delay = BaseDelay
	} else {
		maxMultiplier := float64(MaxDelay) / float64(BaseDelay)
		exp := math.Pow(2, float64(attempt-1))
		if exp >= maxMultiplier {
			delay = MaxDelay
		} else {
			delay = time.Duration(float64(BaseDelay) * exp)
		}
	}
	if delay > MaxDelay {
		delay = MaxDelay
	}
	jitter := time.Duration(float64(delay) * JitterFraction * (rand.Float64()*2 - 1))
	return time.Now().Add(delay + jitter)
}

// FairScheduler runs multiple per-target workers concurrently.
type FairScheduler struct {
	workers []*Worker
}

// NewFairScheduler creates a scheduler for the given workers.
func NewFairScheduler(workers []*Worker) *FairScheduler {
	return &FairScheduler{workers: workers}
}

// Run starts all workers concurrently and waits for ctx done.
func (fs *FairScheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, w := range fs.workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			w.Run(ctx)
		}(w)
	}
	wg.Wait()
}

// Enqueue inserts a new outbox job.
func Enqueue(ctx context.Context, db *sql.DB, bookID int64, target model.Source, payload string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx, `
		INSERT INTO sync_outbox(book_id, target_source, status, attempts, payload, created_at, updated_at)
		VALUES (?, ?, 'queued', 0, ?, ?, ?)
	`, bookID, string(target), payload, now, now)
	if err != nil {
		return fmt.Errorf("outbox.Enqueue: %w", err)
	}
	return nil
}

// SQLStore is a production Store implementation backed by SQLite.
type SQLStore struct{ db *sql.DB }

// NewSQLStore wraps a *sql.DB as an outbox Store.
func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }

func (s *SQLStore) ClaimJob(ctx context.Context, target model.Source) (*model.OutboxJob, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := tx.QueryRowContext(ctx, `
		SELECT id, book_id, target_source, status, attempts, payload
		FROM sync_outbox
		WHERE target_source = ?
		  AND status IN ('queued','retrying')
		  AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY id ASC LIMIT 1
	`, string(target), now)

	var job model.OutboxJob
	var targetStr, statusStr string
	err = row.Scan(&job.ID, &job.BookID, &targetStr, &statusStr, &job.Attempts, &job.Payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.TargetSource = model.Source(targetStr)
	job.Status = model.OutboxStatus(statusStr)

	if _, err = tx.ExecContext(ctx, `
		UPDATE sync_outbox SET status='running', updated_at=? WHERE id=?
	`, now, job.ID); err != nil {
		return nil, err
	}
	return &job, tx.Commit()
}

func (s *SQLStore) MarkSucceeded(ctx context.Context, jobID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sync_outbox SET status='succeeded', updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339Nano), jobID)
	return err
}

func (s *SQLStore) MarkFailed(ctx context.Context, jobID int64, errMsg string, nextRetry time.Time, dead bool) error {
	status := string(model.OutboxRetrying)
	if dead {
		status = string(model.OutboxDeadLetter)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_outbox
		SET status=?, attempts=attempts+1, last_error=?, next_retry_at=?, updated_at=?
		WHERE id=?
	`, status, errMsg, nextRetry.UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano), jobID)
	return err
}
