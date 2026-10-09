package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddleware_ValidSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	_, _ = as.CreateUser("admin@example.com", "password123", "admin")
	session, _ := as.Login("admin@example.com", "password123", 7*24*time.Hour)

	// Create a test handler
	router := gin.New()
	router.GET("/admin", AuthMiddleware(as), func(c *gin.Context) {
		u := GetUserFromContext(c)
		if u == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"email": u.Email})
	})

	// Test with valid session cookie
	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{
		Name:  "photography_session",
		Value: session.ID,
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != `{"email":"admin@example.com"}` {
		t.Errorf("Response = %s, want email in JSON", body)
	}
}

func TestAuthMiddleware_NoSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)

	router := gin.New()
	router.GET("/admin", AuthMiddleware(as), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test without session cookie
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d (redirect)", w.Code, http.StatusFound)
	}

	location := w.Header().Get("Location")
	if location != "/admin/login" {
		t.Errorf("Location = %s, want /admin/login", location)
	}
}

func TestAuthMiddleware_ExpiredSession(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")
	session, _ := as.Login("admin@example.com", "password123", -1*time.Second) // Already expired

	router := gin.New()
	router.GET("/admin", AuthMiddleware(as), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(&http.Cookie{
		Name:  "photography_session",
		Value: session.ID,
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d (redirect)", w.Code, http.StatusFound)
	}
}

func TestAuthMiddleware_HTMXRequest(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)

	router := gin.New()
	router.GET("/admin", AuthMiddleware(as), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Test with HX-Request header but no session
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if w.Header().Get("Location") != "" {
		t.Error("Should not redirect for AJAX requests")
	}
}

func TestSetSessionCookie(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		SetSessionCookie(c, "test-session-id", false)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	var found bool
	for _, cookie := range cookies {
		if cookie.Name == "photography_session" && cookie.Value == "test-session-id" {
			if !cookie.HttpOnly {
				t.Error("Cookie should be HttpOnly")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Error("Cookie should use SameSite=Lax")
			}
			found = true
		}
	}

	if !found {
		t.Error("Session cookie not found or incorrect")
	}
}

func TestClearSessionCookie(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		ClearSessionCookie(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "photography_session" {
			if cookie.Value != "" {
				t.Error("Cookie value should be empty after clear")
			}
			if cookie.MaxAge != -1 {
				t.Error("Cookie MaxAge should be -1 after clear")
			}
		}
	}
}

func TestGetUserFromContext(t *testing.T) {
	// Test when user is set
	c := &gin.Context{}
	user := &User{ID: 1, Email: "test@example.com"}
	c.Set("user", user)

	retrieved := GetUserFromContext(c)
	if retrieved == nil {
		t.Fatal("GetUserFromContext returned nil")
	}
	if retrieved.Email != "test@example.com" {
		t.Errorf("Email = %s, want test@example.com", retrieved.Email)
	}

	// Test when user is not set
	c2 := &gin.Context{}
	retrieved2 := GetUserFromContext(c2)
	if retrieved2 != nil {
		t.Error("GetUserFromContext should return nil when user not set")
	}

	// Test when user is wrong type
	c3 := &gin.Context{}
	c3.Set("user", "not-a-user")
	retrieved3 := GetUserFromContext(c3)
	if retrieved3 != nil {
		t.Error("GetUserFromContext should return nil for wrong type")
	}
}
