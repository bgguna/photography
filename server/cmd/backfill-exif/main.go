// Command backfill-exif re-reads EXIF from every stored original and rewrites
// the photo rows. Run it once after the EXIF extraction fixes. It is safe to
// run again.
//
// Back up the database before running it on the Pi:
//
//	make -f Makefile.db backup
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/bgguna/photography/internal/db"
	"github.com/bgguna/photography/photo"
)

func main() {
	// Load .env first so the storage path default below can come from it.
	_ = godotenv.Load("../.env")

	dbPath := flag.String("db", "../db/gallery.sqlite", "Path to database")
	storagePath := flag.String("storage", os.Getenv("PHOTO_STORAGE_PATH"), "Photo storage path (defaults to PHOTO_STORAGE_PATH)")
	flag.Parse()

	if *storagePath == "" {
		log.Fatal("No photo storage path: set PHOTO_STORAGE_PATH or pass -storage")
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	svc := photo.NewPhotoService(database, *storagePath)
	refreshed, errs := svc.RefreshEXIF()

	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "error:", e)
	}
	fmt.Printf("Refreshed EXIF for %d photos, %d failed\n", refreshed, len(errs))
	if len(errs) > 0 {
		os.Exit(1)
	}
}
