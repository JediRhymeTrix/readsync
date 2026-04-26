// internal/adapters/koreader/translate.go
//
// Translates between KOReader wire format and the ReadSync model.

package koreader

import (
	"strings"

	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/model"
	"github.com/readsync/readsync/internal/resolver"
)

// pushRequest is the JSON body for PUT /syncs/progress.
type pushRequest struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceID   string  `json:"device_id"`
}

// pullResponse is the JSON body for GET /syncs/progress/:document.
type pullResponse struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceID   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

// pushResponse is the JSON body returned after a successful PUT.
type pushResponse struct {
	Document  string `json:"document"`
	Timestamp int64  `json:"timestamp"`
}

// stalePushResponse is returned with HTTP 412 when the server has newer data.
type stalePushResponse struct {
	Message   string `json:"message"`
	Document  string `json:"document"`
	Timestamp int64  `json:"timestamp"`
}

// locatorType classifies the KOReader progress string.
// Returns LocationKOReaderXPtr for epubcfi, LocationPercent otherwise.
func locatorType(progress string) model.LocationType {
	if strings.HasPrefix(progress, "epubcfi(") {
		return model.LocationKOReaderXPtr
	}
	return model.LocationPercent
}

// toAdapterEvent converts a KOReader push payload to a pipeline AdapterEvent.
func toAdapterEvent(req pushRequest) core.AdapterEvent {
	pct := req.Percentage
	rawLoc := req.Progress
	lt := locatorType(req.Progress)

	var rs model.ReadStatus
	switch {
	case pct >= 1.0:
		rs = model.StatusFinished
	case pct > 0:
		rs = model.StatusReading
	default:
		rs = model.StatusUnknown
	}

	return core.AdapterEvent{
		BookEvidence: resolver.Evidence{
			KOReaderDocHash: req.Document,
		},
		Source:          model.SourceKOReader,
		PercentComplete: &pct,
		RawLocator:      &rawLoc,
		LocatorType:     lt,
		ReadStatus:      rs,
	}
}

// canonicalToPull converts a canonical_progress row back to a KOReader pull
// response.  deviceName and deviceID are from the last push for this book.
func canonicalToPull(
	document string,
	pct *float64,
	rawLocator *string,
	deviceName, deviceID string,
	updatedAtUnix int64,
) pullResponse {
	progress := ""
	if rawLocator != nil {
		progress = *rawLocator
	}
	percentage := 0.0
	if pct != nil {
		percentage = *pct
	}
	return pullResponse{
		Document:   document,
		Progress:   progress,
		Percentage: percentage,
		Device:     deviceName,
		DeviceID:   deviceID,
		Timestamp:  updatedAtUnix,
	}
}
