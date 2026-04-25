// internal/adapters/webdav/webdav.go
//
// WebDAV adapter stub. The WebDAV server is embedded in the moon adapter;
// this package provides the server component in isolation for reuse. Phase 3.

package webdav

import (
	"context"
	"fmt"

	"github.com/readsync/readsync/internal/model"
)

// Adapter is the WebDAV adapter stub.
type Adapter struct{}

// New creates a WebDAV adapter stub.
func New() *Adapter { return &Adapter{} }

func (a *Adapter) Source() model.Source             { return model.SourceMoon }
func (a *Adapter) Health() model.AdapterHealthState { return model.HealthDisabled }
func (a *Adapter) Start(_ context.Context) error    { return nil }
func (a *Adapter) Stop() error                      { return nil }
func (a *Adapter) WriteProgress(_ context.Context, _ *model.OutboxJob) error {
	return fmt.Errorf("webdav adapter: not implemented (Phase 3)")
}
