// internal/adapters/goodreads_bridge/goodreads.go
//
// Goodreads Bridge adapter stub. Real implementation in Phase 2.

package goodreads_bridge

import (
	"context"
	"fmt"

	"github.com/readsync/readsync/internal/model"
)

// Adapter is the Goodreads bridge adapter stub.
type Adapter struct{}

// New creates a Goodreads bridge adapter stub.
func New() *Adapter { return &Adapter{} }

func (a *Adapter) Source() model.Source             { return model.SourceGoodreadsBridge }
func (a *Adapter) Health() model.AdapterHealthState { return model.HealthDisabled }
func (a *Adapter) Start(_ context.Context) error    { return nil }
func (a *Adapter) Stop() error                      { return nil }
func (a *Adapter) WriteProgress(_ context.Context, _ *model.OutboxJob) error {
	return fmt.Errorf("goodreads_bridge adapter: not implemented (Phase 2)")
}
