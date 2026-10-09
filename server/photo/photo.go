package photo

import (
	"database/sql"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rs/zerolog/log"
	"github.com/rwcarlsen/goexif/exif"
)

// Photo represents a photo in the database.
type Photo struct {
	ID               int
	UploaderUserID   int
	OriginalMIME     string
	OriginalFilename string
	OriginalPath     string
	CreatedAt        string
	UpdatedAt        string

	// EXIF data
	DatetimeOriginal *string
	CameraMake       *string
	CameraModel      *string
	GPSLat           *float64
	GPSLng           *float64
	ISO              *int
	Aperture         *float64
	ShutterSpeed     *string
	FocalLength      *float64

	// Admin fields
	SortOrder int
	IsPublic  int
}

// Metadata is the display-ready camera settings shown alongside a photo.
// Fields without EXIF data are empty strings.
type Metadata struct {
	Camera       string
	Date         string
	Aperture     string
	ShutterSpeed string
	ISO          string
	FocalLength  string
}

// Metadata formats the photo's EXIF fields for display.
func (p Photo) Metadata() Metadata {
	var m Metadata
	if p.CameraModel != nil {
		m.Camera = *p.CameraModel
	} else if p.CameraMake != nil {
		m.Camera = *p.CameraMake
	}
	if p.DatetimeOriginal != nil {
		if t, err := time.Parse(time.RFC3339, *p.DatetimeOriginal); err == nil {
			m.Date = t.Format("2 Jan 2006")
		}
	}
	if p.Aperture != nil {
		m.Aperture = fmt.Sprintf("f/%.1f", *p.Aperture)
	}
	if p.ShutterSpeed != nil {
		m.ShutterSpeed = *p.ShutterSpeed + " s"
	}
	if p.ISO != nil {
		m.ISO = fmt.Sprintf("ISO %d", *p.ISO)
	}
	if p.FocalLength != nil {
		m.FocalLength = fmt.Sprintf("%.0f mm", *p.FocalLength)
	}
	return m
}

// PhotoService handles photo operations.
type PhotoService struct {
	db            *sql.DB
	storagePath   string
	thumbSize     int
	webSize       int
	maxUploadSize int64
}

// NewPhotoService creates a new PhotoService.
func NewPhotoService(db *sql.DB, storagePath string) *PhotoService {
	return &PhotoService{
		db:            db,
		storagePath:   storagePath,
		thumbSize:     400,
		webSize:       1600,
		maxUploadSize: 50 * 1024 * 1024, // 50MB
	}
}

// UploadPhoto uploads a photo, extracts EXIF, and generates derived sizes.
func (ps *PhotoService) UploadPhoto(file io.Reader, fileHeader *multipart.FileHeader, uploaderUserID int) (*Photo, error) {
	// Validate content type
	if !isValidImageType(fileHeader.Header.Get("Content-Type")) {
		return nil, fmt.Errorf("unsupported image type")
	}

	// Check file size
	if fileHeader.Size > ps.maxUploadSize {
		return nil, fmt.Errorf("file too large")
	}

	// Read file into memory
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get file extension
	ext := getFileExtension(fileHeader.Filename)

	// Calculate original path first
	// Note: We'll insert with a placeholder ID and update after getting the real ID
	// Insert photo row first to get ID
	photo := &Photo{
		UploaderUserID:   uploaderUserID,
		OriginalMIME:     fileHeader.Header.Get("Content-Type"),
		OriginalFilename: fileHeader.Filename,
		IsPublic:         1,
		OriginalPath:     "pending", // Placeholder
	}

	err = ps.createPhoto(photo)
	if err != nil {
		return nil, fmt.Errorf("failed to create photo record: %w", err)
	}

	// Save original file with the actual ID
	originalPath := filepath.Join(ps.storagePath, "originals", fmt.Sprintf("%d.%s", photo.ID, ext))
	photo.OriginalPath = originalPath

	if err := ps.saveFile(originalPath, data); err != nil {
		_ = ps.deletePhotoRow(photo.ID)
		return nil, fmt.Errorf("failed to save original: %w", err)
	}

	// Update the original_path in database
	if err := ps.updatePhotoPath(photo.ID, originalPath); err != nil {
		_ = ps.deletePhotoRow(photo.ID)
		_ = os.Remove(originalPath)
		return nil, fmt.Errorf("failed to update original path: %w", err)
	}

	// Extract EXIF and update photo
	exifData := extractEXIF(data)
	if exifData != nil {
		photo.DatetimeOriginal = exifData.DatetimeOriginal
		photo.CameraMake = exifData.CameraMake
		photo.CameraModel = exifData.CameraModel
		photo.GPSLat = exifData.GPSLat
		photo.GPSLng = exifData.GPSLng
		photo.ISO = exifData.ISO
		photo.Aperture = exifData.Aperture
		photo.ShutterSpeed = exifData.ShutterSpeed
		photo.FocalLength = exifData.FocalLength

		if err := ps.updatePhotoEXIF(photo); err != nil {
			log.Warn().Err(err).Msgf("Failed to update EXIF for photo %d", photo.ID)
		}
	}

	// Generate derived sizes
	if err := ps.generateDerivedSizes(photo.ID, data); err != nil {
		_ = ps.deletePhotoRow(photo.ID)
		_ = os.Remove(originalPath)
		return nil, fmt.Errorf("failed to generate thumbnails: %w", err)
	}

	return photo, nil
}

