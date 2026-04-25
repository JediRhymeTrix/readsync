// internal/adapters/adapter.go
//
// Adapter interface definition. All adapter implementations must satisfy this
// interface. Phase 1 provides stubs/fakes only; real implementations are in
// phases 2-3.

package adapters

import (
	"context"

	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/model"
)

// Adapter is the interface all sync adapters must implement.
type Adapter interface {
	// Source returns the unique identifier for this adapter.
	Source() model.Source

	// Start begins the adapter's background activity (e.g. HTTP server,
	// filesystem watcher). It must be non-blocking; long work goes in goroutines.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the adapter.
	Stop() error

	// Health returns the current health state.
	Health() model.AdapterHealthState
}

// EventEmitter is an Adapter that can also push events into the pipeline.
// Adapters that receive events (KOReader, Moon+, Calibre watcher) implement
// this interface to register a callback.
type EventEmitter interface {
	Adapter
	// SetPipeline provides the adapter with the pipeline to push events to.
	SetPipeline(p *core.Pipeline)
}

// WriteTarget is an Adapter that can receive canonical progress updates
// and write them back to the underlying system (e.g. calibredb, Goodreads).
type WriteTarget interface {
	Adapter
	// WriteProgress writes the canonical progress for the given book.
	WriteProgress(ctx context.Context, job *model.OutboxJob) error
}
