// internal/outbox/worker_test.go
//
// Unit tests for outbox worker: NextRetry backoff, mock store, state transitions.

package outbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/model"
)

// mockStore is a test double for the Store interface.
type mockStore struct {
	mu      sync.Mutex
	jobs    []*model.OutboxJob
	claimed []*model.OutboxJob
	succeeded []int64
	failed    []failRecord
}

type failRecord struct {
	id        int64
	errMsg    string
	nextRetry time.Time
	dead      bool
}

func (m *mockStore) ClaimJob(_ context.Context, target model.Source) (*model.OutboxJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.TargetSource == target &&
			(j.Status == model.OutboxQueued || j.Status == model.OutboxRetrying) {
			j.Status = model.OutboxRunning
			m.claimed = append(m.claimed, j)
			return j, nil
		}
	}
	return nil, nil
}

func (m *mockStore) MarkSucceeded(_ context.Context, jobID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.succeeded = append(m.succeeded, jobID)
	for _, j := range m.jobs {
		if j.ID == jobID {
			j.Status = model.OutboxSucceeded
		}
	}
	return nil
}

func (m *mockStore) MarkFailed(_ context.Context, jobID int64, errMsg string, nextRetry time.Time, dead bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed = append(m.failed, failRecord{jobID, errMsg, nextRetry, dead})
	return nil
}

// mockExecutor is a test double for the Executor interface.
type mockExecutor struct {
	calls  int
	mu     sync.Mutex
	errFn  func(attempt int) error
}

func (e *mockExecutor) Execute(_ context.Context, job *model.OutboxJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	if e.errFn != nil {
		return e.errFn(e.calls)
	}
	return nil
}

func TestNextRetry_BackoffSequence(t *testing.T) {
	delays := make([]time.Duration, 6)
	for i := 1; i <= 6; i++ {
		next := NextRetry(i)
		delays[i-1] = time.Until(next)
	}

	// Each delay should be >= previous (allowing for jitter).
	for i := 1; i < len(delays); i++ {
		// With jitter there's some variance, but delay[i] > delay[i-1] * 0.5 holds.
		if delays[i] < delays[i-1]/2 {
			t.Errorf("retry %d delay %v is less than half of retry %d delay %v",
				i+1, delays[i], i, delays[i-1])
		}
	}
	t.Logf("backoff delays: %v", delays)
}

func TestNextRetry_MaxDelay(t *testing.T) {
	// At attempt 20, delay should be capped at MaxDelay.
	next := NextRetry(20)
	d := time.Until(next)
	// Allow for jitter: MaxDelay ± 20%.
	max := MaxDelay + MaxDelay/5
	if d > max {
		t.Errorf("delay %v exceeds MaxDelay+jitter %v", d, max)
	}
}

func TestWorker_SuccessfulJob(t *testing.T) {
	store := &mockStore{
		jobs: []*model.OutboxJob{
			{ID: 1, BookID: 10, TargetSource: model.SourceCalibre,
				Status: model.OutboxQueued, Attempts: 0, Payload: "{}"},
		},
	}
	exec := &mockExecutor{} // always succeeds
	worker := NewWorker(model.SourceCalibre, store, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	worker.processOne(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.succeeded) != 1 || store.succeeded[0] != 1 {
		t.Errorf("expected job 1 succeeded, got %v", store.succeeded)
	}
}

func TestWorker_FailedJob_SchedulesRetry(t *testing.T) {
	store := &mockStore{
		jobs: []*model.OutboxJob{
			{ID: 2, BookID: 11, TargetSource: model.SourceMoon,
				Status: model.OutboxQueued, Attempts: 0, Payload: "{}"},
		},
	}
	exec := &mockExecutor{
		errFn: func(_ int) error { return errors.New("connection refused") },
	}
	worker := NewWorker(model.SourceMoon, store, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	worker.processOne(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.failed) != 1 {
		t.Fatalf("expected 1 failed record, got %d", len(store.failed))
	}
	fr := store.failed[0]
	if fr.id != 2 {
		t.Errorf("wrong job id failed: %d", fr.id)
	}
	if fr.dead {
		t.Error("should not be dead on first failure")
	}
	if fr.errMsg != "connection refused" {
		t.Errorf("wrong error: %s", fr.errMsg)
	}
}

func TestWorker_ExceedsMaxAttempts_GoesToDeadLetter(t *testing.T) {
	store := &mockStore{
		jobs: []*model.OutboxJob{
			{ID: 3, BookID: 12, TargetSource: model.SourceKOReader,
				Status: model.OutboxRetrying, Attempts: MaxAttempts - 1, Payload: "{}"},
		},
	}
	exec := &mockExecutor{
		errFn: func(_ int) error { return errors.New("still failing") },
	}
	worker := NewWorker(model.SourceKOReader, store, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	worker.processOne(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.failed) != 1 || !store.failed[0].dead {
		t.Error("expected dead=true at MaxAttempts exceeded")
	}
}

func TestWorker_NoJobs_DoesNothing(t *testing.T) {
	store := &mockStore{jobs: []*model.OutboxJob{}}
	exec := &mockExecutor{}
	worker := NewWorker(model.SourceCalibre, store, exec)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	worker.processOne(ctx)

	if exec.calls != 0 {
		t.Errorf("expected 0 executor calls, got %d", exec.calls)
	}
}
