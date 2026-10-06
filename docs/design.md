# Design

Self-hosted photography portfolio: public gallery + contact form, admin mode for
publishing photos, deployed on a Raspberry Pi with photos on an external drive.

## Decisions

- **Client**: Go `html/template` (server-rendered), enhanced with htmx + a little
  vanilla JS/CSS for interactivity (lightbox, async upload with progress,
  drag-to-reorder in admin). No separate SPA/build pipeline.
- **Admin auth**: server-side session cookie (HttpOnly), not JWT.
- **Deployment**: native cross-compiled binary + systemd service + Caddy reverse
  proxy, not Docker.
- **SQLite driver**: `modernc.org/sqlite` (pure Go) instead of `mattn/go-sqlite3`
  (cgo), so cross-compiling to the Pi's ARM arch needs no C toolchain.
- **Logging**: zerolog only (drop logrus).

## Architecture

```
[Browser] → [Caddy: TLS + reverse proxy + serves photo files directly]
                    │
                    ▼
            [Go/Gin API server]  ← systemd service
                    │
         ┌──────────┼──────────────┐
         ▼                         ▼
   [SQLite DB on SD card]   [External drive: originals + generated web/thumb sizes]
```

Caddy terminates TLS, reverse-proxies everything else to the Go process, and
serves `/derived/*` and gated `/originals/*` directly off disk so the Go
process isn't streaming image bytes itself.

## Package layout

```
server/
  main.go              # wiring: config, db, router, graceful shutdown
  internal/
    db/                # db.Open(), migrations runner
    auth/               # session middleware, login/logout, password hashing
    photo/              # upload, EXIF extraction, thumbnail generation, listing
    contact/            # contact form handler + storage
    gallery/             # public gallery rendering, album/visibility logic
    web/
      templates/        # .html templates (layouts, gallery, admin pages)
      static/            # css/js/htmx vendor file
```

## Data model

Base schema (`db/schema.sql`) already has `users`, `gallery_settings`, `photos`,
`contact_messages`. Additions planned on top of that:

- **`albums`**: `id, title, slug, description, cover_photo_id, sort_order, is_public`.
  Either a `photo_albums` join table (photos in multiple albums) or a simpler
  `album_id` FK directly on `photos` if that's not needed.
- **`photos.sort_order INTEGER`** and **`photos.is_public INTEGER DEFAULT 1`** —
  lets a photo be uploaded and held back before it's published.
- **Generated variants are not DB rows.** They're derived files on disk, named
  by convention from the photo id, regenerable from the original at any time:
  - `derived/{photo_id}_thumb.jpg` (grid thumbnail, ~400px)
  - `derived/{photo_id}_web.jpg` (lightbox size, ~1600px)
- **`sessions`**: `id, user_id, expires_at` — cookie holds only the session id.

## Photo storage layout (external drive)

```
/mnt/photo-drive/
  originals/{photo_id}.{ext}      # untouched upload, full EXIF intact
  derived/
    {photo_id}_thumb.jpg          # grid thumbnail, e.g. 400px
    {photo_id}_web.jpg            # lightbox size, e.g. 1600px
```

On upload: save original → extract EXIF into the existing `photos` columns
(camera, GPS, etc.) → generate the two derived sizes synchronously. A single
Pi can resize one image fast enough that this doesn't need a job queue at this
scale.

## Routes

```
Public:
  GET  /                      gallery home (public albums/photos)
  GET  /albums/:slug          album view
  GET  /contact               contact form
  POST /contact               submit (honeypot field + rate limit by IP)

Admin (session-gated):
  GET  /admin/login, POST /admin/login, POST /admin/logout
  GET  /admin                dashboard
  GET  /admin/photos, POST /admin/photos           list/upload
  POST /admin/photos/:id/delete
  POST /admin/photos/:id/visibility
  GET  /admin/albums, POST /admin/albums, ...
  GET  /admin/messages         view contact submissions
```

## Deployment (Raspberry Pi)

- Cross-compile: `GOOS=linux GOARCH=arm64 go build`.
- `systemd` unit runs the binary, `WorkingDirectory` pointed at the repo, env
  file for secrets (session key, admin password hash), `Restart=on-failure`.
- External drive mounted via `/etc/fstab` using a stable UUID (not `/dev/sda1`);
  the app reads the mount path from config/env and treats "drive not mounted"
  as a startup check, not a silent failure.
- Caddy: automatic HTTPS if exposed publicly, reverse-proxies `/` to Go, serves
  `/derived/*` and gated `/originals/*` straight off disk.

## Implementation order

1. Clean up `go.mod` (drop `mattn/go-sqlite3` and logrus; settle on
   `modernc.org/sqlite` + zerolog).
2. Fix `internal/db/db.go` and `contact/contact.go` so they compile and match
   the schema.
3. Scaffold the album/session schema changes above.
4. Wire up `main.go`, build out the route handlers and templates per the
   sketch above.
5. Photo upload + EXIF + thumbnail pipeline.
6. Admin auth (sessions, login/logout, middleware).
7. Deployment: cross-compile, systemd unit, Caddy config, external drive mount.
