package main

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/bgguna/photography/internal/admin"
	"github.com/bgguna/photography/internal/auth"
	"github.com/bgguna/photography/photo"
)

type adminDeps struct {
	tmpl         *template.Template
	authSvc      *auth.AuthService
	adminSvc     *admin.AdminService
	photoSvc     *photo.PhotoService
	loginLimiter *auth.RateLimiter
	csrfKey      []byte
	isSecure     bool
}

// loadTemplates parses the public and admin templates into one set.
func loadTemplates() (*template.Template, error) {
	tmpl, err := template.ParseGlob("./internal/web/templates/*.html")
	if err != nil {
		return nil, err
	}
	return tmpl.ParseGlob("./internal/web/templates/admin/*.html")
}

// render executes a named template into a buffer first so a template error
// produces a clean 500 instead of a half-written page.
func render(c *gin.Context, tmpl *template.Template, status int, name string, data any) {
	if tmpl == nil {
		c.String(http.StatusInternalServerError, "Templates not loaded")
		return
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Error().Err(err).Str("template", name).Msg("Failed to execute template")
		c.String(http.StatusInternalServerError, "Failed to render page")
		return
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

func pathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return 0, false
	}
	return id, true
}

func registerAdminRoutes(router *gin.Engine, d adminDeps) {
	adminGroup := router.Group("/admin")

	adminGroup.GET("/login", func(c *gin.Context) {
		render(c, d.tmpl, http.StatusOK, "admin_login", gin.H{"Error": c.Query("error")})
	})

	adminGroup.POST("/login", func(c *gin.Context) {
		if !d.loginLimiter.Allow(c.ClientIP(), 5, 15*time.Minute) {
			c.Redirect(http.StatusFound, "/admin/login?error=too+many+login+attempts")
			return
		}

		email := c.PostForm("email")
		password := c.PostForm("password")
		if email == "" || password == "" {
			c.Redirect(http.StatusFound, "/admin/login?error=email+and+password+required")
			return
		}

		session, err := d.authSvc.Login(email, password, 7*24*time.Hour)
		if err != nil {
			c.Redirect(http.StatusFound, "/admin/login?error=invalid+credentials")
			return
		}

		auth.SetSessionCookie(c, session.ID, d.isSecure)
		c.Redirect(http.StatusFound, "/admin")
	})

	protected := adminGroup.Group("")
	protected.Use(auth.AuthMiddleware(d.authSvc), auth.CSRFMiddleware(d.csrfKey))

	page := func(c *gin.Context, name string, data gin.H) {
		data["Page"] = name
		data["CSRFToken"] = auth.CSRFTokenFromContext(c)
		render(c, d.tmpl, http.StatusOK, "admin_layout", data)
	}

	protected.POST("/logout", func(c *gin.Context) {
		if sessionID, err := c.Cookie("photography_session"); err == nil {
			_ = d.authSvc.Logout(sessionID)
		}
		auth.ClearSessionCookie(c)
		c.Redirect(http.StatusFound, "/admin/login")
	})

	protected.GET("", func(c *gin.Context) {
		photoCount, err := d.adminSvc.GetPhotoCount()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get photo count")
		}
		unread, err := d.adminSvc.GetUnreadMessageCount()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get unread message count")
		}
		page(c, "dashboard", gin.H{"PhotoCount": photoCount, "UnreadMessages": unread})
	})

	// renderPhotoGrid returns the full photo grid fragment (used after upload/delete/move).
	renderPhotoGrid := func(c *gin.Context) {
		photos, err := d.adminSvc.GetAllPhotos()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get photos")
			c.String(http.StatusInternalServerError, "Failed to load photos")
			return
		}
		render(c, d.tmpl, http.StatusOK, "admin_photo_grid", photos)
	}

	protected.GET("/photos", func(c *gin.Context) {
		photos, err := d.adminSvc.GetAllPhotos()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get photos")
		}
		page(c, "photos", gin.H{"Photos": photos})
	})

	protected.POST("/photos", func(c *gin.Context) {
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.String(http.StatusBadRequest, "No file provided")
			return
		}
		defer file.Close()

		user := auth.GetUserFromContext(c)
		if _, err := d.photoSvc.UploadPhoto(file, header, user.ID); err != nil {
			log.Error().Err(err).Msg("Failed to upload photo")
			c.String(http.StatusUnprocessableEntity, "Upload failed: "+err.Error())
			return
		}
		renderPhotoGrid(c)
	})

	adminImage := func(size string) gin.HandlerFunc {
		return func(c *gin.Context) {
			id, ok := pathID(c)
			if !ok {
				return
			}
			path, err := d.photoSvc.GetAdminPhotoFile(id, size)
			if err != nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			c.Header("Cache-Control", "private, no-store")
			c.File(path)
		}
	}
	protected.GET("/photos/:id/thumb", adminImage("thumb"))
	protected.GET("/photos/:id/web", adminImage("web"))
	protected.GET("/photos/:id/original", adminImage("original"))

	protected.DELETE("/photos/:id", func(c *gin.Context) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		if err := d.photoSvc.DeletePhoto(id); err != nil {
			log.Error().Err(err).Msg("Failed to delete photo")
			c.String(http.StatusInternalServerError, "Failed to delete photo")
			return
		}
		renderPhotoGrid(c)
	})

	protected.POST("/photos/:id/visibility", func(c *gin.Context) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		p, err := d.photoSvc.GetPhoto(id)
		if err != nil {
			c.String(http.StatusNotFound, "Photo not found")
			return
		}
		if err := d.photoSvc.SetPhotoVisibility(id, p.IsPublic == 0); err != nil {
			log.Error().Err(err).Msg("Failed to set photo visibility")
			c.String(http.StatusInternalServerError, "Failed to update photo")
			return
		}
		p, err = d.photoSvc.GetPhoto(id)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to reload photo")
			return
		}
		render(c, d.tmpl, http.StatusOK, "admin_photo_item", p)
	})

	protected.POST("/photos/:id/move", func(c *gin.Context) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		if err := d.photoSvc.MovePhoto(id, c.PostForm("direction")); err != nil {
			log.Error().Err(err).Msg("Failed to move photo")
			c.String(http.StatusBadRequest, "Could not move photo")
			return
		}
		renderPhotoGrid(c)
	})

	protected.GET("/messages", func(c *gin.Context) {
		messages, err := d.adminSvc.GetMessages()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get messages")
		}
		unread, _ := d.adminSvc.GetUnreadMessageCount()
		total, _ := d.adminSvc.GetMessageCount()
		page(c, "messages", gin.H{"Messages": messages, "UnreadCount": unread, "TotalCount": total})
	})

	messageUpdate := func(update func(int) error) gin.HandlerFunc {
		return func(c *gin.Context) {
			id, ok := pathID(c)
			if !ok {
				return
			}
			if err := update(id); err != nil {
				log.Error().Err(err).Msg("Failed to update message")
				c.String(http.StatusInternalServerError, "Failed to update message")
				return
			}
			msg, err := d.adminSvc.GetMessage(id)
			if err != nil {
				c.String(http.StatusNotFound, "Message not found")
				return
			}
			render(c, d.tmpl, http.StatusOK, "admin_message_item", msg)
		}
	}
	protected.POST("/messages/:id/read", messageUpdate(d.adminSvc.MarkMessageAsRead))
	protected.POST("/messages/:id/archive", messageUpdate(d.adminSvc.MarkMessageAsArchived))

	protected.DELETE("/messages/:id", func(c *gin.Context) {
		id, ok := pathID(c)
		if !ok {
			return
		}
		if err := d.adminSvc.DeleteMessage(id); err != nil {
			log.Error().Err(err).Msg("Failed to delete message")
			c.String(http.StatusInternalServerError, "Failed to delete message")
			return
		}
		c.Status(http.StatusOK)
	})
}
