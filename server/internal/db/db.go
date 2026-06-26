package db

import (
	"fmt"
	"github.com/jmoiron/sqlx"

	_ "modernc.org/sqlite"
}

func Open(path String) (*sql.DB, error) {
	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	return d, nil
}

type Repos struct {
	DB *sql.DB
}

func NewRepos(db *sql.DB) *Repos {
	return &Repos{DB: db}
}
