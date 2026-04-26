// internal/adapters/koreader/codec/codec.go
//
// Pure-Go KOReader wire-format codec — no CGO, no internal/core, no internal/db.
// Import chain: internal/model, internal/resolver, strings — stdlib only.
//
// The parent koreader package (which needs CGO via core) calls these functions
// and wraps the results into pipeline types.
// Tests in this package compile and run without any C toolchain.

package codec

import (
	"strings"

	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// ─── Wire types ──────────────────────────────────────────────────────────────

// PushRequest is the JSON body for PUT /syncs/progress.
type PushRequest struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceID   string  `json:"device_id"`
}

// PullResponse is the JSON body for GET /syncs/progress/:document.
type PullResponse struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceID   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

// ─── Event ───────────────────────────────────────────────────────────────────

// Event is the decoded result of a KOReader push payload.
// The parent koreader package wraps this into a core.AdapterEvent.
type Event struct {
	BookEvidence    resolver.Evidence
	PercentComplete *float64
	RawLocator      *string
	LocatorType     model.LocationType
	ReadStatus      model.ReadStatus
}

// ─── Pure codec functions ─────────────────────────────────────────────────────

// LocatorType classifies the KOReader progress string.
// Returns LocationKOReaderXPtr for epubcfi values, LocationPercent otherwise.
func LocatorType(progress string) model.LocationType {
	if strings.HasPrefix(progress, "epubcfi(") {
		return model.LocationKOReaderXPtr
	}
	return model.LocationPercent
}

// ToEvent converts a KOReader push payload to a codec Event.
func ToEvent(req PushRequest) Event {
	pct := req.Percentage
	rawLoc := req.Progress
	lt := LocatorType(req.Progress)

	var rs model.ReadStatus
	switch {
	case pct >= 1.0:
		rs = model.StatusFinished
	case pct > 0:
		rs = model.StatusReading
	default:
		rs = model.StatusUnknown
	}

	return Event{
		BookEvidence:    resolver.Evidence{KOReaderDocHash: req.Document},
		PercentComplete: &pct,
		RawLocator:      &rawLoc,
		LocatorType:     lt,
		ReadStatus:      rs,
	}
}

// CanonicalToPull builds a KOReader pull response from canonical progress data.
func CanonicalToPull(
	document string,
	pct *float64,
	rawLocator *string,
	deviceName, deviceID string,
	updatedAtUnix int64,
) PullResponse {
	progress := ""
	if rawLocator != nil {
		progress = *rawLocator
	}
	percentage := 0.0
	if pct != nil {
		percentage = *pct
	}
	return PullResponse{
		Document:   document,
		Progress:   progress,
		Percentage: percentage,
		Device:     deviceName,
		DeviceID:   deviceID,
		Timestamp:  updatedAtUnix,
	}
}
