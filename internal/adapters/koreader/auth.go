// internal/adapters/koreader/auth.go
//
// Password hashing and authentication helpers for the KOSync adapter.
//
// KOReader sends passwords as hex(md5(plaintext)).  We never store that
// MD5 value (or the plaintext) — on registration we immediately re-hash it
// with bcrypt (cost 12) before writing to the database.  On every
// subsequent request we bcrypt.CompareHashAndPassword(storedHash, md5Key).

package koreader

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// hashMD5Key re-hashes the KOReader md5 key with bcrypt for server-side storage.
// The input is the 32-char hex md5 string that KOReader sends.
func hashMD5Key(md5Key string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(md5Key), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// verifyMD5Key checks a KOReader md5 key against a stored bcrypt hash.
// Returns true only on a valid match.  Uses constant-time comparison inside
// bcrypt to resist timing attacks.
func verifyMD5Key(md5Key, storedHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(md5Key))
	return err == nil
}

// -- Auth middleware ----------------------------------------------------------

// authResult is attached to gin.Context after successful auth.
type authResult struct {
	Username string
	UserID   int64
}

const authKey = "koreader_auth"

// authMiddleware validates x-auth-user / x-auth-key headers and aborts with
// 401 on failure.  On success it stores an authResult in the gin context.
func (a *Adapter) authMiddleware(c *gin.Context) {
	username := c.GetHeader("x-auth-user")
	md5Key := c.GetHeader("x-auth-key")

	// Both headers must be present; absence is not a secret so plain length check is fine.
	if username == "" || md5Key == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	var storedHash string
	var userID int64
	err := a.db.QueryRow(
		`SELECT id, password_hash FROM koreader_users WHERE username=?`, username,
	).Scan(&userID, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		a.recordAuthFailure()
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal error"})
		return
	}

	if !verifyMD5Key(md5Key, storedHash) {
		a.recordAuthFailure()
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return
	}

	a.clearAuthFailures()
	c.Set(authKey, authResult{Username: username, UserID: userID})
	c.Next()
}

// getAuth retrieves the authenticated user from the gin context.
func getAuth(c *gin.Context) (authResult, bool) {
	v, ok := c.Get(authKey)
	if !ok {
		return authResult{}, false
	}
	ar, ok := v.(authResult)
	return ar, ok
}
