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

	// Parse templates
	var tmpl *template.Template
	var err error
	tmpl = template.New("")
	tmpl, err = tmpl.ParseGlob("./internal/web/templates/*.html")
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse templates")
	}
	tmpl, err = tmpl.ParseGlob("./internal/web/templates/admin/*.html")
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse admin templates")
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
			data := gin.H{"Error": c.Query("error")}
			if tmpl != nil {
				c.Header("Content-Type", "text/html; charset=utf-8")
				if err := tmpl.ExecuteTemplate(c.Writer, "admin_login", data); err != nil {
					log.Error().Err(err).Msg("Failed to execute login template")
					c.String(http.StatusInternalServerError, "Failed to render page")
				}
			} else {
				c.String(http.StatusInternalServerError, "Templates not loaded")
			}
		})

		admin.POST("/login", func(c *gin.Context) {
			// Rate limiting
			clientIP := c.ClientIP()
			if !loginLimiter.Allow(clientIP, 5, 15*time.Minute) {
				c.Redirect(http.StatusFound, "/admin/login?error=too+many+login+attempts")
				return
			}

			// Get email and password from form
			email := c.PostForm("email")
			password := c.PostForm("password")

			if email == "" || password == "" {
				c.Redirect(http.StatusFound, "/admin/login?error=email+and+password+required")
				return
			}

			// Attempt login
			session, err := authSvc.Login(email, password, 7*24*time.Hour)
			if err != nil {
				c.Redirect(http.StatusFound, "/admin/login?error=invalid+credentials")
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
				photoCount, err := adminSvc.GetPhotoCount()
				if err != nil {
					log.Error().Err(err).Msg("Failed to get photo count")
					photoCount = 0
				}

				unreadCount, err := adminSvc.GetUnreadMessageCount()
				if err != nil {
					log.Error().Err(err).Msg("Failed to get unread message count")
					unreadCount = 0
				}

				data := gin.H{
					"Page":              "dashboard",
					"PhotoCount":        photoCount,
					"UnreadMessages":    unreadCount,
				}

				if tmpl != nil {
					c.Header("Content-Type", "text/html; charset=utf-8")
					if err := tmpl.ExecuteTemplate(c.Writer, "admin_dashboard", data); err != nil {
						log.Error().Err(err).Msg("Failed to execute dashboard template")
						c.String(http.StatusInternalServerError, "Failed to render page")
					}
				} else {
					c.String(http.StatusInternalServerError, "Templates not loaded")
				}
			})

			protected.GET("/photos", func(c *gin.Context) {
				photos, err := adminSvc.GetAllPhotos()
				if err != nil {
					log.Error().Err(err).Msg("Failed to get photos")
					photos = nil
				}

				data := gin.H{
					"Page":   "photos",
					"Photos": photos,
				}

				if tmpl != nil {
					c.Header("Content-Type", "text/html; charset=utf-8")
					if err := tmpl.ExecuteTemplate(c.Writer, "admin_photos", data); err != nil {
						log.Error().Err(err).Msg("Failed to execute photos template")
						c.String(http.StatusInternalServerError, "Failed to render page")
					}
				} else {
					c.String(http.StatusInternalServerError, "Templates not loaded")
				}
			})

			protected.POST("/photos", func(c *gin.Context) {
				file, header, err := c.Request.FormFile("file")
				if err != nil {
					c.String(http.StatusBadRequest, "No file provided")
					return
				}
				defer file.Close()

				user := auth.GetUserFromContext(c)
				p, err := photoSvc.UploadPhoto(file, header, user.ID)
				if err != nil {
					log.Error().Err(err).Msg("Failed to upload photo")
					c.String(http.StatusInternalServerError, "Failed to upload photo: "+err.Error())
					return
				}

				// Return photo grid item as HTML fragment for htmx swap
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.String(http.StatusOK, fmt.Sprintf(`<div id="photo-%d" class="photo-item" hx-swap="outerHTML">
					<img src="/admin/photos/%d/thumb" alt="Photo %d" loading="lazy">
					<div class="photo-actions">
						<button class="btn-success" hx-post="/admin/photos/%d/visibility" hx-swap="outerHTML">Publish</button>
						<button class="btn-danger" hx-delete="/admin/photos/%d" hx-confirm="Delete this photo?">Delete</button>
					</div>
				</div>`, p.ID, p.ID, p.ID, p.ID, p.ID))
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

			protected.DELETE("/photos/:id", func(c *gin.Context) {
				photoID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				if err := photoSvc.DeletePhoto(photoID); err != nil {
					log.Error().Err(err).Msg("Failed to delete photo")
					c.String(http.StatusInternalServerError, "Failed to delete photo")
					return
				}

				// Return empty response (htmx will remove the element)
				c.String(http.StatusOK, "")
			})

			protected.POST("/photos/:id/visibility", func(c *gin.Context) {
				photoID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				p, err := photoSvc.GetPhoto(photoID)
				if err != nil {
					log.Error().Err(err).Msg("Failed to get photo")
					c.String(http.StatusNotFound, "Photo not found")
					return
				}

				isPublic := p.IsPublic == 0
				if err := photoSvc.SetPhotoVisibility(photoID, isPublic); err != nil {
					log.Error().Err(err).Msg("Failed to set photo visibility")
					c.String(http.StatusInternalServerError, "Failed to update photo")
					return
				}

				// Return updated photo item as HTML fragment
				btnClass := "success"
				btnText := "Publish"
				hiddenClass := ""
				if isPublic {
					btnClass = "danger"
					btnText = "Hide"
				} else {
					hiddenClass = " hidden"
				}

				c.Header("Content-Type", "text/html; charset=utf-8")
				c.String(http.StatusOK, fmt.Sprintf(`<div id="photo-%d" class="photo-item%s">
					<img src="/admin/photos/%d/thumb" alt="Photo %d" loading="lazy">
					<div class="photo-actions">
						<button class="btn-%s" hx-post="/admin/photos/%d/visibility" hx-swap="outerHTML">%s</button>
						<button class="btn-danger" hx-delete="/admin/photos/%d" hx-confirm="Delete this photo?">Delete</button>
					</div>
				</div>`, photoID, hiddenClass, photoID, photoID, btnClass, photoID, btnText, photoID))
			})

			protected.GET("/messages", func(c *gin.Context) {
				messages, err := adminSvc.GetMessages()
				if err != nil {
					log.Error().Err(err).Msg("Failed to get messages")
					messages = nil
				}

				unreadCount, err := adminSvc.GetUnreadMessageCount()
				if err != nil {
					log.Error().Err(err).Msg("Failed to get unread count")
					unreadCount = 0
				}

				totalCount, err := adminSvc.GetMessageCount()
				if err != nil {
					log.Error().Err(err).Msg("Failed to get total count")
					totalCount = 0
				}

				data := gin.H{
					"Page":         "messages",
					"Messages":     messages,
					"UnreadCount":  unreadCount,
					"TotalCount":   totalCount,
				}

				if tmpl != nil {
					c.Header("Content-Type", "text/html; charset=utf-8")
					if err := tmpl.ExecuteTemplate(c.Writer, "admin_messages", data); err != nil {
						log.Error().Err(err).Msg("Failed to execute messages template")
						c.String(http.StatusInternalServerError, "Failed to render page")
					}
				} else {
					c.String(http.StatusInternalServerError, "Templates not loaded")
				}
			})

			protected.POST("/messages/:id/read", func(c *gin.Context) {
				messageID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				if err := adminSvc.MarkMessageAsRead(messageID); err != nil {
					log.Error().Err(err).Msg("Failed to mark message as read")
					c.String(http.StatusInternalServerError, "Failed to update message")
					return
				}

				c.String(http.StatusOK, "")
			})

			protected.POST("/messages/:id/archive", func(c *gin.Context) {
				messageID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				if err := adminSvc.MarkMessageAsArchived(messageID); err != nil {
					log.Error().Err(err).Msg("Failed to archive message")
					c.String(http.StatusInternalServerError, "Failed to archive message")
					return
				}

				c.String(http.StatusOK, "")
			})

			protected.DELETE("/messages/:id", func(c *gin.Context) {
				messageID, err := strconv.Atoi(c.Param("id"))
				if err != nil {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}

				if err := adminSvc.DeleteMessage(messageID); err != nil {
					log.Error().Err(err).Msg("Failed to delete message")
					c.String(http.StatusInternalServerError, "Failed to delete message")
					return
				}

				c.String(http.StatusOK, "")
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
