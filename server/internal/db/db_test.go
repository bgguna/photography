package db

import "testing"

func TestOpenAndRepos(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var fk int
	if err := d.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d, %v", fk, err)
	}
	if r := NewRepos(d); r.DB != d {
		t.Error("NewRepos did not keep handle")
	}
}
