package photo

import (
	"bytes"
	"database/sql"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/bgguna/photography/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	schema := `
	CREATE TABLE photos (
		id INTEGER PRIMARY KEY,
		uploader_user_id INTEGER NOT NULL,
		original_mime_type TEXT NOT NULL,
		original_filename TEXT NOT NULL,
		original_path TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		datetime_original TEXT NULL,
		camera_make TEXT NULL,
		camera_model TEXT NULL,
		gps_lat REAL NULL,
		gps_lng REAL NULL,
		iso INTEGER NULL,
		aperture REAL NULL,
		shutter_speed TEXT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		is_public INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0,1)),
		FOREIGN KEY (uploader_user_id) REFERENCES users(id)
	);

	CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);
	`

	if _, err := database.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create a test user
	database.Exec("INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)",
		"test@example.com", "hash", "admin")

	return database
}

func setupTestStorage(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "photo-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	return tmpDir
}

func createTestImage(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	buf := bytes.NewBuffer(nil)
	jpeg.Encode(buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

func createTestMultipartFile(t *testing.T, filename string, data []byte) (*multipart.FileHeader, io.ReadCloser) {
	fileHeader := &multipart.FileHeader{
		Filename: filename,
		Size:     int64(len(data)),
		Header:   make(textproto.MIMEHeader),
	}
	fileHeader.Header.Set("Content-Disposition", "form-data; name=\"file\"; filename=\""+filename+"\"")
	fileHeader.Header.Set("Content-Type", "image/jpeg")

	file := io.NopCloser(bytes.NewReader(data))
	return fileHeader, file
}

func TestUploadPhoto(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Create test image
	imgData := createTestImage(2000, 1500)

	// Create multipart file
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	defer file.Close()

	// Upload photo
	photo, err := ps.UploadPhoto(file, fileHeader, 1)

	if err != nil {
		t.Fatalf("UploadPhoto() returned error: %v", err)
	}
	if photo == nil {
		t.Fatal("UploadPhoto() returned nil photo")
	}
	if photo.ID == 0 {
		t.Error("Photo ID should not be 0")
	}

	// Check files exist
	originalPath := filepath.Join(storage, "originals", "1.jpg")
	thumbPath := filepath.Join(storage, "derived", "1_thumb.jpg")
	webPath := filepath.Join(storage, "derived", "1_web.jpg")

	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		t.Error("Original file not found")
	}
	if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
		t.Error("Thumb file not found")
	}
	if _, err := os.Stat(webPath); os.IsNotExist(err) {
		t.Error("Web file not found")
	}
}

func TestUploadPhoto_NoEXIF(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Create PNG image (usually no EXIF)
	buf := bytes.NewBuffer(nil)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	png.Encode(buf, img)

	fileHeader, file := createTestMultipartFile(t, "test.png", buf.Bytes())
	defer file.Close()

	photo, err := ps.UploadPhoto(file, fileHeader, 1)

	if err != nil {
		t.Fatalf("UploadPhoto() should succeed without EXIF: %v", err)
	}
	if photo == nil {
		t.Fatal("Photo should not be nil")
	}

	// EXIF fields should be nil
	if photo.DatetimeOriginal != nil {
		t.Error("DatetimeOriginal should be nil for image without EXIF")
	}
}

func TestUploadPhoto_InvalidType(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Create invalid file type
	fileHeader := &multipart.FileHeader{
		Filename: "test.txt",
		Header:   make(textproto.MIMEHeader),
	}
	fileHeader.Header.Set("Content-Type", "text/plain")

	file := io.NopCloser(bytes.NewBuffer([]byte("test")))

	_, err := ps.UploadPhoto(file, fileHeader, 1)

	if err == nil {
		t.Fatal("UploadPhoto() should reject invalid file type")
	}
}

func TestDeletePhoto(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload a photo first
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	defer file.Close()

	photo, _ := ps.UploadPhoto(file, fileHeader, 1)

	// Delete the photo
	err := ps.DeletePhoto(photo.ID)

	if err != nil {
		t.Fatalf("DeletePhoto() returned error: %v", err)
	}

	// Check files are deleted
	originalPath := filepath.Join(storage, "originals", "1.jpg")
	if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
		t.Error("Original file should be deleted")
	}

	// Check DB row is deleted
	_, err = ps.GetPhoto(photo.ID)
	if err == nil {
		t.Error("Photo should not exist in database after deletion")
	}
}

