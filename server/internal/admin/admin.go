package admin

import (
	"database/sql"
	"fmt"

	"github.com/bgguna/photography/photo"
)

// AdminService handles admin operations.
type AdminService struct {
	db       *sql.DB
	photoSvc *photo.PhotoService
}

// NewAdminService creates a new AdminService.
func NewAdminService(db *sql.DB, photoSvc *photo.PhotoService) *AdminService {
	return &AdminService{
		db:       db,
		photoSvc: photoSvc,
	}
}

// GetAllPhotos returns all photos (public and hidden).
func (as *AdminService) GetAllPhotos() ([]photo.Photo, error) {
	return as.photoSvc.ListAllPhotos()
}

// GetPhotoCount returns the total number of photos.
func (as *AdminService) GetPhotoCount() (int, error) {
	var count int
	err := as.db.QueryRow("SELECT COUNT(*) FROM photos").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get photo count: %w", err)
	}
	return count, nil
}

// GetMessages returns all contact messages.
func (as *AdminService) GetMessages() ([]ContactMessage, error) {
	rows, err := as.db.Query(`
		SELECT id, name, email, message, created_at, status
		FROM contact_messages
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []ContactMessage
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate messages: %w", err)
	}

	return messages, nil
}

// GetUnreadMessageCount returns the count of unread messages.
func (as *AdminService) GetUnreadMessageCount() (int, error) {
	var count int
	err := as.db.QueryRow("SELECT COUNT(*) FROM contact_messages WHERE status = 'new'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread message count: %w", err)
	}
	return count, nil
}

// GetMessageCount returns the total count of messages.
func (as *AdminService) GetMessageCount() (int, error) {
	var count int
	err := as.db.QueryRow("SELECT COUNT(*) FROM contact_messages").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get message count: %w", err)
	}
	return count, nil
}

// MarkMessageAsRead marks a message as read.
func (as *AdminService) MarkMessageAsRead(messageID int) error {
	_, err := as.db.Exec("UPDATE contact_messages SET status = 'read', read_at = datetime('now') WHERE id = ?", messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as read: %w", err)
	}
	return nil
}

// MarkMessageAsArchived marks a message as archived.
func (as *AdminService) MarkMessageAsArchived(messageID int) error {
	_, err := as.db.Exec("UPDATE contact_messages SET status = 'archived' WHERE id = ?", messageID)
	if err != nil {
		return fmt.Errorf("failed to archive message: %w", err)
	}
	return nil
}

// DeleteMessage deletes a message.
func (as *AdminService) DeleteMessage(messageID int) error {
	_, err := as.db.Exec("DELETE FROM contact_messages WHERE id = ?", messageID)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}

// ContactMessage represents a contact message.
type ContactMessage struct {
	ID        int
	Name      string
	Email     string
	Message   string
	CreatedAt string
	Status    string
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMessage(r rowScanner) (*ContactMessage, error) {
	var msg ContactMessage
	var email sql.NullString
	if err := r.Scan(&msg.ID, &msg.Name, &email, &msg.Message, &msg.CreatedAt, &msg.Status); err != nil {
		return nil, fmt.Errorf("failed to scan message: %w", err)
	}
	msg.Email = email.String
	return &msg, nil
}

// GetMessage returns a single contact message.
func (as *AdminService) GetMessage(messageID int) (*ContactMessage, error) {
	return scanMessage(as.db.QueryRow(
		"SELECT id, name, email, message, created_at, status FROM contact_messages WHERE id = ?",
		messageID,
	))
}
