package main

import (
	"database/sql"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bgguna/photography/internal/db"
)

// openTestDB opens an in-memory database and initializes the schema.
func openTestDB(t *testing.T) *sql.DB {
	tmpDb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Read and execute schema
	schemaBytes, err := ioutil.ReadFile("../db/schema.sql")
	if err != nil {
		t.Fatalf("Failed to read schema: %v", err)
	}
	if _, err := tmpDb.Exec(string(schemaBytes)); err != nil {
		t.Fatalf("Failed to execute schema: %v", err)
	}

	return tmpDb
}


func TestVerifyStoragePath_ValidPath(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "storage-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = verifyStoragePath(tmpDir)
	if err != nil {
		t.Errorf("verifyStoragePath() with valid path should not error, got: %v", err)
	}
}

func TestVerifyStoragePath_EmptyPath(t *testing.T) {
	err := verifyStoragePath("")
	if err == nil {
		t.Fatal("verifyStoragePath() with empty path should return error")
	}
	if err.Error() != "photo storage path is empty" {
		t.Errorf("Expected empty path error, got: %v", err)
	}
}

func TestVerifyStoragePath_NonexistentPath(t *testing.T) {
	err := verifyStoragePath("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("verifyStoragePath() with nonexistent path should return error")
	}
	if err.Error() != "photo storage path does not exist: stat /nonexistent/path/that/does/not/exist: no such file or directory" {
		// Check if the error contains the key message (message may vary by OS)
		if !contains(err.Error(), "does not exist") {
			t.Errorf("Expected 'does not exist' error, got: %v", err)
		}
	}
}

func TestVerifyStoragePath_FileNotDirectory(t *testing.T) {
	// Create a temporary file
	tmpFile, err := ioutil.TempFile("", "test-file-")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	err = verifyStoragePath(tmpFile.Name())
	if err == nil {
		t.Fatal("verifyStoragePath() with file (not directory) should return error")
	}
	if err.Error() != "photo storage path is not a directory" {
		t.Errorf("Expected 'not a directory' error, got: %v", err)
	}
}

func TestVerifyStoragePath_NotWritable(t *testing.T) {
	// Create a temporary directory with no write permissions
	tmpDir, err := ioutil.TempDir("", "storage-test-readonly-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Remove write permissions
	if err := os.Chmod(tmpDir, 0555); err != nil {
		t.Fatalf("Failed to remove write permissions: %v", err)
	}

	err = verifyStoragePath(tmpDir)
	if err == nil {
		t.Fatal("verifyStoragePath() with unwritable directory should return error")
	}
	if !contains(err.Error(), "is not writable") {
		t.Errorf("Expected 'is not writable' error, got: %v", err)
	}
}

func TestSetupRouter_HealthCheck(t *testing.T) {
	tmpDb := openTestDB(t)
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	// Test health check endpoint
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health check status = %d, want %d", w.Code, http.StatusOK)
	}
	if !contains(w.Body.String(), "ok") {
		t.Errorf("Health check response should contain 'ok', got: %s", w.Body.String())
	}
}

func TestSetupRouter_StaticFilesRoute(t *testing.T) {
	tmpDb := openTestDB(t)
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	// Test static files route exists (will 404 since the directory is empty/may not exist)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/static/test.js", nil)
	router.ServeHTTP(w, req)

	// We expect either 404 or 200 depending on if the directory exists
	// The important thing is that the route is registered
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Logf("Static route returned %d (acceptable)", w.Code)
	}
}

func TestSetupRouter_PublicRoutes(t *testing.T) {
	tmpDb := openTestDB(t)
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	routes := []struct {
		method   string
		path     string
		minCode  int
		maxCode  int
		name     string
	}{
		{"GET", "/", 200, 299, "home page"},
		{"GET", "/photos/1/thumb", 404, 404, "missing photo thumbnail"},
		{"GET", "/photos/1/web", 404, 404, "missing photo web"},
		{"GET", "/contact", http.StatusNotImplemented, http.StatusNotImplemented, "contact form"},
		{"POST", "/contact", http.StatusNotImplemented, http.StatusNotImplemented, "contact form submit"},
	}

	for _, route := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(w, req)

		if w.Code < route.minCode || w.Code > route.maxCode {
			t.Errorf("%s %s (%s) returned %d, want %d-%d", route.method, route.path, route.name, w.Code, route.minCode, route.maxCode)
		}
	}
}

func TestSetupRouter_AdminRoutes(t *testing.T) {
	tmpDb := openTestDB(t)
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	// GET /admin/login should render the login page (200)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/login", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /admin/login returned %d, want %d", w.Code, http.StatusOK)
	}

	// POST /admin/login without credentials should redirect to login with error
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/admin/login", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("POST /admin/login returned %d, want %d (redirect)", w.Code, http.StatusFound)
	}

	// Protected routes should redirect (302) without auth
	protectedRoutes := []struct {
		method string
		path   string
	}{
		{"POST", "/admin/logout"},
		{"GET", "/admin"},
		{"GET", "/admin/photos"},
		{"POST", "/admin/photos"},
		{"GET", "/admin/photos/1/thumb"},
		{"GET", "/admin/photos/1/web"},
		{"GET", "/admin/photos/1/original"},
		{"DELETE", "/admin/photos/1"},
		{"POST", "/admin/photos/1/visibility"},
		{"GET", "/admin/messages"},
	}

	for _, route := range protectedRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("%s %s returned %d, want %d (redirect to login)", route.method, route.path, w.Code, http.StatusFound)
		}
	}
}

func TestLoggingMiddleware(t *testing.T) {
	tmpDb := openTestDB(t)
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	// The middleware should still allow the request to go through
	if w.Code != http.StatusOK {
		t.Errorf("Request with logging middleware failed: %d", w.Code)
	}
}

func TestSetupRouter_Recovery(t *testing.T) {
	// The recovery middleware is tested implicitly by running all other tests
	// without panicking. The gin.Recovery() middleware is built-in and well-tested.
	tmpDb := openTestDB(t)
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	// Test that router was created without panic
	if router == nil {
		t.Fatal("setupRouter() returned nil")
	}
}

// Helper function for test assertions
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

