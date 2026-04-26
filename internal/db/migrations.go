// internal/db/migrations.go
//
// SQL migration definitions for ReadSync's SQLite schema.

package db

// migration holds a single versioned SQL statement.
type migration struct {
	version int
	sql     string
}

// migrations is the ordered list of all schema migrations.
// New migrations MUST be appended; existing entries MUST NOT be modified.
var migrations = []migration{


	{version: 1, sql: migration1},
	{version: 2, sql: migration2},
	{version: 3, sql: migration3},
}

const migration1 = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS books (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    calibre_id          INTEGER,
    goodreads_id        TEXT,
    isbn13              TEXT,
    isbn10              TEXT,
    asin                TEXT,
    epub_id             TEXT,
    file_hash           TEXT,
    title               TEXT NOT NULL DEFAULT '',
    author_sort         TEXT NOT NULL DEFAULT '',
    page_count          INTEGER,
    identity_confidence INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_books_file_hash
    ON books(file_hash) WHERE file_hash IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_books_calibre_id
    ON books(calibre_id) WHERE calibre_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_books_goodreads_id
    ON books(goodreads_id) WHERE goodreads_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_books_isbn13
    ON books(isbn13) WHERE isbn13 IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_books_asin
    ON books(asin) WHERE asin IS NOT NULL;

CREATE TABLE IF NOT EXISTS book_aliases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id     INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    source      TEXT NOT NULL,
    adapter_key TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(source, adapter_key)
);
CREATE INDEX IF NOT EXISTS idx_book_aliases_book_id ON book_aliases(book_id);

CREATE TABLE IF NOT EXISTS progress_events (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id             INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    source              TEXT NOT NULL,
    received_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    device_ts           TEXT,
    percent_complete    REAL,
    page_number         INTEGER,
    total_pages         INTEGER,
    raw_locator         TEXT,
    locator_type        TEXT NOT NULL DEFAULT 'raw',
    read_status         TEXT NOT NULL DEFAULT 'unknown',
    identity_confidence INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_progress_events_book_id
    ON progress_events(book_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_progress_events_source
    ON progress_events(source, received_at DESC);

CREATE TABLE IF NOT EXISTS conflicts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id         INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    detected_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    resolved_at     TEXT,
    status          TEXT NOT NULL DEFAULT 'open',
    event_a_id      INTEGER NOT NULL REFERENCES progress_events(id),
    event_b_id      INTEGER NOT NULL REFERENCES progress_events(id),
    winner_event_id INTEGER REFERENCES progress_events(id),
    reason          TEXT NOT NULL DEFAULT '',
    resolved_by     TEXT
);
CREATE INDEX IF NOT EXISTS idx_conflicts_book_id ON conflicts(book_id, status);
CREATE INDEX IF NOT EXISTS idx_conflicts_status   ON conflicts(status, detected_at DESC);

CREATE TABLE IF NOT EXISTS canonical_progress (
    book_id          INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_by       TEXT NOT NULL,
    event_id         INTEGER NOT NULL REFERENCES progress_events(id),
    percent_complete REAL,
    page_number      INTEGER,
    total_pages      INTEGER,
    raw_locator      TEXT,
    locator_type     TEXT NOT NULL DEFAULT 'raw',
    read_status      TEXT NOT NULL DEFAULT 'unknown',
    user_pinned      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sync_outbox (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    book_id              INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    target_source        TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'queued',
    attempts             INTEGER NOT NULL DEFAULT 0,
    next_retry_at        TEXT,
    last_error           TEXT,
    payload              TEXT NOT NULL DEFAULT '{}',
    blocking_conflict_id INTEGER REFERENCES conflicts(id)
);
CREATE INDEX IF NOT EXISTS idx_sync_outbox_status
    ON sync_outbox(status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_sync_outbox_target
    ON sync_outbox(target_source, status);

CREATE TABLE IF NOT EXISTS adapter_health (
    source          TEXT PRIMARY KEY,
    state           TEXT NOT NULL DEFAULT 'ok',
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_error      TEXT,
    consec_failures INTEGER NOT NULL DEFAULT 0,
    notes           TEXT
);
`

const migration2 = `
CREATE TABLE IF NOT EXISTS koreader_users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    created_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at    TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS koreader_devices (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES koreader_users(id) ON DELETE CASCADE,
    device_id   TEXT    NOT NULL,
    device_name TEXT    NOT NULL DEFAULT '',
    last_seen   TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE(user_id, device_id)
);
CREATE INDEX IF NOT EXISTS idx_koreader_devices_user ON koreader_devices(user_id);
`


const migration3 = `
CREATE TABLE IF NOT EXISTS moon_users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT    NOT NULL UNIQUE,
    password_hash   TEXT    NOT NULL,
    created_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS moon_uploads (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES moon_users(id) ON DELETE CASCADE,
    rel_path        TEXT    NOT NULL,
    version         INTEGER NOT NULL,
    received_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    size_bytes      INTEGER NOT NULL,
    sha256          TEXT    NOT NULL,
    archive_path    TEXT    NOT NULL,
    parsed          INTEGER NOT NULL DEFAULT 0,
    parse_error     TEXT,
    UNIQUE(user_id, rel_path, version)
);
CREATE INDEX IF NOT EXISTS idx_moon_uploads_user_path
    ON moon_uploads(user_id, rel_path, version DESC);
CREATE INDEX IF NOT EXISTS idx_moon_uploads_received
    ON moon_uploads(received_at DESC);
`