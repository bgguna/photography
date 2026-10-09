package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents an authenticated user.
type User struct {
	ID    int
	Email string
	Role  string
}

// Session represents an active session.
type Session struct {
	ID        string
	UserID    int
	ExpiresAt time.Time
}

// AuthService handles authentication operations.
type AuthService struct {
	db *sql.DB
}

// NewAuthService creates a new AuthService.
func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db}
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash.
func VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Login authenticates a user and creates a session.
func (as *AuthService) Login(email, password string, sessionDuration time.Duration) (*Session, error) {
	// Look up user by email
	user, err := as.getUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if !VerifyPassword(user.passwordHash, password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session: %w", err)
	}

	// Insert session
	expiresAt := time.Now().Add(sessionDuration)
	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	}

	if err := as.createSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Clean up old sessions
	_ = as.cleanupSessions()

	return session, nil
}

// GetSession retrieves a session by ID.
func (as *AuthService) GetSession(sessionID string) (*Session, error) {
	var session Session
	var expiresAtStr string

	err := as.db.QueryRow(
		"SELECT id, user_id, expires_at FROM sessions WHERE id = ?",
		sessionID,
	).Scan(&session.ID, &session.UserID, &expiresAtStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Parse expires_at
	expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse session expiry: %w", err)
	}
	session.ExpiresAt = expiresAt

	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		_ = as.deleteSession(sessionID)
		return nil, fmt.Errorf("session expired")
	}

	return &session, nil
}

// GetUser retrieves a user by ID.
func (as *AuthService) GetUser(userID int) (*User, error) {
	var user User
	err := as.db.QueryRow(
		"SELECT id, email, role FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Email, &user.Role)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// Logout deletes a session.
func (as *AuthService) Logout(sessionID string) error {
	return as.deleteSession(sessionID)
}

// CreateUser creates a new user with hashed password.
func (as *AuthService) CreateUser(email, password, role string) (*User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	result, err := as.db.Exec(
		"INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)",
		email, hash, role,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	return &User{
		ID:    int(userID),
		Email: email,
		Role:  role,
	}, nil
}

// getUserByEmail looks up a user by email (internal).
func (as *AuthService) getUserByEmail(email string) (*struct {
	ID           int
	passwordHash string
}, error) {
	var user struct {
		ID           int
		passwordHash string
	}

	err := as.db.QueryRow(
		"SELECT id, password_hash FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.passwordHash)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// createSession inserts a session into the database.
func (as *AuthService) createSession(session *Session) error {
	expiresAtStr := session.ExpiresAt.Format(time.RFC3339Nano)
	_, err := as.db.Exec(
		"INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)",
		session.ID, session.UserID, expiresAtStr,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}
	return nil
}

// deleteSession removes a session from the database.
func (as *AuthService) deleteSession(sessionID string) error {
	_, err := as.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// cleanupSessions removes expired sessions.
func (as *AuthService) cleanupSessions() error {
	_, err := as.db.Exec("DELETE FROM sessions WHERE expires_at < datetime('now')")
	if err != nil {
		return fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	return nil
}

// generateSessionID generates a random session ID.
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// RateLimiter implements a simple token bucket rate limiter keyed by IP.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens    int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
	}
}

// Allow checks if a request from the given IP should be allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(ip string, maxRequests int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Extract base IP (remove port if present)
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}

	bucket, exists := rl.buckets[host]
	now := time.Now()

	if !exists || now.Sub(bucket.lastRefill) > window {
		// Create or reset bucket
		rl.buckets[host] = &tokenBucket{
			tokens:    maxRequests - 1,
			lastRefill: now,
		}
		return true
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// Cleanup removes old buckets from the rate limiter.
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, bucket := range rl.buckets {
		if now.Sub(bucket.lastRefill) > maxAge {
			delete(rl.buckets, ip)
		}
	}
}
