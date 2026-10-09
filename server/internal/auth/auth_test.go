package auth

import (
	"database/sql"
	"testing"
	"time"

	"github.com/bgguna/photography/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	schema := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);

	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
	`

	if _, err := database.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return database
}

func TestHashPassword(t *testing.T) {
	password := "mysecretpassword"
	hash, err := HashPassword(password)

	if err != nil {
		t.Errorf("HashPassword() returned error: %v", err)
	}
	if hash == "" {
		t.Error("HashPassword() returned empty hash")
	}
	if hash == password {
		t.Error("HashPassword() should not return plaintext password")
	}
}

func TestVerifyPassword_Valid(t *testing.T) {
	password := "mysecretpassword"
	hash, _ := HashPassword(password)

	if !VerifyPassword(hash, password) {
		t.Error("VerifyPassword() should return true for correct password")
	}
}

func TestVerifyPassword_Invalid(t *testing.T) {
	password := "mysecretpassword"
	hash, _ := HashPassword(password)

	if VerifyPassword(hash, "wrongpassword") {
		t.Error("VerifyPassword() should return false for wrong password")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	if VerifyPassword("not-a-valid-hash", "password") {
		t.Error("VerifyPassword() should return false for invalid hash")
	}
}

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	user, err := as.CreateUser("admin@example.com", "password123", "admin")

	if err != nil {
		t.Fatalf("CreateUser() returned error: %v", err)
	}
	if user == nil {
		t.Fatal("CreateUser() returned nil user")
	}
	if user.Email != "admin@example.com" {
		t.Errorf("Email = %s, want admin@example.com", user.Email)
	}
	if user.Role != "admin" {
		t.Errorf("Role = %s, want admin", user.Role)
	}
	if user.ID == 0 {
		t.Error("User ID should not be 0")
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")

	// Try to create another user with the same email
	_, err := as.CreateUser("admin@example.com", "password456", "admin")
	if err == nil {
		t.Fatal("CreateUser() should return error for duplicate email")
	}
}

func TestLogin_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")

	session, err := as.Login("admin@example.com", "password123", 7*24*time.Hour)

	if err != nil {
		t.Fatalf("Login() returned error: %v", err)
	}
	if session == nil {
		t.Fatal("Login() returned nil session")
	}
	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if session.UserID == 0 {
		t.Error("User ID should not be 0")
	}
	if time.Now().After(session.ExpiresAt) {
		t.Error("Session should not be expired")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")

	_, err := as.Login("admin@example.com", "wrongpassword", 7*24*time.Hour)

	if err == nil {
		t.Fatal("Login() should return error for wrong password")
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	_, err := as.Login("nonexistent@example.com", "password123", 7*24*time.Hour)

	if err == nil {
		t.Fatal("Login() should return error for nonexistent user")
	}
}

func TestGetSession_Valid(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	user, _ := as.CreateUser("admin@example.com", "password123", "admin")
	session, _ := as.Login("admin@example.com", "password123", 7*24*time.Hour)

	retrieved, err := as.GetSession(session.ID)

	if err != nil {
		t.Fatalf("GetSession() returned error: %v", err)
	}
	if retrieved.ID != session.ID {
		t.Errorf("Session ID = %s, want %s", retrieved.ID, session.ID)
	}
	if retrieved.UserID != user.ID {
		t.Errorf("User ID = %d, want %d", retrieved.UserID, user.ID)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	_, err := as.GetSession("nonexistent-session-id")

	if err == nil {
		t.Fatal("GetSession() should return error for nonexistent session")
	}
}

func TestGetSession_Expired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")

	// Create a session with very short expiry
	session, _ := as.Login("admin@example.com", "password123", -1*time.Second)

	// Try to retrieve the expired session
	_, err := as.GetSession(session.ID)

	if err == nil {
		t.Fatal("GetSession() should return error for expired session")
	}
}

func TestGetUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	created, _ := as.CreateUser("admin@example.com", "password123", "admin")

	user, err := as.GetUser(created.ID)

	if err != nil {
		t.Fatalf("GetUser() returned error: %v", err)
	}
	if user.ID != created.ID {
		t.Errorf("User ID = %d, want %d", user.ID, created.ID)
	}
	if user.Email != "admin@example.com" {
		t.Errorf("Email = %s, want admin@example.com", user.Email)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	_, err := as.GetUser(999)

	if err == nil {
		t.Fatal("GetUser() should return error for nonexistent user")
	}
}

func TestLogout(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")
	session, _ := as.Login("admin@example.com", "password123", 7*24*time.Hour)

	err := as.Logout(session.ID)

	if err != nil {
		t.Fatalf("Logout() returned error: %v", err)
	}

	// Verify session is deleted
	_, err = as.GetSession(session.ID)
	if err == nil {
		t.Fatal("GetSession() should return error after logout")
	}
}

func TestCleanupSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	as := NewAuthService(db)
	as.CreateUser("admin@example.com", "password123", "admin")

	// Create multiple sessions with different expiry times
	session1, _ := as.Login("admin@example.com", "password123", 1*time.Hour)
	_, _ = as.Login("admin@example.com", "password123", -1*time.Second) // Already expired

	// Cleanup should remove expired session
	as.cleanupSessions()

	// Session 1 should still exist
	_, err := as.GetSession(session1.ID)
	if err != nil {
		t.Error("Valid session should still exist after cleanup")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()

	// First request should be allowed
	if !rl.Allow("192.168.1.1:12345", 3, 1*time.Minute) {
		t.Error("First request should be allowed")
	}

	// Next two should be allowed (3 total)
	if !rl.Allow("192.168.1.1:12345", 3, 1*time.Minute) {
		t.Error("Second request should be allowed")
	}
	if !rl.Allow("192.168.1.1:12345", 3, 1*time.Minute) {
		t.Error("Third request should be allowed")
	}

	// Fourth should be rate limited
	if rl.Allow("192.168.1.1:12345", 3, 1*time.Minute) {
		t.Error("Fourth request should be rate limited")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter()

	// Different IPs should have independent limits
	if !rl.Allow("192.168.1.1", 2, 1*time.Minute) {
		t.Error("IP 1 first request should be allowed")
	}
	if !rl.Allow("192.168.1.2", 2, 1*time.Minute) {
		t.Error("IP 2 first request should be allowed")
	}

	// Exhaust IP 1
	if !rl.Allow("192.168.1.1", 2, 1*time.Minute) {
		t.Error("IP 1 second request should be allowed")
	}
	if rl.Allow("192.168.1.1", 2, 1*time.Minute) {
		t.Error("IP 1 third request should be rate limited")
	}

	// IP 2 should still have requests
	if !rl.Allow("192.168.1.2", 2, 1*time.Minute) {
		t.Error("IP 2 second request should be allowed")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter()
	rl.Allow("192.168.1.1", 1, 1*time.Hour)

	// Cleanup with very short maxAge should remove the bucket
	rl.Cleanup(1 * time.Nanosecond)
	time.Sleep(1 * time.Millisecond) // Ensure time passes

	// Now new request should get full tokens
	if !rl.Allow("192.168.1.1", 3, 1*time.Minute) {
		t.Error("After cleanup, new request should be allowed with fresh bucket")
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1, err1 := generateSessionID()
	id2, err2 := generateSessionID()

	if err1 != nil || err2 != nil {
		t.Fatal("generateSessionID() should not return error")
	}
	if id1 == "" || id2 == "" {
		t.Error("generateSessionID() should return non-empty ID")
	}
	if id1 == id2 {
		t.Error("generateSessionID() should return unique IDs")
	}
	if len(id1) != 64 { // 32 bytes * 2 hex chars
		t.Errorf("Session ID length = %d, want 64", len(id1))
	}
}
