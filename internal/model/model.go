// internal/model/model.go
//
// Domain model types for ReadSync.

package model

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// Source identifies which adapter produced a progress event.
type Source string

const (
	SourceCalibre            Source = "calibre"
	SourceGoodreadsBridge    Source = "goodreads_bridge"
	SourceKOReader           Source = "koreader"
	SourceMoon               Source = "moon"
	SourceKindleViaGoodreads Source = "kindle_via_goodreads"
)

// ReadStatus is the reading state of a book.
type ReadStatus string

const (
	StatusNotStarted ReadStatus = "not_started"
	StatusReading    ReadStatus = "reading"
	StatusFinished   ReadStatus = "finished"
	StatusAbandoned  ReadStatus = "abandoned"
	StatusUnknown    ReadStatus = "unknown"
)

// LocationType describes the format of a raw location string.
type LocationType string

const (
	LocationPercent      LocationType = "percent"
	LocationPage         LocationType = "page"
	LocationKOReaderXPtr LocationType = "koreader_xpointer"
	LocationMoonPosition LocationType = "moon_position"
	LocationEpubCFI      LocationType = "epub_cfi"
	LocationRaw          LocationType = "raw"
)

// OutboxStatus is the state machine for outbox jobs.
type OutboxStatus string

const (
	OutboxQueued                 OutboxStatus = "queued"
	OutboxRunning                OutboxStatus = "running"
	OutboxSucceeded              OutboxStatus = "succeeded"
	OutboxRetrying               OutboxStatus = "retrying"
	OutboxDeadLetter             OutboxStatus = "deadletter"
	OutboxBlockedByConflict      OutboxStatus = "blocked_by_conflict"
	OutboxBlockedByLowConfidence OutboxStatus = "blocked_by_low_confidence"
	OutboxBlockedByAdapterHealth OutboxStatus = "blocked_by_adapter_health"
)

// ConflictStatus tracks resolution state of a conflict.
type ConflictStatus string

const (
	ConflictOpen         ConflictStatus = "open"
	ConflictAutoResolved ConflictStatus = "auto_resolved"
	ConflictUserPinned   ConflictStatus = "user_pinned"
	ConflictDismissed    ConflictStatus = "dismissed"
)

// AdapterHealthState represents the health of an adapter.
type AdapterHealthState string

const (
	HealthOK              AdapterHealthState = "ok"
	HealthDegraded        AdapterHealthState = "degraded"
	HealthNeedsUserAction AdapterHealthState = "needs_user_action"
	HealthFailed          AdapterHealthState = "failed"
	HealthDisabled        AdapterHealthState = "disabled"
)
