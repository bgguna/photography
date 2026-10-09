package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bgguna/photography/internal/config"
	"github.com/bgguna/photography/internal/db"
)

func init() {
	// Load .env file
	_ = godotenv.Load("../.env")

	// Configure logging
	environment := getEnvironment()
	configureLogging(environment)

	log.Info().Msgf("Server application initialized in %s mode", environment)
}

// getEnvironment returns the MODE_ENV value or "development" if not set.
func getEnvironment() string {
	environment := os.Getenv("MODE_ENV")
	if environment == "" {
		environment = "development"
	}
	return environment
}

// configureLogging sets up zerolog based on the environment mode.
func configureLogging(environment string) {
	if environment == "local" || environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Verify photo storage path exists and is writable
	if err := verifyStoragePath(cfg.PhotoStoragePath); err != nil {
		log.Fatal().Err(err).Msg("Photo storage path validation failed")
	}

	// Open database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to open database at %s", cfg.DBPath)
	}
	defer database.Close()

	// Create router
	router := setupRouter(database)

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Info().Msgf("Server listening on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server listen error")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}

	log.Info().Msg("Server stopped")
}

// verifyStoragePath checks if the storage path exists and is writable.
func verifyStoragePath(path string) error {
	if path == "" {
		return fmt.Errorf("photo storage path is empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("photo storage path does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("photo storage path is not a directory")
	}

	// Test writeability
	testFile := fmt.Sprintf("%s/.write-test-%d", path, time.Now().UnixNano())
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("photo storage path is not writable: %w", err)
	}
	os.Remove(testFile)

	return nil
}

// setupRouter creates and configures the Gin router with all routes.
func setupRouter(database *sql.DB) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware())

	// Static files
	router.Static("/static", "./internal/web/static")

	// Health check
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Public routes
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: gallery home")
	})

	router.GET("/photos/:id/thumb", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: photo thumbnail")
	})

	router.GET("/photos/:id/web", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: photo web size")
	})

	router.GET("/contact", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: contact form")
	})

	router.POST("/contact", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: submit contact form")
	})

	// Admin routes (session-gated)
	admin := router.Group("/admin")
	{
		admin.GET("/login", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin login page")
		})

		admin.POST("/login", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin login handler")
		})

		admin.POST("/logout", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin logout")
		})

		admin.GET("", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin dashboard")
		})

		admin.GET("/photos", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin photos list")
		})

		admin.POST("/photos", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin upload photo")
		})

		admin.GET("/photos/:id/thumb", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin photo thumbnail")
		})

		admin.GET("/photos/:id/web", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin photo web size")
		})

		admin.GET("/photos/:id/original", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin photo original")
		})

		admin.POST("/photos/:id/delete", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin delete photo")
		})

		admin.POST("/photos/:id/visibility", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin toggle photo visibility")
		})

		admin.GET("/messages", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin messages")
		})
	}

	return router
}

// loggingMiddleware creates a zerolog-based request logging middleware.
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		log.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.RequestURI).
			Int("status", c.Writer.Status()).
			Dur("latency", duration).
			Msg("request")
	}
}
