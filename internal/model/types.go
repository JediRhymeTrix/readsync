// internal/model/types.go
//
// Struct definitions for all core domain entities.

package model

import "time"

// Book is the canonical identity record for a single book.
type Book struct {
	ID        int64     `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	// Known identifiers (any may be empty).
	CalibreID   *int64  `db:"calibre_id"`
	GoodreadsID *string `db:"goodreads_id"`
	ISBN13      *string `db:"isbn13"`
	ISBN10      *string `db:"isbn10"`
	ASIN        *string `db:"asin"`
	EpubID      *string `db:"epub_id"`
	FileHash    *string `db:"file_hash"`

	// Bibliographic data.
	Title      string `db:"title"`
	AuthorSort string `db:"author_sort"`
	PageCount  *int32 `db:"page_count"`

	// Identity confidence (0-100).
	IdentityConfidence int `db:"identity_confidence"`
}

// BookAlias maps an adapter-specific key to a canonical book ID.
type BookAlias struct {
	ID         int64     `db:"id"`
	BookID     int64     `db:"book_id"`
	Source     Source    `db:"source"`
	AdapterKey string    `db:"adapter_key"`
	CreatedAt  time.Time `db:"created_at"`
}

// ProgressEvent is a single immutable progress observation from one adapter.
type ProgressEvent struct {
	ID         int64      `db:"id"`
	BookID     int64      `db:"book_id"`
	Source     Source     `db:"source"`
	ReceivedAt time.Time  `db:"received_at"`
	DeviceTS   *time.Time `db:"device_ts"`

	// Normalized progress.
	PercentComplete *float64 `db:"percent_complete"` // 0.0-1.0
	PageNumber      *int32   `db:"page_number"`
	TotalPages      *int32   `db:"total_pages"`

	// Raw locator preserved for round-trip fidelity.
	RawLocator  *string      `db:"raw_locator"`
	LocatorType LocationType `db:"locator_type"`

	ReadStatus ReadStatus `db:"read_status"`

	// Resolver outcome at event time.
	IdentityConfidence int `db:"identity_confidence"`
}

// CanonicalProgress is the authoritative reading position for one book.
// One row per book; updated transactionally when a progress event wins.
type CanonicalProgress struct {
	BookID    int64     `db:"book_id"`
	UpdatedAt time.Time `db:"updated_at"`
	UpdatedBy Source    `db:"updated_by"`
	EventID   int64     `db:"event_id"`

	PercentComplete *float64     `db:"percent_complete"`
	PageNumber      *int32       `db:"page_number"`
	TotalPages      *int32       `db:"total_pages"`
	RawLocator      *string      `db:"raw_locator"`
	LocatorType     LocationType `db:"locator_type"`
	ReadStatus      ReadStatus   `db:"read_status"`

	// Whether a user has pinned this value (blocks auto-overwrite).
	UserPinned bool `db:"user_pinned"`
}

// OutboxJob represents a pending write-back to an adapter.
type OutboxJob struct {
	ID           int64        `db:"id"`
	CreatedAt    time.Time    `db:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at"`
	BookID       int64        `db:"book_id"`
	TargetSource Source       `db:"target_source"`
	Status       OutboxStatus `db:"status"`
	Attempts     int          `db:"attempts"`
	NextRetryAt  *time.Time   `db:"next_retry_at"`
	LastError    *string      `db:"last_error"`

	// Payload (JSON-encoded adapter-specific write request).
	Payload string `db:"payload"`

	// Foreign key to conflict that is blocking this job (if any).
	BlockingConflictID *int64 `db:"blocking_conflict_id"`
}

// Conflict records a detected divergence between two progress observations.
type Conflict struct {
	ID         int64          `db:"id"`
	BookID     int64          `db:"book_id"`
	DetectedAt time.Time      `db:"detected_at"`
	ResolvedAt *time.Time     `db:"resolved_at"`
	Status     ConflictStatus `db:"status"`

	// The two competing events.
	EventAID int64 `db:"event_a_id"`
	EventBID int64 `db:"event_b_id"`

	// Which event won (after resolution).
	WinnerEventID *int64 `db:"winner_event_id"`

	// Human-readable reason for the conflict.
	Reason string `db:"reason"`

	// Audit: who resolved and how.
	ResolvedBy *string `db:"resolved_by"`
}

// AdapterHealth stores runtime health for each adapter.
type AdapterHealth struct {
	Source     Source             `db:"source"`
	State      AdapterHealthState `db:"state"`
	UpdatedAt  time.Time          `db:"updated_at"`
	LastError  *string            `db:"last_error"`
	ConsecFail int                `db:"consec_failures"`
	Notes      *string            `db:"notes"`
}
