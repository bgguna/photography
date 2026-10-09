package gallery

import (
	"bytes"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/textproto"
	"os"
	"testing"

	"github.com/bgguna/photography/internal/db"
	"github.com/bgguna/photography/photo"
)

func TestGallery(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	schema, err := os.ReadFile("../../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO users (email, password_hash, role) VALUES ('a@b.c','h','admin')"); err != nil {
		t.Fatal(err)
	}

	ps := photo.NewPhotoService(database, t.TempDir())
	g := NewGallery(ps)

	buf := new(bytes.Buffer)
	jpeg.Encode(buf, image.NewRGBA(image.Rect(0, 0, 50, 50)), nil)
	hdr := &multipart.FileHeader{Filename: "x.jpg", Size: int64(buf.Len()), Header: textproto.MIMEHeader{}}
	hdr.Header.Set("Content-Type", "image/jpeg")
	p, err := ps.UploadPhoto(bytes.NewReader(buf.Bytes()), hdr, 1)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	if photos, err := g.GetPublicPhotos(); err != nil || len(photos) != 1 {
		t.Fatalf("GetPublicPhotos = %v, %v", photos, err)
	}
	if got, err := g.GetPhotoByID(p.ID); err != nil || got.ID != p.ID {
		t.Fatalf("GetPhotoByID = %v, %v", got, err)
	}

	if err := ps.SetPhotoVisibility(p.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := g.GetPhotoByID(p.ID); err == nil {
		t.Error("hidden photo should not be returned")
	}
	if _, err := g.GetPhotoByID(9999); err == nil {
		t.Error("missing photo should error")
	}
}
