// Package conflicts implements ReadSync's conflict detection and resolution engine (spec §6).
//
// # Detection
//
// DetectSuspiciousJump checks five conditions:
//  1. Progress decreased by >10% (backward jump without abandon/restart).
//  2. Goodreads reports "finished" from <85% canonical.
//  3. Total page count changed between events.
//  4. Identity confidence <60 (low-trust event).
//  5. Locator type changed (format inconsistency).
//
// # Resolution
//
// Source precedence (lower = more trusted):
//  1. calibre  (ground truth for metadata)
//  2. koreader (reliable timestamps, continuous sync)
//  3. moon     (fire-and-forget WebDAV)
//  4. goodreads_bridge
//  5. kindle_via_goodreads
//
// CanAutoResolve returns true only when all five gating conditions are met:
// trustworthy timestamps, high confidence, plausible movement, writeback enabled, no user pin.
//
// All functions are pure (no I/O, no CGO).
package conflicts
