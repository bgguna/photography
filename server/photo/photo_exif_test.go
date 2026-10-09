package photo

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

type ifdEntry struct {
	tag, typ uint16
	count    uint32
	data     []byte // little-endian payload
	sub      int    // index into subIFDs for LONG pointer entries, -1 otherwise
}

func ascii(s string) []byte { return append([]byte(s), 0) }

func rationals(vals ...[2]uint32) []byte {
	b := new(bytes.Buffer)
	for _, v := range vals {
		binary.Write(b, binary.LittleEndian, v[0])
		binary.Write(b, binary.LittleEndian, v[1])
	}
	return b.Bytes()
}

func short(v uint16) []byte { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, v); return b }

func ifdLen(es []ifdEntry) int {
	n := 2 + 12*len(es) + 4
	for _, e := range es {
		if e.sub < 0 && len(e.data) > 4 {
			n += (len(e.data) + 1) &^ 1
		}
	}
	return n
}

// buildEXIFJPEG returns a JPEG whose APP1 segment carries IFD0 (with pointers to
// an Exif IFD and a GPS IFD).
func buildEXIFJPEG() []byte {
	exifIFD := []ifdEntry{
		{0x829A, 5, 1, rationals([2]uint32{1, 250}), -1},
		{0x829D, 5, 1, rationals([2]uint32{28, 10}), -1},
		{0x8827, 3, 1, short(400), -1},
		{0x920A, 5, 1, rationals([2]uint32{50, 1}), -1},
		{0x9003, 2, 20, ascii("2024:05:06 07:08:09"), -1},
	}
	gpsIFD := []ifdEntry{
		{0x0001, 2, 2, ascii("N"), -1},
		{0x0002, 5, 3, rationals([2]uint32{45, 1}, [2]uint32{30, 1}, [2]uint32{0, 1}), -1},
		{0x0003, 2, 2, ascii("E"), -1},
		{0x0004, 5, 3, rationals([2]uint32{9, 1}, [2]uint32{0, 1}, [2]uint32{0, 1}), -1},
	}
	ifd0 := []ifdEntry{
		{0x010F, 2, 6, ascii("Canon"), -1},
		{0x0110, 2, 6, ascii("EOS R"), -1},
		{0x8769, 4, 1, nil, 0},
		{0x8825, 4, 1, nil, 1},
	}
	subs := [][]ifdEntry{exifIFD, gpsIFD}

	offs := []int{8 + ifdLen(ifd0)}
	offs = append(offs, offs[0]+ifdLen(exifIFD))

	tiff := new(bytes.Buffer)
	tiff.WriteString("II*\x00")
	binary.Write(tiff, binary.LittleEndian, uint32(8))

	write := func(es []ifdEntry, base int) {
		binary.Write(tiff, binary.LittleEndian, uint16(len(es)))
		dataOff := base + 2 + 12*len(es) + 4
		var tail []byte
		for _, e := range es {
			binary.Write(tiff, binary.LittleEndian, e.tag)
			binary.Write(tiff, binary.LittleEndian, e.typ)
			binary.Write(tiff, binary.LittleEndian, e.count)
			switch {
			case e.sub >= 0:
				binary.Write(tiff, binary.LittleEndian, uint32(offs[e.sub]))
			case len(e.data) <= 4:
				v := make([]byte, 4)
				copy(v, e.data)
				tiff.Write(v)
			default:
				binary.Write(tiff, binary.LittleEndian, uint32(dataOff+len(tail)))
				tail = append(tail, e.data...)
				if len(tail)%2 == 1 {
					tail = append(tail, 0)
				}
			}
		}
		binary.Write(tiff, binary.LittleEndian, uint32(0))
		tiff.Write(tail)
	}
	write(ifd0, 8)
	for i, s := range subs {
		write(s, offs[i])
	}

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := new(bytes.Buffer)
	seg.Write([]byte{0xFF, 0xD8, 0xFF, 0xE1})
	binary.Write(seg, binary.BigEndian, uint16(len(payload)+2))
	seg.Write(payload)
	seg.Write(createTestImage(40, 30)[2:]) // drop the original SOI
	return seg.Bytes()
}

func TestExtractEXIF_Full(t *testing.T) {
	e := extractEXIF(buildEXIFJPEG())
	if e.CameraMake == nil || *e.CameraMake != "Canon" {
		t.Errorf("make = %v", e.CameraMake)
	}
	if e.CameraModel == nil || *e.CameraModel != "EOS R" {
		t.Errorf("model = %v", e.CameraModel)
	}
	if e.DatetimeOriginal == nil {
		t.Error("datetime missing")
	}
	if e.ISO == nil || *e.ISO != 400 {
		t.Errorf("iso = %v", e.ISO)
	}
	if e.Aperture == nil || *e.Aperture != 2.8 {
		t.Errorf("aperture = %v", e.Aperture)
	}
	if e.ShutterSpeed == nil || *e.ShutterSpeed != "1/250" {
		t.Errorf("shutter = %v", e.ShutterSpeed)
	}
	if e.FocalLength == nil || *e.FocalLength != 50 {
		t.Errorf("focal length = %v", e.FocalLength)
	}
	if e.GPSLat == nil || *e.GPSLat != 45.5 {
		t.Errorf("lat = %v", e.GPSLat)
	}
	if e.GPSLng == nil || *e.GPSLng != 9 {
		t.Errorf("lng = %v", e.GPSLng)
	}
}

