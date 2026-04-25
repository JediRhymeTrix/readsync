// internal/core/pipeline.go
//
// Event pipeline: adapter event → validate → normalize → identity resolve →
// append progress_events → evaluate canonical update → detect conflict →
// update canonical_progress → enqueue outbox jobs.
//
// A single writer goroutine serialises all database writes (spec §4).

package core

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/readsync/readsync/internal/conflicts"
	"github.com/readsync/readsync/internal/logging"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// AdapterEvent is the raw input submitted by an adapter.
type AdapterEvent struct {
	BookEvidence    resolver.Evidence
	Source          model.Source
	DeviceTS        *time.Time
	PercentComplete *float64
	PageNumber      *int32
	TotalPages      *int32
	RawLocator      *string
	LocatorType     model.LocationType
	ReadStatus      model.ReadStatus
}

type adapterWork struct {
	ev     AdapterEvent
	result chan<- error
}

// Pipeline processes adapter events and persists the results.
type Pipeline struct {
	db     *sql.DB
	log    *logging.Logger
	ingest chan adapterWork
}

// NewPipeline creates a pipeline. Call Run to start the writer goroutine.
func NewPipeline(db *sql.DB, log *logging.Logger) *Pipeline {
	return &Pipeline{
		db:     db,
		log:    log,
		ingest: make(chan adapterWork, 256),
	}
}

// Submit enqueues an adapter event for processing.
func (p *Pipeline) Submit(ctx context.Context, ev AdapterEvent) error {
	result := make(chan error, 1)
	select {
	case p.ingest <- adapterWork{ev: ev, result: result}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run starts the single writer goroutine.
func (p *Pipeline) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case work := <-p.ingest:
			work.result <- p.process(ctx, work.ev)
		}
	}
}

// process is the core pipeline logic.
func (p *Pipeline) process(ctx context.Context, ev AdapterEvent) error {
	if err := validateEvent(ev); err != nil {
		return fmt.Errorf("pipeline.validate: %w", err)
	}
	ev = normalizeEvent(ev)

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("pipeline.begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	bookID, confidence, err := resolveBook(ctx, tx, ev)
	if err != nil {
		return fmt.Errorf("pipeline.resolve: %w", err)
	}

	eventID, err := insertEvent(ctx, tx, bookID, confidence, ev)
	if err != nil {
		return fmt.Errorf("pipeline.insert_event: %w", err)
	}

	canon, err := loadCanonical(ctx, tx, bookID)
	if err != nil {
		return fmt.Errorf("pipeline.load_canonical: %w", err)
	}

	newEv := buildProgressEvent(bookID, eventID, confidence, ev)

	var conflictID *int64
	if canon != nil && !canon.UserPinned {
		jump := conflicts.DetectSuspiciousJump(canon, newEv)
		if jump.Suspicious {
			cid, cerr := insertConflict(ctx, tx, bookID, canon.EventID, eventID, jump.Reason)
			if cerr != nil {
				return fmt.Errorf("pipeline.insert_conflict: %w", cerr)
			}
			conflictID = &cid
			p.log.Warn("conflict detected",
				logging.F("book_id", bookID),
				logging.F("reason", jump.Reason),
				logging.F("conflict_id", cid),
			)
		}
	}

	updateCanon := true
	if conflictID != nil {
		autoParams := conflicts.AutoResolveParams{
			TrustworthyTimestamps: ev.DeviceTS != nil,
			ConfidenceHigh:        confidence >= 80,
			PlausibleMovement:     false,
			WritebackEnabled:      resolver.WritebackEnabled(confidence),
			NoUserPin:             canon == nil || !canon.UserPinned,
		}
		if !conflicts.CanAutoResolve(autoParams) {
			updateCanon = false
		}
	}

	if updateCanon && (canon == nil || !canon.UserPinned) {
		if err := upsertCanonical(ctx, tx, bookID, eventID, ev); err != nil {
			return fmt.Errorf("pipeline.upsert_canonical: %w", err)
		}
	}

	if resolver.WritebackEnabled(confidence) && conflictID == nil {
		if err := enqueueWritebacks(ctx, tx, bookID, eventID, ev.Source); err != nil {
			return fmt.Errorf("pipeline.enqueue: %w", err)
		}
	} else if conflictID != nil {
		if err := enqueueBlockedJobs(ctx, tx, bookID, eventID, ev.Source, *conflictID); err != nil {
			return fmt.Errorf("pipeline.enqueue_blocked: %w", err)
		}
	}

	return tx.Commit()
}
