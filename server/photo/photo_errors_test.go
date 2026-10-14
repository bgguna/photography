package photo

import (
	"errors"
	"os"
	"testing"
)

// failingReader simulates a broken upload stream.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("stream broke") }

func TestGetPhotoFile_InvalidSize(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	storage := setupTestStorage(t)
	defer os.RemoveAll(storage)
	ps := NewPhotoService(database, storage)

	fh, f := createTestMultipartFile(t, "s.jpg", createTestImage(200, 150))
	defer f.Close()
	p, err := ps.UploadPhoto(f, fh, 1)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ps.GetPhotoFile(p.ID, "huge"); err == nil {
		t.Error("GetPhotoFile accepted an unknown size")
	}
	if _, err := ps.GetAdminPhotoFile(p.ID, "huge"); err == nil {
		t.Error("GetAdminPhotoFile accepted an unknown size")
	}
	if _, err := ps.GetPhotoFile(9999, "thumb"); err == nil {
		t.Error("GetPhotoFile accepted a missing photo")
	}
}

func TestUploadPhoto_ReadError(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	ps := NewPhotoService(database, t.TempDir())

	fh, f := createTestMultipartFile(t, "x.jpg", createTestImage(10, 10))
	defer f.Close()
	if _, err := ps.UploadPhoto(failingReader{}, fh, 1); err == nil {
		t.Error("UploadPhoto succeeded when the stream failed")
	}
}

func TestMovePhoto_DatabaseFailure(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	if _, err := database.Exec("DROP TABLE photos"); err != nil {
		t.Fatal(err)
	}
	ps := NewPhotoService(database, t.TempDir())

	if err := ps.MovePhoto(1, "up"); err == nil {
		t.Error("MovePhoto succeeded without the photos table")
	}
}

func TestNonEmpty(t *testing.T) {
	if nonEmpty("") != nil {
		t.Error("nonEmpty(\"\") should be nil")
	}
	if got := nonEmpty("Canon"); got == nil || *got != "Canon" {
		t.Errorf("nonEmpty(\"Canon\") = %v", got)
	}
}