// DeletePhoto deletes a photo and all associated files.
func (ps *PhotoService) DeletePhoto(photoID int) error {
	// Get photo to find files
	photo, err := ps.GetPhoto(photoID)
	if err != nil {
		return err
	}

	// Delete original file
	if photo.OriginalPath != "" {
		if err := os.Remove(photo.OriginalPath); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Msgf("Failed to delete original file for photo %d", photoID)
		}
	}

	// Delete derived files
	thumbPath := filepath.Join(ps.storagePath, "derived", fmt.Sprintf("%d_thumb.jpg", photoID))
	webPath := filepath.Join(ps.storagePath, "derived", fmt.Sprintf("%d_web.jpg", photoID))

	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Msgf("Failed to delete thumb file for photo %d", photoID)
	}
	if err := os.Remove(webPath); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Msgf("Failed to delete web file for photo %d", photoID)
	}

	// Delete from database
	return ps.deletePhotoRow(photoID)
}

// GetPhoto retrieves a photo by ID.
func (ps *PhotoService) GetPhoto(photoID int) (*Photo, error) {
	var photo Photo
	var dateOrig, make, model, speed sql.NullString
	var gpsLat, gpsLng, ap, fl sql.NullFloat64
	var iso sql.NullInt64

	err := ps.db.QueryRow(`
		SELECT id, uploader_user_id, original_mime_type, original_filename,
		       original_path, created_at, updated_at,
		       datetime_original, camera_make, camera_model,
		       gps_lat, gps_lng, iso, aperture, shutter_speed, focal_length,
		       sort_order, is_public
		FROM photos WHERE id = ?
	`, photoID).Scan(
		&photo.ID, &photo.UploaderUserID, &photo.OriginalMIME, &photo.OriginalFilename,
		&photo.OriginalPath, &photo.CreatedAt, &photo.UpdatedAt,
		&dateOrig, &make, &model, &gpsLat, &gpsLng, &iso, &ap, &speed, &fl,
		&photo.SortOrder, &photo.IsPublic,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("photo not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get photo: %w", err)
	}

	// Convert NULL values
	if dateOrig.Valid {
		photo.DatetimeOriginal = &dateOrig.String
	}
	if make.Valid {
		photo.CameraMake = &make.String
	}
	if model.Valid {
		photo.CameraModel = &model.String
	}
	if gpsLat.Valid {
		photo.GPSLat = &gpsLat.Float64
	}
	if gpsLng.Valid {
		photo.GPSLng = &gpsLng.Float64
	}
	if iso.Valid {
		isoVal := int(iso.Int64)
		photo.ISO = &isoVal
	}
	if ap.Valid {
		photo.Aperture = &ap.Float64
	}
	if speed.Valid {
		photo.ShutterSpeed = &speed.String
	}
	if fl.Valid {
		photo.FocalLength = &fl.Float64
	}

	return &photo, nil
}

