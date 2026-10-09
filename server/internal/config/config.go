package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port             string
	DBPath           string
	PhotoStoragePath string
	SessionSecret    string
	Mode             string
}

// Load reads environment variables and returns a Config struct.
// It enforces required fields based on the environment mode.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		DBPath:           getEnv("DB_PATH", "../db/gallery.sqlite"),
		PhotoStoragePath: os.Getenv("PHOTO_STORAGE_PATH"),
		SessionSecret:    os.Getenv("SESSION_SECRET"),
		Mode:             getEnv("MODE_ENV", "development"),
	}

	// Validate required fields
	if cfg.PhotoStoragePath == "" {
		return nil, fmt.Errorf("PHOTO_STORAGE_PATH environment variable is required")
	}

	if cfg.Mode == "production" && cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET environment variable is required in production mode")
	}

	return cfg, nil
}

// getEnv returns the environment variable or a default value if not set.
func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
