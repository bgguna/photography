package contact

import (
	"database/sql"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ContactMsg is an incoming contact message/request.
type ContactMsg struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at,omitempty"`
	Status    string `json:"status,omitempty"`
}

// GetMessages returns a handler that lists all stored contact messages.
func GetMessages(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Query(
			"SELECT id, name, email, message, created_at, status FROM contact_messages ORDER BY created_at DESC",
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch contact messages")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "fail"})
			return
		}
		defer rows.Close()

		messages := []ContactMsg{}
		for rows.Next() {
			var msg ContactMsg
			if err := rows.Scan(&msg.ID, &msg.Name, &msg.Email, &msg.Message, &msg.CreatedAt, &msg.Status); err != nil {
				log.Error().Err(err).Msg("failed to scan contact message row")
				c.JSON(http.StatusInternalServerError, gin.H{"status": "fail"})
				return
			}
			messages = append(messages, msg)
		}

		log.Info().Int("count", len(messages)).Msg("fetched contact messages")
		c.JSON(http.StatusOK, messages)
	}
}

// HandleNewMsg returns a handler that saves an incoming contact message to the database.
func HandleNewMsg(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var msg ContactMsg
		if err := c.ShouldBindJSON(&msg); err != nil {
			log.Error().Err(err).Msg("failed to bind contact message")
			c.JSON(http.StatusBadRequest, gin.H{"status": "fail"})
			return
		}

		log.Info().Str("name", msg.Name).Msg("received contact message")
		_, err := db.Exec(
			"INSERT INTO contact_messages (name, email, message) VALUES (?, ?, ?)",
			msg.Name, msg.Email, msg.Message,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to store contact message")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "fail"})
			return
		}

		log.Info().Msg("contact message stored")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// HandleNewMsgForm returns a handler that saves a form-submitted contact message to the database.
// It includes honeypot and validation checks.
func HandleNewMsgForm(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check honeypot field
		website := c.PostForm("website")
		if website != "" {
			// Bot filled the honeypot field - silently accept but don't store
			log.Warn().Msg("honeypot field filled - ignoring submission")
			c.String(http.StatusOK, "Thank you for your message!")
			return
		}

		// Get form fields
		name := strings.TrimSpace(c.PostForm("name"))
		email := strings.TrimSpace(c.PostForm("email"))
		message := strings.TrimSpace(c.PostForm("message"))

		// Validate required fields
		if name == "" {
			c.String(http.StatusBadRequest, "Name is required")
			return
		}
		if message == "" {
			c.String(http.StatusBadRequest, "Message is required")
			return
		}

		// Validate email if provided
		if email != "" {
			if !isValidEmail(email) {
				c.String(http.StatusBadRequest, "Invalid email address")
				return
			}
		}

		// Store message
		log.Info().Str("name", name).Msg("received contact message from form")
		_, err := db.Exec(
			"INSERT INTO contact_messages (name, email, message) VALUES (?, ?, ?)",
			name, email, message,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to store contact message")
			c.String(http.StatusInternalServerError, "Failed to send message. Please try again later.")
			return
		}

		log.Info().Msg("contact message stored")
		c.String(http.StatusOK, "Thank you for your message!")
	}
}

// isValidEmail performs a basic email validation.
func isValidEmail(email string) bool {
	// Basic email regex pattern
	pattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}
