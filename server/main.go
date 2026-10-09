package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bgguna/photography/contact"
	"github.com/bgguna/photography/internal/admin"
	"github.com/bgguna/photography/internal/auth"
	"github.com/bgguna/photography/internal/config"
	"github.com/bgguna/photography/internal/db"
	"github.com/bgguna/photography/internal/gallery"
	"github.com/bgguna/photography/photo"
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

	// Create auth service and rate limiter
	authSvc := auth.NewAuthService(database)
	loginLimiter := auth.NewRateLimiter()
	isSecure := os.Getenv("MODE_ENV") == "production"

	// Create photo and gallery services
	photoSvc := photo.NewPhotoService(database, os.Getenv("PHOTO_STORAGE_PATH"))
	gal := gallery.NewGallery(photoSvc)
	adminSvc := admin.NewAdminService(database, photoSvc)

	tmpl, err := loadTemplates()
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse templates")
	}

	csrfKey := []byte(os.Getenv("SESSION_SECRET"))
	if len(csrfKey) == 0 {
		csrfKey = auth.NewCSRFKey()
	}

	// Static files
	router.Static("/static", "./internal/web/static")

	// Health check
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Public routes
	router.GET("/", func(c *gin.Context) {
		photos, err := gal.GetPublicPhotos()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get public photos")
			c.String(http.StatusInternalServerError, "Error loading photos")
			return
		}

		render(c, tmpl, http.StatusOK, "layout.html", gin.H{"Photos": photos})
	})

	router.GET("/photos/:id/thumb", func(c *gin.Context) {
		photoID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		path, err := photoSvc.GetPhotoFile(photoID, "thumb")
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.Header("Cache-Control", "public, max-age=300")
		c.File(path)
	})

	router.GET("/photos/:id/web", func(c *gin.Context) {
		photoID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		path, err := photoSvc.GetPhotoFile(photoID, "web")
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.Header("Cache-Control", "public, max-age=300")
		c.File(path)
	})

	// Create rate limiter for contact form
	contactLimiter := auth.NewRateLimiter()

	router.GET("/contact", func(c *gin.Context) {
		render(c, tmpl, http.StatusOK, "contact_page", gin.H{})
	})

	router.POST("/contact", func(c *gin.Context) {
		// Rate limiting: 5 submissions per IP per hour
		clientIP := c.ClientIP()
		if !contactLimiter.Allow(clientIP, 5, 60*time.Minute) {
			c.String(http.StatusTooManyRequests, "Too many submission attempts. Please try again later.")
			return
		}

		// Call the form handler
		contact.HandleNewMsgForm(database)(c)
	})

	registerAdminRoutes(router, adminDeps{
		tmpl:         tmpl,
		authSvc:      authSvc,
		adminSvc:     adminSvc,
		photoSvc:     photoSvc,
		loginLimiter: loginLimiter,
		csrfKey:      csrfKey,
		isSecure:     isSecure,
	})

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
