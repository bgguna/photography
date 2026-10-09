package admin

import (
	"database/sql"
	"os"
	"testing"

	"github.com/bgguna/photography/internal/db"
	"github.com/bgguna/photography/photo"
)

func newTestService(t *testing.T) (*AdminService, *sql.DB) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	schema, err := os.ReadFile("../../../db/schema.sql")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := database.Exec(string(schema)); err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return NewAdminService(database, photo.NewPhotoService(database, t.TempDir())), database
}

func addMsg(t *testing.T, d *sql.DB, name, email string) int {
	t.Helper()
	var e any = email
	if email == "" {
		e = nil
	}
	res, err := d.Exec("INSERT INTO contact_messages (name, email, message) VALUES (?, ?, ?)", name, e, "hello")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, _ := res.LastInsertId()
	return int(id)
}

func TestCounts(t *testing.T) {
	svc, d := newTestService(t)

	if n, err := svc.GetPhotoCount(); err != nil || n != 0 {
		t.Fatalf("photo count = %d, %v", n, err)
	}
	if photos, err := svc.GetAllPhotos(); err != nil || len(photos) != 0 {
		t.Fatalf("photos = %v, %v", photos, err)
	}

	a := addMsg(t, d, "a", "a@example.com")
	addMsg(t, d, "b", "")
	if n, _ := svc.GetMessageCount(); n != 2 {
		t.Errorf("message count = %d", n)
	}
	if n, _ := svc.GetUnreadMessageCount(); n != 2 {
		t.Errorf("unread = %d", n)
	}
	if err := svc.MarkMessageAsRead(a); err != nil {
		t.Fatal(err)
	}
	if n, _ := svc.GetUnreadMessageCount(); n != 1 {
		t.Errorf("unread after read = %d", n)
	}
}

func TestMessageLifecycle(t *testing.T) {
	svc, d := newTestService(t)
	id := addMsg(t, d, "a", "")

	msgs, err := svc.GetMessages()
	if err != nil || len(msgs) != 1 {
		t.Fatalf("GetMessages = %v, %v", msgs, err)
	}
	if msgs[0].Email != "" || msgs[0].Status != "new" {
		t.Errorf("unexpected message %+v", msgs[0])
	}

	if err := svc.MarkMessageAsArchived(id); err != nil {
		t.Fatal(err)
	}
	m, err := svc.GetMessage(id)
	if err != nil || m.Status != "archived" {
		t.Fatalf("GetMessage = %+v, %v", m, err)
	}

	if err := svc.DeleteMessage(id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetMessage(id); err == nil {
		t.Error("expected error for deleted message")
	}
}

func TestErrorsOnClosedDB(t *testing.T) {
	svc, d := newTestService(t)
	d.Close()

	if _, err := svc.GetPhotoCount(); err == nil {
		t.Error("GetPhotoCount")
	}
	if _, err := svc.GetMessages(); err == nil {
		t.Error("GetMessages")
	}
	if _, err := svc.GetUnreadMessageCount(); err == nil {
		t.Error("GetUnreadMessageCount")
	}
	if _, err := svc.GetMessageCount(); err == nil {
		t.Error("GetMessageCount")
	}
	if err := svc.MarkMessageAsRead(1); err == nil {
		t.Error("MarkMessageAsRead")
	}
	if err := svc.MarkMessageAsArchived(1); err == nil {
		t.Error("MarkMessageAsArchived")
	}
	if err := svc.DeleteMessage(1); err == nil {
		t.Error("DeleteMessage")
	}
}
