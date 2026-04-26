// internal/adapters/koreader/handlers.go
//
// HTTP handlers for the KOSync-compatible endpoints:
//   POST /users/create
//   GET  /users/auth
//   PUT  /syncs/progress
//   GET  /syncs/progress/:document

package koreader

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readsync/readsync/internal/logging"
)

// sha256HexRe accepts only a 64-char hex string (KOReader SHA256).
var sha256HexRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// registerRequest is the JSON body for POST /users/create.
type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// handleRegister implements POST /users/create.
func (a *Adapter) handleRegister(c *gin.Context) {
	if !a.cfg.RegistrationOpen {
		c.JSON(http.StatusForbidden, gin.H{"message": "Registration is closed."})
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "username and password are required."})
		return
	}
	if err := a.createUser(req.Username, req.Password); err != nil {
		if errors.Is(err, ErrUserExists) {
			c.JSON(402, gin.H{"message": "Username is already registered."})
			return
		}
		a.log.Error("register: create user", logging.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal error."})
		return
	}
	a.log.Info("koreader: user registered", logging.F("username", req.Username))
	c.JSON(http.StatusCreated, gin.H{"username": req.Username})
}

// handleAuth implements GET /users/auth.
// authMiddleware already validated credentials; just confirm.
func (a *Adapter) handleAuth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"authorized": "OK"})
}

// handlePush implements PUT /syncs/progress.
//
// KOSync 412 semantics: if the server already has a NEWER canonical entry for
// this document (from a different device that synced more recently), reject the
// push and tell the client the server's current timestamp so KOReader can skip
// the stale update.
func (a *Adapter) handlePush(c *gin.Context) {
	auth, _ := getAuth(c)

	var req pushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body."})
		return
	}
	if !sha256HexRe.MatchString(req.Document) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid document hash format."})
		return
	}
	if req.Percentage < 0 || req.Percentage > 1.01 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "percentage out of range."})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Load existing canonical BEFORE submit to detect stale pushes.
	// If the server already has a newer entry (by timestamp), return 412.
	existing, err := a.loadCanonicalByDocHash(ctx, req.Document)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		a.log.Warn("koreader: pre-push canonical load", logging.F("error", err))
	}
	now := time.Now()
	if existing != nil && !existing.UpdatedAt.IsZero() &&
		existing.UpdatedAt.After(now.Add(-time.Second)) &&
		existing.PercentComplete != nil &&
		*existing.PercentComplete > req.Percentage+0.001 {
		// Server canonical is newer and at a higher position — client is stale.
		c.JSON(http.StatusPreconditionFailed, stalePushResponse{
			Message:   "Document update is not newer.",
			Document:  req.Document,
			Timestamp: existing.UpdatedAt.Unix(),
		})
		return
	}

	if req.DeviceID != "" {
		if err := a.upsertDevice(auth.UserID, req.DeviceID, req.Device); err != nil {
			a.log.Warn("koreader: upsert device", logging.F("error", err))
		}
	}

	ev := toAdapterEvent(req)
	if err := a.pipeline.Submit(ctx, ev); err != nil {
		a.log.Error("koreader: pipeline submit", logging.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record progress."})
		return
	}

	a.log.Info("koreader: progress pushed",
		logging.F("username", auth.Username),
		logging.F("document", req.Document),
		logging.F("pct", req.Percentage),
	)
	c.JSON(http.StatusOK, pushResponse{Document: req.Document, Timestamp: now.Unix()})
}

// handlePull implements GET /syncs/progress/:document.
func (a *Adapter) handlePull(c *gin.Context) {
	auth, _ := getAuth(c)
	document := c.Param("document")

	if !sha256HexRe.MatchString(document) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid document hash format."})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	canon, err := a.loadCanonicalByDocHash(ctx, document)
	if errors.Is(err, sql.ErrNoRows) || canon == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	if err != nil {
		a.log.Error("koreader: pull canonical", logging.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal error."})
		return
	}

	deviceID, deviceName := a.lastDeviceForBook(ctx, auth.UserID, canon.BookID)
	resp := canonicalToPull(document, canon.PercentComplete, canon.RawLocator,
		deviceName, deviceID, canon.UpdatedAt.Unix())

	a.log.Info("koreader: progress pulled",
		logging.F("username", auth.Username),
		logging.F("document", document),
	)
	c.JSON(http.StatusOK, resp)
}

// loadCanonicalByDocHash looks up canonical_progress via book_aliases.
func (a *Adapter) loadCanonicalByDocHash(ctx context.Context, docHash string) (*canonicalRow, error) {
	var row canonicalRow
	var updatedAtStr string
	err := a.db.QueryRowContext(ctx, `
		SELECT cp.book_id, cp.updated_at, cp.percent_complete, cp.raw_locator
		FROM canonical_progress cp
		JOIN book_aliases ba ON ba.book_id = cp.book_id
		WHERE ba.source = 'koreader' AND ba.adapter_key = ?
		LIMIT 1
	`, docHash).Scan(&row.BookID, &updatedAtStr, &row.PercentComplete, &row.RawLocator)
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
		row.UpdatedAt = t
	}
	return &row, nil
}

// canonicalRow is a minimal projection of canonical_progress.
type canonicalRow struct {
	BookID          int64
	UpdatedAt       time.Time
	PercentComplete *float64
	RawLocator      *string
}

// lastDeviceForBook returns (deviceID, deviceName) for the most-recently-seen
// device for a given user.  We use last_seen rather than correlating with
// progress_events since the core pipeline does not store source_device.
func (a *Adapter) lastDeviceForBook(ctx context.Context, userID, _ int64) (string, string) {
	var deviceID, deviceName string
	_ = a.db.QueryRowContext(ctx, `
		SELECT device_id, device_name FROM koreader_devices
		WHERE user_id = ? ORDER BY last_seen DESC LIMIT 1
	`, userID).Scan(&deviceID, &deviceName)
	return deviceID, deviceName
}
