// Package outbox implements ReadSync's reliable write-back queue.
//
// When the pipeline confirms a canonical progress update, it enqueues
// outbox jobs for each adapter target via Enqueue. The FairScheduler
// runs one Worker per adapter target; each worker polls every 2 seconds
// and retries failed jobs with exponential backoff.
//
// # Retry schedule
//
// NextRetry(n) = BaseDelay(5s) × 2^(n-1) ± 20% jitter, capped at MaxDelay(2h).
// After MaxAttempts(10) the job moves to status "deadletter".
//
// # State machine
//
//	queued → running → succeeded
//	queued → running → retrying → running → …
//	retrying → running → deadletter (after 10 attempts)
//
// # CGO note
//
// The SQLStore implementation requires go-sqlite3 (CGO). The Worker,
// FairScheduler, and NextRetry are pure Go and can be tested without CGO
// by providing a mock Store.
package outbox
