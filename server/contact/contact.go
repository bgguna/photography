package contact

import (
	"database/sql"
	"net/http"

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