func TestUploadPhoto_WithEXIFRoundTrip(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	ps := NewPhotoService(database, t.TempDir())

	fh, f := createTestMultipartFile(t, "e.jpg", buildEXIFJPEG())
	defer f.Close()
	p, err := ps.UploadPhoto(f, fh, 1)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ps.GetPhoto(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CameraMake == nil || got.CameraModel == nil || got.DatetimeOriginal == nil ||
		got.GPSLat == nil || got.GPSLng == nil || got.ISO == nil || got.Aperture == nil || got.ShutterSpeed == nil || got.FocalLength == nil {
		t.Errorf("EXIF fields not persisted: %+v", got)
	}
	all, err := ps.ListAllPhotos()
	if err != nil || len(all) != 1 || all[0].ISO == nil || *all[0].ISO != 400 {
		t.Errorf("list = %+v, %v", all, err)
	}
}

func TestUploadPhoto_Failures(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	storage := t.TempDir()
	ps := NewPhotoService(database, storage)

	// Too large.
	fh, f := createTestMultipartFile(t, "big.jpg", createTestImage(10, 10))
	fh.Size = ps.maxUploadSize + 1
	if _, err := ps.UploadPhoto(f, fh, 1); err == nil {
		t.Error("expected too-large error")
	}

	// Undecodable data: derived-size generation fails and the row is cleaned up.
	fh, f = createTestMultipartFile(t, "bad.jpg", []byte("garbage"))
	if _, err := ps.UploadPhoto(f, fh, 1); err == nil {
		t.Error("expected decode error")
	}
	if all, _ := ps.ListAllPhotos(); len(all) != 0 {
		t.Errorf("row not cleaned up: %d", len(all))
	}

	// Unwritable originals location.
	blocker := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocker, []byte("x"), 0644)
	ps2 := NewPhotoService(database, blocker)
	fh, f = createTestMultipartFile(t, "ok.jpg", createTestImage(10, 10))
	if _, err := ps2.UploadPhoto(f, fh, 1); err == nil {
		t.Error("expected save error")
	}

	// Unknown uploader violates the foreign key.
	fh, f = createTestMultipartFile(t, "ok.jpg", createTestImage(10, 10))
	if _, err := ps.UploadPhoto(f, fh, 999); err == nil {
		t.Error("expected create error")
	}
}

func TestDeletePhoto_MissingFilesAndErrors(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	ps := NewPhotoService(database, t.TempDir())

	if err := ps.DeletePhoto(42); err == nil {
		t.Error("expected not-found error")
	}

	fh, f := createTestMultipartFile(t, "d.jpg", createTestImage(10, 10))
	p, err := ps.UploadPhoto(f, fh, 1)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(p.OriginalPath) // files already gone is tolerated
	if err := ps.DeletePhoto(p.ID); err != nil {
		t.Errorf("DeletePhoto: %v", err)
	}
}

func TestClosedDBErrors(t *testing.T) {
	database := setupTestDB(t)
	ps := NewPhotoService(database, t.TempDir())
	database.Close()

	if _, err := ps.GetPhoto(1); err == nil {
		t.Error("GetPhoto")
	}
	if _, err := ps.ListAllPhotos(); err == nil {
		t.Error("ListAllPhotos")
	}
	if err := ps.SetPhotoVisibility(1, true); err == nil {
		t.Error("SetPhotoVisibility")
	}
	if err := ps.MovePhoto(1, "up"); err == nil {
		t.Error("MovePhoto")
	}
	if err := ps.MovePhoto(1, "sideways"); err == nil {
		t.Error("MovePhoto invalid direction")
	}
}

func TestGenerateDerivedSizes_Errors(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	blocker := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocker, []byte("x"), 0644)
	ps := NewPhotoService(database, blocker)
	if err := ps.generateDerivedSizes(1, createTestImage(10, 10)); err == nil {
		t.Error("expected mkdir error")
	}
	if err := ps.saveJPEG(filepath.Join(blocker, "x.jpg"), nil); err == nil {
		t.Error("expected create error")
	}
}

func TestPhotoMetadata(t *testing.T) {
	str := func(s string) *string { return &s }
	f := func(v float64) *float64 { return &v }
	i := func(v int) *int { return &v }

	full := Photo{
		DatetimeOriginal: str("2024-05-06T07:08:09Z"),
		CameraMake:       str("Canon"),
		CameraModel:      str("EOS R"),
		ISO:              i(400),
		Aperture:         f(2.8),
		ShutterSpeed:     str("1/250"),
		FocalLength:      f(49.6),
	}
	want := Metadata{
		Camera:       "EOS R",
		Date:         "6 May 2024",
		Aperture:     "f/2.8",
		ShutterSpeed: "1/250 s",
		ISO:          "ISO 400",
		FocalLength:  "50 mm",
	}
	if got := full.Metadata(); got != want {
		t.Errorf("Metadata() = %+v, want %+v", got, want)
	}

	// Make is used when the model is missing; empty photo yields empty fields.
	if got := (Photo{CameraMake: str("Fuji")}).Metadata(); got.Camera != "Fuji" {
		t.Errorf("camera fallback = %q", got.Camera)
	}
	if got := (Photo{}).Metadata(); got != (Metadata{}) {
		t.Errorf("empty Metadata() = %+v", got)
	}
	// Unparseable date is dropped rather than shown raw.
	if got := (Photo{DatetimeOriginal: str("not a date")}).Metadata(); got.Date != "" {
		t.Errorf("bad date = %q", got.Date)
	}
}