// ListPublicPhotos lists public photos ordered by sort_order.
func (ps *PhotoService) ListPublicPhotos() ([]Photo, error) {
	return ps.listPhotos("WHERE is_public = 1")
}

// ListAllPhotos lists all photos (for admin).
func (ps *PhotoService) ListAllPhotos() ([]Photo, error) {
	return ps.listPhotos("")
}

// SetPhotoVisibility sets a photo's public/hidden status.
func (ps *PhotoService) SetPhotoVisibility(photoID int, isPublic bool) error {
	val := 0
	if isPublic {
		val = 1
	}

	_, err := ps.db.Exec(
		"UPDATE photos SET is_public = ? WHERE id = ?",
		val, photoID,
	)
	if err != nil {
		return fmt.Errorf("failed to update visibility: %w", err)
	}
	return nil
}

// GetPhotoFile returns the path to a photo file for serving.
func (ps *PhotoService) GetPhotoFile(photoID int, size string) (string, error) {
	photo, err := ps.GetPhoto(photoID)
	if err != nil {
		return "", err
	}

	// Check if photo is public
	if photo.IsPublic == 0 {
		return "", fmt.Errorf("photo not found")
	}

	switch size {
	case "thumb":
		return filepath.Join(ps.storagePath, "derived", fmt.Sprintf("%d_thumb.jpg", photoID)), nil
	case "web":
		return filepath.Join(ps.storagePath, "derived", fmt.Sprintf("%d_web.jpg", photoID)), nil
	default:
		return "", fmt.Errorf("invalid size")
	}
}

// GetAdminPhotoFile returns the path to a photo file (admin only, no visibility check).
func (ps *PhotoService) GetAdminPhotoFile(photoID int, size string) (string, error) {
	_, err := ps.GetPhoto(photoID)
	if err != nil {
		return "", err
	}

	switch size {
	case "thumb":
		return filepath.Join(ps.storagePath, "derived", fmt.Sprintf("%d_thumb.jpg", photoID)), nil
	case "web":
		return filepath.Join(ps.storagePath, "derived", fmt.Sprintf("%d_web.jpg", photoID)), nil
	case "original":
		photo, _ := ps.GetPhoto(photoID)
		return photo.OriginalPath, nil
	default:
		return "", fmt.Errorf("invalid size")
	}
}

// RegenerateThumbnails regenerates thumbnail and web sizes from the original.
func (ps *PhotoService) RegenerateThumbnails(photoID int) error {
	photo, err := ps.GetPhoto(photoID)
	if err != nil {
		return err
	}

	// Read original file
	data, err := os.ReadFile(photo.OriginalPath)
	if err != nil {
		return fmt.Errorf("failed to read original: %w", err)
	}

	// Generate derived sizes
	return ps.generateDerivedSizes(photoID, data)
}

// Private helper functions

// createPhoto inserts a new photo into the database.
func (ps *PhotoService) createPhoto(photo *Photo) error {
	result, err := ps.db.Exec(`
		INSERT INTO photos (uploader_user_id, original_mime_type, original_filename,
		                   original_path, is_public, sort_order)
		VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM photos))
	`, photo.UploaderUserID, photo.OriginalMIME, photo.OriginalFilename,
		"", photo.IsPublic)

	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	photo.ID = int(id)
	return nil
}

// updatePhotoEXIF updates EXIF data for a photo.
func (ps *PhotoService) updatePhotoEXIF(photo *Photo) error {
	_, err := ps.db.Exec(`
		UPDATE photos
		SET datetime_original = ?, camera_make = ?, camera_model = ?,
		    gps_lat = ?, gps_lng = ?, iso = ?, aperture = ?, shutter_speed = ?,
		    focal_length = ?
		WHERE id = ?
	`, photo.DatetimeOriginal, photo.CameraMake, photo.CameraModel,
		photo.GPSLat, photo.GPSLng, photo.ISO, photo.Aperture, photo.ShutterSpeed,
		photo.FocalLength, photo.ID)

	return err
}

