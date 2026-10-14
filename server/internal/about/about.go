// Package about stores the editable text shown on the public About page.
package about

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxLength is the longest body accepted, in characters.
const MaxLength = 10000

// ErrTooLong is returned by Save when the body exceeds MaxLength.
var ErrTooLong = errors.New("about text is too long")

// blankLine matches a line break followed by only whitespace and another line break.
var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

// Service reads and writes the About page text.
type Service struct {
	db *sql.DB
}

// NewService creates a Service backed by db.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Get returns the stored body, or "" if nothing has been saved yet.
func (s *Service) Get() (string, error) {
	var body string
	err := s.db.QueryRow("SELECT body FROM about_page WHERE id = 1").Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read about page: %w", err)
	}
	return body, nil
}

// Save replaces the stored body. Surrounding whitespace is trimmed.
func (s *Service) Save(body string) error {
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) > MaxLength {
		return ErrTooLong
	}
	_, err := s.db.Exec(`
		INSERT INTO about_page (id, body, updated_at)
		VALUES (1, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		ON CONFLICT(id) DO UPDATE SET body = excluded.body, updated_at = excluded.updated_at
	`, body)
	if err != nil {
		return fmt.Errorf("failed to save about page: %w", err)
	}
	return nil
}

// Paragraphs splits body into paragraphs at blank lines. Single line breaks
// stay inside their paragraph; empty paragraphs are dropped.
func Paragraphs(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	var paragraphs []string
	for _, p := range blankLine.Split(body, -1) {
		if p = strings.TrimSpace(p); p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	return paragraphs
}
