// Package resolver maps adapter-supplied evidence to a canonical book identity.
//
// The core function is Score, which evaluates up to 10 signals in priority
// order (file_hash=100, epub_id=97, calibre_id=95, … moon_key=65) and returns
// a Match with a confidence score (0–100) and a reason string.
//
// Confidence bands:
//   - 95–100 BandAutoResolve:      auto-resolve conflicts + writeback
//   - 80–94  BandWritebackSafe:    writeback enabled; conflict auto-resolved
//   - 60–79  BandWritebackWary:    writeback enabled; suspicious events trigger conflict
//   - 40–59  BandUserReview:       writeback disabled; flag for user review
//   - 0–39   BandQuarantine:       no writeback; event quarantined
//
// All functions in this package are pure (no I/O, no CGO).
package resolver
