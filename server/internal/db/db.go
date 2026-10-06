package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path and enables foreign key enforcement.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	return db, nil
}

// Repos bundles the shared database handle for repository packages to embed.
type Repos struct {
	DB *sql.DB
}

func NewRepos(db *sql.DB) *Repos {
	return &Repos{DB: db}
}
