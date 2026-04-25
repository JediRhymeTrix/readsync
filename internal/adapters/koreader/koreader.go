// internal/adapters/koreader/koreader.go
//
// KOReader adapter stub. Real implementation (KOSync HTTP server) in Phase 2.

package koreader

import (
	"context"
	"fmt"

	"github.com/readsync/readsync/internal/model"
)

// Adapter is the KOReader adapter stub.
type Adapter struct{}

// New creates a KOReader adapter stub.
func New() *Adapter { return &Adapter{} }

func (a *Adapter) Source() model.Source             { return model.SourceKOReader }
func (a *Adapter) Health() model.AdapterHealthState { return model.HealthDisabled }
func (a *Adapter) Start(_ context.Context) error    { return nil }
func (a *Adapter) Stop() error                      { return nil }
func (a *Adapter) WriteProgress(_ context.Context, _ *model.OutboxJob) error {
	return fmt.Errorf("koreader adapter: not implemented (Phase 2)")
}
