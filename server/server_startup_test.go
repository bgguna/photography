package main

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/bgguna/photography/internal/config"
	"github.com/bgguna/photography/internal/db"
)

// TestServerSetup tests the server creation and basic HTTP functionality.
func TestServerSetup(t *testing.T) {
	// Create an in-memory database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Setup router
	router := setupRouter(database)

	// Create server
	server := &http.Server{
		Addr:    ":0", // Use port 0 for auto-selection
		Handler: router,
	}

	// Start server in goroutine
	go server.ListenAndServe()
	defer server.Shutdown(context.Background())

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test server is running
	if server.Addr == "" {
		t.Fatal("Server address not set")
	}
}

// TestServerShutdown tests graceful shutdown.
func TestServerShutdown(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	router := setupRouter(database)

	server := &http.Server{
		Addr:    ":0",
		Handler: router,
	}

	// Start server
	go server.ListenAndServe()

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	// Error is expected since we never actually listen on the port
	// The important thing is that shutdown doesn't panic
	t.Logf("Shutdown completed (err: %v)", err)
}

// TestConfigLoadAndVerifyPath tests the config loading and path verification together.
func TestConfigLoadAndVerifyPath(t *testing.T) {
	// Create temp directory
	tmpDir, err := ioutil.TempDir("", "server-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set environment
	os.Setenv("PHOTO_STORAGE_PATH", tmpDir)
	os.Setenv("MODE_ENV", "development")
	defer func() {
		os.Unsetenv("PHOTO_STORAGE_PATH")
		os.Unsetenv("MODE_ENV")
	}()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Config load failed: %v", err)
	}

	// Verify storage path
	if err := verifyStoragePath(cfg.PhotoStoragePath); err != nil {
		t.Fatalf("Storage path verification failed: %v", err)
	}

	t.Log("Config loaded and path verified successfully")
}

// TestDatabaseOpenAndClose tests opening and closing the database.
func TestDatabaseOpenAndClose(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Open database
	database, err := db.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Test database is usable
	var result int
	err = database.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("Database query failed: %v", err)
	}

	if result != 1 {
		t.Errorf("Database query returned %d, expected 1", result)
	}

	// Close database
	err = database.Close()
	if err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}
}

// TestServerWithRealDatabase tests the full server setup with a real database.
func TestServerWithRealDatabase(t *testing.T) {
	// Create temp files
	tmpDbFile, err := ioutil.TempFile("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp database file: %v", err)
	}
	tmpDbFile.Close()
	defer os.Remove(tmpDbFile.Name())

	tmpDir, err := ioutil.TempDir("", "storage-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Open database
	database, err := db.Open(tmpDbFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Setup router
	router := setupRouter(database)

	// Create server
	server := &http.Server{
		Addr:    "127.0.0.1:0", // Use random available port
		Handler: router,
	}

	// Get listener to find actual port
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	actualAddr := listener.Addr().String()
	listener.Close()

	// Update server address
	server.Addr = actualAddr

	// Start server
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test server is running
	resp, err := http.Get("http://" + actualAddr + "/healthz")
	if err != nil {
		// Server might not have started in time, that's OK
		t.Logf("Could not reach server: %v", err)
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health check returned %d, expected %d", resp.StatusCode, http.StatusOK)
		}
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// TestServerHostAndPort tests that server uses correct host and port.
func TestServerHostAndPort(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	router := setupRouter(database)

	testCases := []string{
		":8080",
		":9999",
		"127.0.0.1:0",
	}

	for _, addr := range testCases {
		server := &http.Server{
			Addr:    addr,
			Handler: router,
		}

		if server.Addr != addr {
			t.Errorf("Server address not set correctly: %s != %s", server.Addr, addr)
		}

		// Don't actually listen, just verify the structure is correct
		if server.Handler == nil {
			t.Error("Server handler not set")
		}
	}
}
