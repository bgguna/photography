package config

import (
	"os"
	"testing"
)

func TestLoad_WithValidConfig(t *testing.T) {
	// Setup
	os.Setenv("PORT", "9090")
	os.Setenv("DB_PATH", "/tmp/test.db")
	os.Setenv("PHOTO_STORAGE_PATH", "/mnt/photos")
	os.Setenv("MODE_ENV", "development")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("PHOTO_STORAGE_PATH")
		os.Unsetenv("MODE_ENV")
	}()

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() should not return error, got: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %v, want %v", cfg.Port, "9090")
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %v, want %v", cfg.DBPath, "/tmp/test.db")
	}
	if cfg.PhotoStoragePath != "/mnt/photos" {
		t.Errorf("PhotoStoragePath = %v, want %v", cfg.PhotoStoragePath, "/mnt/photos")
	}
	if cfg.Mode != "development" {
		t.Errorf("Mode = %v, want %v", cfg.Mode, "development")
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	// Setup - clear env vars
	os.Unsetenv("PORT")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("MODE_ENV")
	os.Setenv("PHOTO_STORAGE_PATH", "/mnt/photos")
	defer os.Unsetenv("PHOTO_STORAGE_PATH")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() should not return error, got: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %v, want %v (default)", cfg.Port, "8080")
	}
	if cfg.DBPath != "../db/gallery.sqlite" {
		t.Errorf("DBPath = %v, want %v (default)", cfg.DBPath, "../db/gallery.sqlite")
	}
	if cfg.Mode != "development" {
		t.Errorf("Mode = %v, want %v (default)", cfg.Mode, "development")
	}
}

func TestLoad_MissingPhotoStoragePath(t *testing.T) {
	// Setup
	os.Unsetenv("PHOTO_STORAGE_PATH")
	os.Setenv("MODE_ENV", "development")
	defer os.Unsetenv("MODE_ENV")

	_, err := Load()

	if err == nil {
		t.Fatal("Load() should return error when PHOTO_STORAGE_PATH is missing")
	}
	if err.Error() != "PHOTO_STORAGE_PATH environment variable is required" {
		t.Errorf("Expected PHOTO_STORAGE_PATH error, got: %v", err)
	}
}

func TestLoad_ProductionMissingSessionSecret(t *testing.T) {
	// Setup
	os.Setenv("PHOTO_STORAGE_PATH", "/mnt/photos")
	os.Setenv("MODE_ENV", "production")
	os.Unsetenv("SESSION_SECRET")
	defer func() {
		os.Unsetenv("PHOTO_STORAGE_PATH")
		os.Unsetenv("MODE_ENV")
	}()

	_, err := Load()

	if err == nil {
		t.Fatal("Load() should return error when SESSION_SECRET is missing in production")
	}
	if err.Error() != "SESSION_SECRET environment variable is required in production mode" {
		t.Errorf("Expected SESSION_SECRET error, got: %v", err)
	}
}

func TestLoad_ProductionWithSessionSecret(t *testing.T) {
	// Setup
	os.Setenv("PHOTO_STORAGE_PATH", "/mnt/photos")
	os.Setenv("MODE_ENV", "production")
	os.Setenv("SESSION_SECRET", "my-secret-key")
	defer func() {
		os.Unsetenv("PHOTO_STORAGE_PATH")
		os.Unsetenv("MODE_ENV")
		os.Unsetenv("SESSION_SECRET")
	}()

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() should not return error, got: %v", err)
	}
	if cfg.SessionSecret != "my-secret-key" {
		t.Errorf("SessionSecret = %v, want %v", cfg.SessionSecret, "my-secret-key")
	}
}

func TestGetEnv_WithExistingVar(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnv("TEST_VAR", "default")

	if result != "test-value" {
		t.Errorf("getEnv() = %v, want %v", result, "test-value")
	}
}

func TestGetEnv_WithMissingVar(t *testing.T) {
	os.Unsetenv("NONEXISTENT_VAR")

	result := getEnv("NONEXISTENT_VAR", "default-value")

	if result != "default-value" {
		t.Errorf("getEnv() = %v, want %v", result, "default-value")
	}
}
