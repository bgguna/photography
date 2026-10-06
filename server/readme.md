# Server

Go backend (Gin + SQLite via `modernc.org/sqlite` + zerolog) serving the API,
the server-rendered gallery/admin pages, and the photo files. Builds as a
single pure-Go binary, cross-compiled for the Raspberry Pi.

See [../docs/design.md](../docs/design.md) for the architecture and reasoning
behind these choices.

## Layout

- `main.go` — wiring: config, db, router, graceful shutdown
- `internal/db` — database connection handling
- `contact` — contact form handler + storage
- `photo` — photo upload, EXIF extraction, thumbnail generation
- (planned) `internal/auth`, `internal/gallery`, `internal/web` — see the
  design doc's [package layout](../docs/design.md#package-layout)

## Running

- `make build` — build the server binary
- `make run` — run the built binary
- `make test` / `make coverage` — run tests
- `make init-db` / `make shell-db` / `make backup-db` / `make reset-db` —
  database helpers (see `Makefile.db`)

[ [**back**](../readme.md) | [**client**](../client) | [**docs**](../docs) ]