func TestGetPhoto(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload a photo
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	defer file.Close()

	uploaded, _ := ps.UploadPhoto(file, fileHeader, 1)

	// Get the photo
	retrieved, err := ps.GetPhoto(uploaded.ID)

	if err != nil {
		t.Fatalf("GetPhoto() returned error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetPhoto() returned nil")
	}
	if retrieved.ID != uploaded.ID {
		t.Errorf("ID mismatch: %d vs %d", retrieved.ID, uploaded.ID)
	}
}

func TestGetPhoto_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	ps := NewPhotoService(database, storage)

	_, err := ps.GetPhoto(999)

	if err == nil {
		t.Fatal("GetPhoto() should return error for nonexistent photo")
	}
}

func TestListPublicPhotos(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload multiple photos
	imgData := createTestImage(1000, 800)
	for i := 0; i < 3; i++ {
		fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
		ps.UploadPhoto(file, fileHeader, 1)
		file.Close()
	}

	// List public photos
	photos, err := ps.ListPublicPhotos()

	if err != nil {
		t.Fatalf("ListPublicPhotos() returned error: %v", err)
	}
	if len(photos) != 3 {
		t.Errorf("Expected 3 photos, got %d", len(photos))
	}
}

func TestListAllPhotos(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload a photo and make it private
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	photo, _ := ps.UploadPhoto(file, fileHeader, 1)
	file.Close()

	ps.SetPhotoVisibility(photo.ID, false)

	// List all photos
	photos, err := ps.ListAllPhotos()

	if err != nil {
		t.Fatalf("ListAllPhotos() returned error: %v", err)
	}
	if len(photos) != 1 {
		t.Errorf("Expected 1 photo, got %d", len(photos))
	}
	if photos[0].IsPublic != 0 {
		t.Error("Photo should be private")
	}
}

func TestSetPhotoVisibility(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload a photo
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	photo, _ := ps.UploadPhoto(file, fileHeader, 1)
	file.Close()

	// Hide the photo
	err := ps.SetPhotoVisibility(photo.ID, false)

	if err != nil {
		t.Fatalf("SetPhotoVisibility() returned error: %v", err)
	}

	// Verify it's hidden
	updated, _ := ps.GetPhoto(photo.ID)
	if updated.IsPublic != 0 {
		t.Error("Photo should be hidden")
	}

	// Publish it again
	ps.SetPhotoVisibility(photo.ID, true)
	updated, _ = ps.GetPhoto(photo.ID)
	if updated.IsPublic != 1 {
		t.Error("Photo should be public")
	}
}

func TestGetPhotoFile_Public(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload a photo
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	photo, _ := ps.UploadPhoto(file, fileHeader, 1)
	file.Close()

	// Get thumbnail
	thumbPath, err := ps.GetPhotoFile(photo.ID, "thumb")

	if err != nil {
		t.Fatalf("GetPhotoFile() returned error: %v", err)
	}
	if thumbPath == "" {
		t.Error("Thumb path should not be empty")
	}

	// Get web size
	webPath, err := ps.GetPhotoFile(photo.ID, "web")

	if err != nil {
		t.Fatalf("GetPhotoFile() for web returned error: %v", err)
	}
	if webPath == "" {
		t.Error("Web path should not be empty")
	}
}

func TestGetPhotoFile_Hidden(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload and hide a photo
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	photo, _ := ps.UploadPhoto(file, fileHeader, 1)
	file.Close()

	ps.SetPhotoVisibility(photo.ID, false)

	// Try to get file
	_, err := ps.GetPhotoFile(photo.ID, "thumb")

	if err == nil {
		t.Fatal("GetPhotoFile() should return error for hidden photo")
	}
}

func TestGetAdminPhotoFile(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)

	ps := NewPhotoService(database, storage)

	// Upload a photo
	imgData := createTestImage(1000, 800)
	fileHeader, file := createTestMultipartFile(t, "test.jpg", imgData)
	photo, _ := ps.UploadPhoto(file, fileHeader, 1)
	file.Close()

	// Admin should be able to get all sizes
	thumbPath, err := ps.GetAdminPhotoFile(photo.ID, "thumb")
	if err != nil {
		t.Errorf("GetAdminPhotoFile thumb returned error: %v", err)
	}

	webPath, err := ps.GetAdminPhotoFile(photo.ID, "web")
	if err != nil {
		t.Errorf("GetAdminPhotoFile web returned error: %v", err)
	}

	origPath, err := ps.GetAdminPhotoFile(photo.ID, "original")
	if err != nil {
		t.Errorf("GetAdminPhotoFile original returned error: %v", err)
	}

	if thumbPath == "" || webPath == "" || origPath == "" {
		t.Error("Paths should not be empty")
	}
}

