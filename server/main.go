package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

	// Parse templates
	tmpl, err := template.ParseGlob("./internal/web/templates/*.html")
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse templates")
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

		photosJSON, _ := json.Marshal(photos)
		data := gin.H{
			"Photos":      photos,
			"PhotosJSON":  string(photosJSON),
		}

		if tmpl != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.ExecuteTemplate(c.Writer, "layout.html", data); err != nil {
				log.Error().Err(err).Msg("Failed to execute template")
				c.String(http.StatusInternalServerError, "Failed to render page")
			}
		} else {
			c.String(http.StatusInternalServerError, "Templates not loaded")
		}
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

	router.GET("/contact", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: contact form")
	})

	router.POST("/contact", func(c *gin.Context) {
		c.String(http.StatusNotImplemented, "TODO: submit contact form")
	})

	// Admin routes
	admin := router.Group("/admin")
	{
		// Login routes (no auth middleware)
		admin.GET("/login", func(c *gin.Context) {
			c.String(http.StatusNotImplemented, "TODO: admin login page")
		})

		admin.POST("/login", func(c *gin.Context) {
			// Rate limiting
			clientIP := c.ClientIP()
			if !loginLimiter.Allow(clientIP, 5, 15*time.Minute) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "too many login attempts",
				})
				return
			}

			// Get email and password from form
			email := c.PostForm("email")
			password := c.PostForm("password")

			if email == "" || password == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "email and password required",
				})
				return
			}

			// Attempt login
			session, err := authSvc.Login(email, password, 7*24*time.Hour)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "invalid credentials",
				})
				return
			}

			// Set session cookie
			auth.SetSessionCookie(c, session.ID, isSecure)
			c.Redirect(http.StatusFound, "/admin")
		})

		// Protected routes
		protected := admin.Group("")
		protected.Use(auth.AuthMiddleware(authSvc))
		{
			protected.POST("/logout", func(c *gin.Context) {
				// Get session cookie
				sessionID, err := c.Cookie("photography_session")
				if err == nil {
					_ = authSvc.Logout(sessionID)
				}
				auth.ClearSessionCookie(c)
				c.Redirect(http.StatusFound, "/admin/login")
			})

			protected.GET("", func(c *gin.Context) {
				user := auth.GetUserFromContext(c)
				c.JSON(http.StatusOK, gin.H{
					"message": "admin dashboard",
					"user":    user.Email,
				})
			})

			protected.GET("/photos", func(c *gin.Context) {
				c.String(http.StatusNotImplemented, "TODO: admin photos list")
			})

			protected.POST("/photos", func(c *gin.Context) {
				c.String(http.StatusNotImplemented, "TODO: admin upload photo")
			})

			protected.GET("/photos/:id/thumb", func(c *gin.Context) {
				photoID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				path, err := photoSvc.GetAdminPhotoFile(photoID, "thumb")
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				c.Header("Cache-Control", "private, no-store")
				c.File(path)
			})

			protected.GET("/photos/:id/web", func(c *gin.Context) {
				photoID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				path, err := photoSvc.GetAdminPhotoFile(photoID, "web")
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				c.Header("Cache-Control", "private, no-store")
				c.File(path)
			})

			protected.GET("/photos/:id/original", func(c *gin.Context) {
				photoID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				path, err := photoSvc.GetAdminPhotoFile(photoID, "original")
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				c.Header("Cache-Control", "private, no-store")
				c.File(path)
			})

			protected.POST("/photos/:id/delete", func(c *gin.Context) {
				c.String(http.StatusNotImplemented, "TODO: admin delete photo")
			})

			protected.POST("/photos/:id/visibility", func(c *gin.Context) {
				c.String(http.StatusNotImplemented, "TODO: admin toggle photo visibility")
			})

			protected.GET("/messages", func(c *gin.Context) {
				c.String(http.StatusNotImplemented, "TODO: admin messages")
			})
		}
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
