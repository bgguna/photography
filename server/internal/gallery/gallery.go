package gallery

import (
	"fmt"

	"github.com/bgguna/photography/photo"
)

// Gallery represents the gallery service.
type Gallery struct {
	photoSvc *photo.PhotoService
}

// NewGallery creates a new Gallery service.
func NewGallery(photoSvc *photo.PhotoService) *Gallery {
	return &Gallery{
		photoSvc: photoSvc,
	}
}

// GetPublicPhotos returns all public photos ordered by sort_order.
func (g *Gallery) GetPublicPhotos() ([]photo.Photo, error) {
	return g.photoSvc.ListPublicPhotos()
}

// GetPhotoByID returns a photo if it's public.
func (g *Gallery) GetPhotoByID(photoID int) (*photo.Photo, error) {
	p, err := g.photoSvc.GetPhoto(photoID)
	if err != nil {
		return nil, err
	}

	if p.IsPublic == 0 {
		return nil, fmt.Errorf("photo not found")
	}

	return p, nil
}
