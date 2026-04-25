// internal/adapters/moon/moon.go
//
// Moon+ Reader Pro adapter stub. Real implementation (WebDAV server) in Phase 3.

package moon

import (
	"context"
	"fmt"

	"github.com/readsync/readsync/internal/model"
)

// Adapter is the Moon+ adapter stub.
type Adapter struct{}

// New creates a Moon+ adapter stub.
func New() *Adapter { return &Adapter{} }

func (a *Adapter) Source() model.Source             { return model.SourceMoon }
func (a *Adapter) Health() model.AdapterHealthState { return model.HealthDisabled }
func (a *Adapter) Start(_ context.Context) error    { return nil }
func (a *Adapter) Stop() error                      { return nil }
func (a *Adapter) WriteProgress(_ context.Context, _ *model.OutboxJob) error {
	return fmt.Errorf("moon adapter: not implemented (Phase 3)")
}