// deletePhotoRow deletes a photo record from the database.
func (ps *PhotoService) deletePhotoRow(photoID int) error {
	_, err := ps.db.Exec("DELETE FROM photos WHERE id = ?", photoID)
	return err
}

// updatePhotoPath updates the original_path for a photo.
func (ps *PhotoService) updatePhotoPath(photoID int, path string) error {
	_, err := ps.db.Exec(
		"UPDATE photos SET original_path = ? WHERE id = ?",
		path, photoID)
	return err
}

// listPhotos retrieves photos with optional WHERE clause.
func (ps *PhotoService) listPhotos(whereClause string) ([]Photo, error) {
	query := `
		SELECT id, uploader_user_id, original_mime_type, original_filename,
		       original_path, created_at, updated_at,
		       datetime_original, camera_make, camera_model,
		       gps_lat, gps_lng, iso, aperture, shutter_speed, focal_length,
		       sort_order, is_public
		FROM photos
	`
	if whereClause != "" {
		query += " " + whereClause
	}
	query += " ORDER BY sort_order ASC, id ASC"

	rows, err := ps.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query photos: %w", err)
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var photo Photo
		var dateOrig, make, model, speed sql.NullString
		var gpsLat, gpsLng, ap, fl sql.NullFloat64
		var iso sql.NullInt64

		if err := rows.Scan(
			&photo.ID, &photo.UploaderUserID, &photo.OriginalMIME, &photo.OriginalFilename,
			&photo.OriginalPath, &photo.CreatedAt, &photo.UpdatedAt,
			&dateOrig, &make, &model, &gpsLat, &gpsLng, &iso, &ap, &speed, &fl,
			&photo.SortOrder, &photo.IsPublic,
		); err != nil {
			return nil, err
		}

		// Convert NULL values
		if dateOrig.Valid {
			photo.DatetimeOriginal = &dateOrig.String
		}
		if make.Valid {
			photo.CameraMake = &make.String
		}
		if model.Valid {
			photo.CameraModel = &model.String
		}
		if gpsLat.Valid {
			photo.GPSLat = &gpsLat.Float64
		}
		if gpsLng.Valid {
			photo.GPSLng = &gpsLng.Float64
		}
		if iso.Valid {
			isoVal := int(iso.Int64)
			photo.ISO = &isoVal
		}
		if ap.Valid {
			photo.Aperture = &ap.Float64
		}
		if speed.Valid {
			photo.ShutterSpeed = &speed.String
		}
		if fl.Valid {
			photo.FocalLength = &fl.Float64
		}

		photos = append(photos, photo)
	}

	return photos, rows.Err()
}

// generateDerivedSizes generates thumbnail and web-sized versions.
func (ps *PhotoService) generateDerivedSizes(photoID int, imageData []byte) error {
	// Decode image
	img, err := imaging.Decode(strings.NewReader(string(imageData)))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Ensure deriveddir exists
	derivedDir := filepath.Join(ps.storagePath, "derived")
	if err := os.MkdirAll(derivedDir, 0755); err != nil {
		return fmt.Errorf("failed to create derived directory: %w", err)
	}

	// Generate thumb
	thumb := imaging.Resize(img, ps.thumbSize, ps.thumbSize, imaging.Lanczos)
	thumbPath := filepath.Join(derivedDir, fmt.Sprintf("%d_thumb.jpg", photoID))
	if err := ps.saveJPEG(thumbPath, thumb); err != nil {
		return fmt.Errorf("failed to save thumb: %w", err)
	}

	// Generate web size
	web := imaging.Resize(img, ps.webSize, ps.webSize, imaging.Lanczos)
	webPath := filepath.Join(derivedDir, fmt.Sprintf("%d_web.jpg", photoID))
	if err := ps.saveJPEG(webPath, web); err != nil {
		_ = os.Remove(thumbPath)
		return fmt.Errorf("failed to save web: %w", err)
	}

	return nil
}

