// internal/outbox/statemachine_test.go
//
// Tests the full outbox state machine transitions:
//   queued → running → succeeded
//   queued → running → retrying (attempt < max)
//   queued → running → deadletter (attempt = max)
//
// Also verifies that backoff delays grow correctly and that jitter stays
// within the documented ±20% range.

package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/readsync/readsync/internal/model"
)

// TestStateMachine_FullLifecycle walks through all valid state transitions.
func TestStateMachine_FullLifecycle(t *testing.T) {
	t.Run("queued→running→succeeded", func(t *testing.T) {
		store := &mockStore{jobs: []*model.OutboxJob{
			{ID: 10, BookID: 1, TargetSource: model.SourceCalibre,
				Status: model.OutboxQueued, Attempts: 0, Payload: "{}"},
		}}
		exec := &mockExecutor{}
		w := NewWorker(model.SourceCalibre, store, exec)
		w.processOne(context.Background())

		store.mu.Lock()
		defer store.mu.Unlock()
		if len(store.succeeded) != 1 || store.succeeded[0] != 10 {
			t.Errorf("expected job 10 succeeded, got %v", store.succeeded)
		}
		if len(store.failed) != 0 {
			t.Errorf("unexpected failures: %v", store.failed)
		}
	})

	t.Run("queued→running→retrying (attempt 1)", func(t *testing.T) {
		store := &mockStore{jobs: []*model.OutboxJob{
			{ID: 11, BookID: 2, TargetSource: model.SourceMoon,
				Status: model.OutboxQueued, Attempts: 0, Payload: "{}"},
		}}
		exec := &mockExecutor{errFn: func(_ int) error { return errors.New("timeout") }}
		w := NewWorker(model.SourceMoon, store, exec)
		w.processOne(context.Background())

		store.mu.Lock()
		defer store.mu.Unlock()
		if len(store.failed) != 1 {
			t.Fatalf("expected 1 failed record, got %d", len(store.failed))
		}
		fr := store.failed[0]
		if fr.id != 11 {
			t.Errorf("wrong job id: %d", fr.id)
		}
		if fr.dead {
			t.Error("attempt 1 must NOT be dead-lettered")
		}
		if fr.errMsg != "timeout" {
			t.Errorf("wrong error: %s", fr.errMsg)
		}
		// Retry time must be in the future.
		if !fr.nextRetry.After(time.Now()) {
			t.Errorf("next retry should be in the future, got %v", fr.nextRetry)
		}
	})

	t.Run("retrying→deadletter at MaxAttempts", func(t *testing.T) {
		store := &mockStore{jobs: []*model.OutboxJob{
			{ID: 12, BookID: 3, TargetSource: model.SourceKOReader,
				Status: model.OutboxRetrying, Attempts: MaxAttempts - 1, Payload: "{}"},
		}}
		exec := &mockExecutor{errFn: func(_ int) error { return errors.New("still failing") }}
		w := NewWorker(model.SourceKOReader, store, exec)
		w.processOne(context.Background())

		store.mu.Lock()
		defer store.mu.Unlock()
		if len(store.failed) != 1 || !store.failed[0].dead {
			t.Errorf("expected dead=true at MaxAttempts, got %v", store.failed)
		}
	})

	t.Run("no_jobs_is_noop", func(t *testing.T) {
		store := &mockStore{jobs: []*model.OutboxJob{}}
		exec := &mockExecutor{}
		w := NewWorker(model.SourceCalibre, store, exec)
		w.processOne(context.Background())
		if exec.calls != 0 {
			t.Errorf("expected 0 calls with empty queue, got %d", exec.calls)
		}
	})
}

// TestStateMachine_BackoffGrowth verifies exponential growth with bounded jitter.
func TestStateMachine_BackoffGrowth(t *testing.T) {
	prev := time.Duration(0)
	for i := 1; i <= 8; i++ {
		next := NextRetry(i)
		delay := time.Until(next)
		if delay <= 0 {
			t.Errorf("attempt %d: delay must be positive, got %v", i, delay)
		}
		if i > 1 && delay < prev/2 {
			t.Errorf("attempt %d delay %v is less than half of attempt %d delay %v (jitter too wide?)",
				i, delay, i-1, prev)
		}
		prev = delay
	}
}

// TestStateMachine_MaxDelayCapped verifies the cap at MaxDelay ± jitter.
func TestStateMachine_MaxDelayCapped(t *testing.T) {
	// At very high attempt counts the delay should not exceed MaxDelay + 20%.
	for _, attempt := range []int{20, 50, 100} {
		next := NextRetry(attempt)
		delay := time.Until(next)
		ceiling := MaxDelay + MaxDelay/5 // MaxDelay + 20% jitter
		if delay > ceiling {
			t.Errorf("attempt %d: delay %v exceeds ceiling %v", attempt, delay, ceiling)
		}
	}
}

// TestStateMachine_WrongTargetIgnored verifies that a job for a different
// target is not claimed by this worker.
func TestStateMachine_WrongTargetIgnored(t *testing.T) {
	store := &mockStore{jobs: []*model.OutboxJob{
		{ID: 20, BookID: 5, TargetSource: model.SourceMoon,
			Status: model.OutboxQueued, Attempts: 0, Payload: "{}"},
	}}
	exec := &mockExecutor{}
	w := NewWorker(model.SourceCalibre, store, exec) // Calibre worker, but job is for Moon
	w.processOne(context.Background())

	if exec.calls != 0 {
		t.Errorf("calibre worker must not claim moon job; calls=%d", exec.calls)
	}
}
