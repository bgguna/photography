package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	sessionCookieName = "photography_session"
	userContextKey    = "user"
)

// AuthMiddleware creates a Gin middleware that checks for valid sessions.
func AuthMiddleware(as *AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session cookie
		sessionID, err := c.Cookie(sessionCookieName)
		if err != nil {
			redirectOrUnauthorized(c)
			return
		}

		// Get session from database
		session, err := as.GetSession(sessionID)
		if err != nil {
			redirectOrUnauthorized(c)
			return
		}

		// Get user from database
		user, err := as.GetUser(session.UserID)
		if err != nil {
			redirectOrUnauthorized(c)
			return
		}

		// Store user in context
		c.Set(userContextKey, user)
		c.Next()
	}
}

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(c *gin.Context) *User {
	user, exists := c.Get(userContextKey)
	if !exists {
		return nil
	}
	if u, ok := user.(*User); ok {
		return u
	}
	return nil
}

// redirectOrUnauthorized redirects to login page for regular requests,
// returns 401 for AJAX/htmx requests.
func redirectOrUnauthorized(c *gin.Context) {
	// Check if this is an htmx request
	if c.GetHeader("HX-Request") == "true" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	// For regular requests, redirect to login
	c.Redirect(http.StatusFound, "/admin/login")
	c.Abort()
}

// SetSessionCookie sets the session cookie on the response.
func SetSessionCookie(c *gin.Context, sessionID string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		sessionCookieName,
		sessionID,
		86400*7, // 7 days in seconds
		"/",
		"",
		secure,
		true, // HttpOnly
	)
}

// ClearSessionCookie clears the session cookie.
func ClearSessionCookie(c *gin.Context) {
	c.SetCookie(
		sessionCookieName,
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
}