// saveJPEG saves an image as JPEG.
func (ps *PhotoService) saveJPEG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
}

// saveFile writes data to a file.
func (ps *PhotoService) saveFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Helper functions

// isValidImageType checks if the mime type is a supported image.
func isValidImageType(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	}
	return false
}

// getFileExtension extracts the file extension.
func getFileExtension(filename string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	if ext == "" {
		ext = "jpg"
	}
	return ext
}

// EXIFData holds extracted EXIF information.
type EXIFData struct {
	DatetimeOriginal *string
	CameraMake       *string
	CameraModel      *string
	GPSLat           *float64
	GPSLng           *float64
	ISO              *int
	Aperture         *float64
	ShutterSpeed     *string
	FocalLength      *float64
}

// extractEXIF extracts EXIF data from image bytes.
func extractEXIF(data []byte) *EXIFData {
	exifData := &EXIFData{}
	reader := strings.NewReader(string(data))

	x, err := exif.Decode(reader)
	if err != nil {
		return exifData
	}

	// DateTime
	if dt, err := x.DateTime(); err == nil {
		dtStr := dt.Format(time.RFC3339)
		exifData.DatetimeOriginal = &dtStr
	}

	// Camera Make
	if make, err := x.Get(exif.Make); err == nil && make != nil {
		if str, err := make.StringVal(); err == nil {
			exifData.CameraMake = &str
		}
	}

	// Camera Model
	if model, err := x.Get(exif.Model); err == nil && model != nil {
		if str, err := model.StringVal(); err == nil {
			exifData.CameraModel = &str
		}
	}

	// GPS (decimal degrees, sign applied from the N/S and E/W reference tags)
	if lat, lng, err := x.LatLong(); err == nil {
		exifData.GPSLat = &lat
		exifData.GPSLng = &lng
	}

	// ISO
	if iso, err := x.Get(exif.ISOSpeedRatings); err == nil && iso != nil {
		if v, err := iso.Int(0); err == nil {
			exifData.ISO = &v
		}
	}

	// Aperture
	if f, err := x.Get(exif.FNumber); err == nil && f != nil {
		if r, err := f.Rat(0); err == nil {
			val, _ := r.Float64()
			exifData.Aperture = &val
		}
	}

	// Shutter Speed
	if ss, err := x.Get(exif.ExposureTime); err == nil && ss != nil {
		if r, err := ss.Rat(0); err == nil {
			str := r.String()
			exifData.ShutterSpeed = &str
		}
	}

	// Focal Length
	if fl, err := x.Get(exif.FocalLength); err == nil && fl != nil {
		if r, err := fl.Rat(0); err == nil {
			val, _ := r.Float64()
			exifData.FocalLength = &val
		}
	}

	return exifData
}

// MovePhoto moves a photo one position up or down in the display order.
// Moving past either end is a no-op.
func (ps *PhotoService) MovePhoto(photoID int, direction string) error {
	if direction != "up" && direction != "down" {
		return fmt.Errorf("invalid direction %q", direction)
	}

	tx, err := ps.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query("SELECT id FROM photos ORDER BY sort_order ASC, id ASC")
	if err != nil {
		return fmt.Errorf("failed to list photo order: %w", err)
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	idx := -1
	for i, id := range ids {
		if id == photoID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("photo not found")
	}

	swap := idx - 1
	if direction == "down" {
		swap = idx + 1
	}
	if swap >= 0 && swap < len(ids) {
		ids[idx], ids[swap] = ids[swap], ids[idx]
	}

	for i, id := range ids {
		if _, err := tx.Exec("UPDATE photos SET sort_order = ? WHERE id = ?", i, id); err != nil {
			return fmt.Errorf("failed to update sort order: %w", err)
		}
	}
	return tx.Commit()
}
