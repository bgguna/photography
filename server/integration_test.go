package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bgguna/photography/internal/config"
	"github.com/bgguna/photography/internal/db"
)

// TestIntegration_ConfigAndRouter tests the config loading and router setup together.
func TestIntegration_ConfigAndRouter(t *testing.T) {
	// Setup temp directory
	tmpDir, err := ioutil.TempDir("", "integration-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup environment
	os.Setenv("PHOTO_STORAGE_PATH", tmpDir)
	os.Setenv("PORT", "9999")
	os.Setenv("DB_PATH", ":memory:")
	os.Setenv("MODE_ENV", "development")
	defer func() {
		os.Unsetenv("PHOTO_STORAGE_PATH")
		os.Unsetenv("PORT")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("MODE_ENV")
	}()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Config load failed: %v", err)
	}

	if cfg.Port != "9999" {
		t.Errorf("Port not loaded correctly: %s", cfg.Port)
	}

	// Verify storage path
	if err := verifyStoragePath(cfg.PhotoStoragePath); err != nil {
		t.Fatalf("Storage path verification failed: %v", err)
	}

	// Open database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("Database open failed: %v", err)
	}
	defer database.Close()

	// Setup router
	router := setupRouter(database)

	// Test a few endpoints
	endpoints := []struct {
		method string
		path   string
		expect int
	}{
		{"GET", "/healthz", http.StatusOK},
		{"GET", "/", http.StatusNotImplemented},
		{"GET", "/admin/login", http.StatusNotImplemented},
	}

	for _, ep := range endpoints {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(ep.method, ep.path, nil)
		router.ServeHTTP(w, req)

		if w.Code != ep.expect {
			t.Errorf("%s %s: got %d, want %d", ep.method, ep.path, w.Code, ep.expect)
		}
	}
}

// TestIntegration_RouterWithRealDatabase tests the router with a real database file.
func TestIntegration_RouterWithRealDatabase(t *testing.T) {
	// Create a temporary database file
	tmpFile, err := ioutil.TempFile("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp database file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Setup temp directory
	tmpDir, err := ioutil.TempDir("", "integration-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup environment
	os.Setenv("PHOTO_STORAGE_PATH", tmpDir)
	os.Setenv("DB_PATH", tmpFile.Name())
	os.Setenv("MODE_ENV", "development")
	defer func() {
		os.Unsetenv("PHOTO_STORAGE_PATH")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("MODE_ENV")
	}()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Config load failed: %v", err)
	}

	// Open database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("Database open failed: %v", err)
	}
	defer database.Close()

	// Setup router
	router := setupRouter(database)

	// Test health check
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health check failed: got %d, want %d", w.Code, http.StatusOK)
	}
}

// TestIntegration_AllPublicRoutes ensures all public routes are accessible.
func TestIntegration_AllPublicRoutes(t *testing.T) {
	tmpDb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	publicRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/photos/123/thumb"},
		{"GET", "/photos/123/web"},
		{"GET", "/contact"},
		{"POST", "/contact"},
		{"GET", "/healthz"},
	}

	for _, route := range publicRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(w, req)

		// Should not return 404 for route not found
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s: route not registered (got 404)", route.method, route.path)
		}
	}
}

// TestIntegration_AllAdminRoutes ensures all admin routes are accessible.
func TestIntegration_AllAdminRoutes(t *testing.T) {
	tmpDb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer tmpDb.Close()

	router := setupRouter(tmpDb)

	adminRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/login"},
		{"POST", "/admin/login"},
		{"POST", "/admin/logout"},
		{"GET", "/admin"},
		{"GET", "/admin/photos"},
		{"POST", "/admin/photos"},
		{"GET", "/admin/photos/123/thumb"},
		{"GET", "/admin/photos/123/web"},
		{"GET", "/admin/photos/123/original"},
		{"POST", "/admin/photos/123/delete"},
		{"POST", "/admin/photos/123/visibility"},
		{"GET", "/admin/messages"},
	}

	for _, route := range adminRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(w, req)

		// Should not return 404 for route not found
		if w.Code == http.StatusNotFound {
			t.Errorf("%s %s: route not registered (got 404)", route.method, route.path)
		}
	}
}
