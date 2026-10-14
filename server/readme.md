# Server

Go backend (Gin, SQLite via `modernc.org/sqlite`, zerolog). It serves the
public pages, the admin area, and the photo files. The build is a single
pure-Go binary, cross-compiled for the Raspberry Pi.

Pages are server-rendered from `internal/web/templates`. htmx and Alpine.js,
in `internal/web/static/vendor`, handle small interactions. The shared
stylesheet and theme toggle are in `internal/web/static/theme.css` and
`theme.js`.

See [../docs/design.md](../docs/design.md) for the architecture and the
reasoning behind these choices.

## Layout

- `main.go`: wiring. Config, database, router, public routes, graceful
  shutdown.
- `admin_routes.go`: the `/admin` routes, behind login and CSRF checks.
- `internal/config`: reads settings from the environment.
- `internal/db`: opens the SQLite database.
- `internal/auth`: login sessions, auth middleware, CSRF, rate limiting.
- `internal/admin`: admin queries (photo count, contact messages).
- `internal/gallery`: the public photo list, showing only visible photos.
- `internal/about`: the editable About page text and its paragraph splitting.
- `internal/web`: `templates/` (public pages, shared header partial, admin
  pages) and `static/`.
- `photo`: upload, EXIF extraction, derived sizes, visibility, ordering.
- `contact`: contact form handler and storage.
- `cmd/createadmin`: creates an admin user and applies the schema.
- `cmd/backfill-exif`: re-reads EXIF from the stored originals. Used once
  after the EXIF fixes; safe to run again.

## Configuration

Settings are read from the environment. The `.env` file in the repository root
is loaded automatically by the `cmd/` tools and the Makefiles.

| Variable | Default | Notes |
|----------|---------|-------|
| `PORT` | `8080` | HTTP port the server listens on. |
| `DB_PATH` | `../db/gallery.sqlite` | SQLite file. The default is relative to `server/`. |
| `PHOTO_STORAGE_PATH` | none, **required** | Root folder for photo files. The server writes `originals/` and `derived/` inside it, and checks that it exists and is writable at startup. |
| `SESSION_SECRET` | none | Required when `MODE_ENV=production`. Keys the CSRF tokens. |
| `MODE_ENV` | `development` | `production` enables secure cookies and requires `SESSION_SECRET`. |

The database helpers in `Makefile.db` also read `DB_DIR` and `BACKUP_DIR` from
`.env`.

## Routes

Public:

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/` | Home: the gallery of visible photos. |
| GET | `/about` | About page. |
| GET | `/contact` | Contact form. |
| POST | `/contact` | Submit a message. Honeypot field and 5 submissions per IP per hour. |
| GET | `/photos/:id/thumb`, `/photos/:id/web` | Derived image files. 404 for hidden or missing photos. |
| GET | `/healthz` | Health check. |

Admin (login required, CSRF token on every change). Login allows 5 attempts per
IP per 15 minutes:

| Method | Path | Purpose |
|--------|------|---------|
| GET, POST | `/admin/login` | Log in. |
| POST | `/admin/logout` | Log out. |
| GET | `/admin` | Dashboard. |
| GET, POST | `/admin/photos` | List photos; upload one. |
| GET | `/admin/photos/:id/thumb`, `/web`, `/original` | Any photo, including hidden ones. |
| DELETE | `/admin/photos/:id` | Delete a photo. |
| POST | `/admin/photos/:id/visibility` | Show or hide a photo. |
| POST | `/admin/photos/:id/move` | Move a photo up or down the order. |
| GET | `/admin/messages` | Contact messages. |
| POST | `/admin/messages/:id/read`, `/archive` | Mark a message read or archived. |
| DELETE | `/admin/messages/:id` | Delete a message. |
| GET, POST | `/admin/about` | Edit the About page text. |

## Running

Run these from this directory.

- `make init-db`: create the database from `../db/schema.sql`. Safe to re-run;
  it only adds missing tables.
- `go run ./cmd/createadmin -email <email> -password <password>`: create an
  admin user. Run `make init-db` first.
- `go run .`, or `make build` then `make run`: start the server.
- `make test`: run the tests. `make coverage`: run them with a coverage report.
- `make format`, `make lint`: format and lint the code.

Database helpers (see `Makefile.db`): `make init-db`, `make shell-db`,
`make backup-db`, `make restore-db BACKUP_FILE=...`, `make reset-db` (deletes
the database and recreates it, so use it only on a throwaway copy).

## Maintenance

- **After a schema change:** run `make init-db` against the database in use
  (locally, or on the Pi with `sqlite3`; see the deployment guide). Tables are
  added without touching existing data.
- **EXIF backfill:** `go run ./cmd/backfill-exif`. It reads `DB_PATH` and
  `PHOTO_STORAGE_PATH` from the environment, or from `-db` and `-storage`. It
  prints how many photos were refreshed. Back up the database first.

## Tests

`make test` runs every package. The tests use an in-memory database with the
schema from `../db/schema.sql`, so they don't touch your data.

[ [**back**](../readme.md) | [**deploy**](../deploy/DEPLOYMENT.md) | [**docs**](../docs) ]
