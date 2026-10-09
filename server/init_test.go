package main

import (
	"os"
	"testing"
)

// TestGetEnvironment_WithModeSet tests getEnvironment when MODE_ENV is set.
func TestGetEnvironment_WithModeSet(t *testing.T) {
	os.Setenv("MODE_ENV", "production")
	defer os.Unsetenv("MODE_ENV")

	env := getEnvironment()
	if env != "production" {
		t.Errorf("getEnvironment() = %s, want production", env)
	}
}

// TestGetEnvironment_WithModeNotSet tests getEnvironment when MODE_ENV is not set.
func TestGetEnvironment_WithModeNotSet(t *testing.T) {
	os.Unsetenv("MODE_ENV")

	env := getEnvironment()
	if env != "development" {
		t.Errorf("getEnvironment() = %s, want development (default)", env)
	}
}

// TestGetEnvironment_WithEmptyMode tests getEnvironment when MODE_ENV is empty string.
func TestGetEnvironment_WithEmptyMode(t *testing.T) {
	os.Setenv("MODE_ENV", "")
	defer os.Unsetenv("MODE_ENV")

	env := getEnvironment()
	if env != "development" {
		t.Errorf("getEnvironment() = %s, want development (default)", env)
	}
}

// TestGetEnvironment_WithLocalMode tests getEnvironment with local mode.
func TestGetEnvironment_WithLocalMode(t *testing.T) {
	os.Setenv("MODE_ENV", "local")
	defer os.Unsetenv("MODE_ENV")

	env := getEnvironment()
	if env != "local" {
		t.Errorf("getEnvironment() = %s, want local", env)
	}
}

// TestConfigureLogging_DevelopmentMode tests configureLogging for development mode.
func TestConfigureLogging_DevelopmentMode(t *testing.T) {
	// This test just ensures the function doesn't panic
	configureLogging("development")
	// Logging is configured, we can't easily verify the output, but we test for panics
}

// TestConfigureLogging_LocalMode tests configureLogging for local mode.
func TestConfigureLogging_LocalMode(t *testing.T) {
	// This test just ensures the function doesn't panic
	configureLogging("local")
}

// TestConfigureLogging_ProductionMode tests configureLogging for production mode.
func TestConfigureLogging_ProductionMode(t *testing.T) {
	// This test just ensures the function doesn't panic
	configureLogging("production")
}
