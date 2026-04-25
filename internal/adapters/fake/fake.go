// internal/adapters/fake/fake.go
//
// Fake adapter that emits scripted progress events for end-to-end pipeline
// tests. This is the only "real" adapter in Phase 1.

package fake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// ScriptedEvent defines one event in the fake adapter's emission sequence.
type ScriptedEvent struct {
	// Delay before emitting this event (relative to adapter start).
	Delay time.Duration

	// BookEvidence identifies the book.
	BookEvidence resolver.Evidence

	// Progress fields.
	PercentComplete *float64
	PageNumber      *int32
	TotalPages      *int32
	RawLocator      *string
	LocatorType     model.LocationType
	ReadStatus      model.ReadStatus
	DeviceTS        *time.Time
}

// Fake is a scripted adapter that replays a fixed list of events.
type Fake struct {
	source   model.Source
	script   []ScriptedEvent
	pipeline *core.Pipeline
	health   model.AdapterHealthState

	mu      sync.Mutex
	emitted []ScriptedEvent
	errors  []error
}

// New creates a Fake adapter with the given script.
func New(source model.Source, script []ScriptedEvent) *Fake {
	return &Fake{
		source: source,
		script: script,
		health: model.HealthOK,
	}
}

func (f *Fake) Source() model.Source         { return f.source }
func (f *Fake) Health() model.AdapterHealthState { return f.health }
func (f *Fake) SetPipeline(p *core.Pipeline) { f.pipeline = p }

// Start begins replaying the script in a goroutine.
func (f *Fake) Start(ctx context.Context) error {
	if f.pipeline == nil {
		return fmt.Errorf("fake adapter %s: pipeline not set", f.source)
	}
	go f.replay(ctx)
	return nil
}

// Stop is a no-op for the fake adapter (ctx cancellation stops it).
func (f *Fake) Stop() error { return nil }

func (f *Fake) replay(ctx context.Context) {
	for _, se := range f.script {
		select {
		case <-ctx.Done():
			return
		case <-time.After(se.Delay):
		}

		ev := core.AdapterEvent{
			BookEvidence:    se.BookEvidence,
			Source:          f.source,
			DeviceTS:        se.DeviceTS,
			PercentComplete: se.PercentComplete,
			PageNumber:      se.PageNumber,
			TotalPages:      se.TotalPages,
			RawLocator:      se.RawLocator,
			LocatorType:     se.LocatorType,
			ReadStatus:      se.ReadStatus,
		}

		err := f.pipeline.Submit(ctx, ev)
		f.mu.Lock()
		f.emitted = append(f.emitted, se)
		if err != nil {
			f.errors = append(f.errors, err)
		}
		f.mu.Unlock()
	}
}

// Emitted returns all events that have been submitted to the pipeline.
func (f *Fake) Emitted() []ScriptedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ScriptedEvent, len(f.emitted))
	copy(out, f.emitted)
	return out
}

// Errors returns any pipeline submission errors.
func (f *Fake) Errors() []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]error, len(f.errors))
	copy(out, f.errors)
	return out
}

// WriteProgress is a no-op for the fake adapter.
func (f *Fake) WriteProgress(ctx context.Context, job *model.OutboxJob) error {
	return nil
}
