// internal/adapters/calibre/calibre.go
//
// Calibre adapter stub. Real implementation in Phase 2.

package calibre

import (
	"context"
	"fmt"

	"github.com/readsync/readsync/internal/model"
)

// Adapter is the Calibre adapter stub.
type Adapter struct{}

// New creates a Calibre adapter stub.
func New() *Adapter { return &Adapter{} }

func (a *Adapter) Source() model.Source             { return model.SourceCalibre }
func (a *Adapter) Health() model.AdapterHealthState { return model.HealthDisabled }
func (a *Adapter) Start(_ context.Context) error    { return nil }
func (a *Adapter) Stop() error                      { return nil }
func (a *Adapter) WriteProgress(_ context.Context, job *model.OutboxJob) error {
	return fmt.Errorf("calibre adapter: not implemented (Phase 2)")
}
