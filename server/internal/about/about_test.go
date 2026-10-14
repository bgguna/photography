package about

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/bgguna/photography/internal/db"
)

func openTestDB(t *testing.T) *Service {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Each new in-memory connection is a separate database, so pin the pool to one.
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { database.Close() })

	schema, err := os.ReadFile("../../../db/schema.sql")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := database.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return NewService(database)
}

func TestGet_EmptyWhenNothingSaved(t *testing.T) {
	svc := openTestDB(t)

	body, err := svc.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if body != "" {
		t.Errorf("Get() = %q, want empty", body)
	}
}

func TestSave_RoundTripAndOverwrite(t *testing.T) {
	svc := openTestDB(t)

	if err := svc.Save("  First draft  "); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got, _ := svc.Get(); got != "First draft" {
		t.Errorf("after first save Get() = %q, want %q", got, "First draft")
	}

	if err := svc.Save("Second draft"); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	if got, _ := svc.Get(); got != "Second draft" {
		t.Errorf("after overwrite Get() = %q, want %q", got, "Second draft")
	}
}

func TestSave_RejectsTooLong(t *testing.T) {
	svc := openTestDB(t)

	err := svc.Save(strings.Repeat("a", MaxLength+1))
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("Save() error = %v, want ErrTooLong", err)
	}
	if got, _ := svc.Get(); got != "" {
		t.Errorf("rejected save still stored %d characters", len(got))
	}

	if err := svc.Save(strings.Repeat("a", MaxLength)); err != nil {
		t.Errorf("Save() at exactly MaxLength error = %v", err)
	}
}

func TestParagraphs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", " \n\n \t \n", nil},
		{"single paragraph keeps line breaks", "Line one\nLine two", []string{"Line one\nLine two"}},
		{"blank line splits", "One\n\nTwo", []string{"One", "Two"}},
		{"blank line with spaces splits", "One\n  \t\nTwo", []string{"One", "Two"}},
		{"windows line endings", "One\r\n\r\nTwo", []string{"One", "Two"}},
		{"extra blank lines dropped", "One\n\n\n\nTwo\n\n", []string{"One", "Two"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Paragraphs(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Paragraphs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
