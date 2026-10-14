package db

import "testing"

func TestOpen_UnopenablePath(t *testing.T) {
	// A directory cannot be opened as a SQLite file, so the PRAGMA fails.
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open succeeded on a directory")
	}
}
