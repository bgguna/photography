package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	csrfContextKey = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfFormField  = "csrf_token"
)

// NewCSRFKey returns a random key for signing CSRF tokens.
func NewCSRFKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("csrf: cannot read random bytes: " + err.Error())
	}
	return key
}

// CSRFToken derives the CSRF token bound to a session ID.
func CSRFToken(key []byte, sessionID string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))
}

// CSRFMiddleware exposes the session's CSRF token to handlers and rejects
// state-changing requests that don't present it. Must run after AuthMiddleware.
func CSRFMiddleware(key []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		expected := CSRFToken(key, sessionID)
		c.Set(csrfContextKey, expected)

		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		got := c.GetHeader(csrfHeaderName)
		if got == "" {
			got = c.PostForm(csrfFormField)
		}
		if !hmac.Equal([]byte(got), []byte(expected)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}
		c.Next()
	}
}

// CSRFTokenFromContext returns the token set by CSRFMiddleware.
func CSRFTokenFromContext(c *gin.Context) string {
	return c.GetString(csrfContextKey)
}