func TestIsValidImageType(t *testing.T) {
	tests := []struct {
		mimeType string
		valid    bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/gif", true},
		{"image/webp", true},
		{"text/plain", false},
		{"video/mp4", false},
		{"", false},
	}

	for _, test := range tests {
		if isValidImageType(test.mimeType) != test.valid {
			t.Errorf("isValidImageType(%s) = %v, want %v",
				test.mimeType, !test.valid, test.valid)
		}
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		filename string
		ext      string
	}{
		{"photo.jpg", "jpg"},
		{"photo.JPG", "jpg"},
		{"photo.png", "png"},
		{"photo", "jpg"},
		{".hidden", "hidden"}, // Hidden file without real extension becomes "hidden"
	}

	for _, test := range tests {
		result := getFileExtension(test.filename)
		if result != test.ext {
			t.Errorf("getFileExtension(%s) = %s, want %s",
				test.filename, result, test.ext)
		}
	}
}

func TestMovePhoto(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)
	ps := NewPhotoService(database, storage)

	var ids []int
	for i := 0; i < 3; i++ {
		fh, f := createTestMultipartFile(t, "t.jpg", createTestImage(500, 400))
		p, err := ps.UploadPhoto(f, fh, 1)
		f.Close()
		if err != nil {
			t.Fatalf("upload: %v", err)
		}
		ids = append(ids, p.ID)
	}

	order := func() []int {
		all, err := ps.ListAllPhotos()
		if err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, p := range all {
			out = append(out, p.ID)
		}
		return out
	}
	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	if got := order(); !eq(got, ids) {
		t.Fatalf("new uploads should append: got %v want %v", got, ids)
	}

	if err := ps.MovePhoto(ids[2], "up"); err != nil {
		t.Fatal(err)
	}
	if got, want := order(), []int{ids[0], ids[2], ids[1]}; !eq(got, want) {
		t.Errorf("after up: got %v want %v", got, want)
	}

	if err := ps.MovePhoto(ids[0], "down"); err != nil {
		t.Fatal(err)
	}
	if got, want := order(), []int{ids[2], ids[0], ids[1]}; !eq(got, want) {
		t.Errorf("after down: got %v want %v", got, want)
	}

	// Past the ends is a no-op
	ps.MovePhoto(ids[2], "up")
	ps.MovePhoto(ids[1], "down")
	if got, want := order(), []int{ids[2], ids[0], ids[1]}; !eq(got, want) {
		t.Errorf("edge moves should be no-ops: got %v want %v", got, want)
	}

	if err := ps.MovePhoto(ids[0], "sideways"); err == nil {
		t.Error("expected error for invalid direction")
	}
	if err := ps.MovePhoto(9999, "up"); err == nil {
		t.Error("expected error for unknown photo")
	}
}

func TestRegenerateThumbnails(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)
	ps := NewPhotoService(database, storage)

	fh, f := createTestMultipartFile(t, "r.jpg", createTestImage(1200, 800))
	defer f.Close()
	p, err := ps.UploadPhoto(f, fh, 1)
	if err != nil {
		t.Fatal(err)
	}

	thumb := filepath.Join(storage, "derived", "1_thumb.jpg")
	os.Remove(thumb)
	if err := ps.RegenerateThumbnails(p.ID); err != nil {
		t.Fatalf("RegenerateThumbnails: %v", err)
	}
	if _, err := os.Stat(thumb); err != nil {
		t.Error("thumbnail not regenerated")
	}

	if err := ps.RegenerateThumbnails(9999); err == nil {
		t.Error("expected error for missing photo")
	}

	os.Remove(p.OriginalPath)
	if err := ps.RegenerateThumbnails(p.ID); err == nil {
		t.Error("expected error for missing original")
	}
}

func TestExtractEXIF_NoData(t *testing.T) {
	if e := extractEXIF([]byte("not an image")); e != nil && e.CameraMake != nil {
		t.Errorf("unexpected EXIF: %+v", e)
	}
	_ = extractEXIF(createTestImage(10, 10))
}
