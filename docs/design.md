# Design

Self-hosted photography portfolio: public gallery + contact form, admin mode for
publishing photos, deployed on a Raspberry Pi with photos on an external drive.

## Decisions

- **Client**: Go `html/template` (server-rendered), enhanced with htmx + Alpine.js
  for interactivity (lightbox, async upload with progress, drag-to-reorder in
  admin). No separate SPA/build pipeline — see
  [Front-end: htmx + Alpine.js](#front-end-htmx--alpinejs) for the reasoning.
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

## Front-end: htmx + Alpine.js

**What they are:**
- [htmx](https://htmx.org/) lets server responses drive the page: HTML
  attributes (`hx-post`, `hx-target`, `hx-swap`, ...) trigger an AJAX request
  and splice the returned HTML fragment into the DOM. The Go handler renders a
  small chunk of HTML instead of JSON, so there's no client-side state
  management or separate API layer.
- [Alpine.js](https://alpinejs.dev/) handles small bits of pure client-side
  state that don't need a server round-trip — lightbox open/close, a dropdown
  toggle — declared inline in HTML (`x-data`, `x-show`, ...).
- Both ship as a single `<script>` tag (no build step, no bundler, no
  `node_modules`), which keeps the whole app a single Go binary + templates,
  matching the deployment model below.

**Why not Angular / React / Vue / Svelte:**
- **Problem fit.** This app is read-heavy (browse a gallery, view a photo) with
  occasional write interactions (admin CRUD, a like, a contact form). None of
  that needs client-side routing, component state trees, or a virtual DOM —
  it's exactly what server-rendered HTML + partial swaps was built for.
- **Perceived speed isn't actually better with a SPA here.** "Responsive" (the
  layout adapting to phone vs. desktop) is a CSS concern, orthogonal to
  framework choice. "Feels fast" is about per-interaction latency: htmx ships a
  small HTML fragment per action, a SPA ships a JS bundle up front (parsed and
  executed before anything renders) plus a JSON round-trip plus a client-side
  re-render. For a photo gallery, the real bottleneck is image bytes over the
  network either way — a heavier framework doesn't fix that.
- **Operational cost on a Raspberry Pi.** A SPA needs a build pipeline
  (webpack/vite, `node_modules`, a `dist/` to deploy) and a real JSON API with
  its own auth story (tokens, CORS) running alongside it. htmx/Alpine need
  neither — same-origin session cookies work as-is, and there's nothing to
  build beyond the Go binary.
- **No native mobile app planned.** The strongest reason to stand up a full
  JSON API (and therefore a case for React/Vue-style client) is sharing that
  API with a native app. Confirmed this isn't on the roadmap — viewers only
  consume content through the web pages — so that cost isn't worth paying
  pre-emptively.
- **Where a heavier framework *would* win:** complex, persistent client-side
  state (offline-first browsing, rich drag-and-drop with multi-step undo) or
  an actual native app sharing the backend's API. If priorities change later,
  a JSON API can be added underneath the existing Go handlers without
  discarding the backend — but nothing here calls for designing that in now.

**Planned feature fit** (raised when scoping this out — view counts, likes,
deeper gallery control, Instagram cross-posting): all of these fit the
htmx/template model without any architecture change:
- **View counts / likes**: a counter column on `photos` + a small
  `POST /photos/:id/like` endpoint returning the updated count as an HTML
  fragment.
- **More gallery control** (reordering, albums, visibility): more admin
  screens following the same CRUD + htmx pattern already planned.
- **Instagram cross-posting**: a backend integration (an admin "publish"
  action calls Meta's Graph API), unrelated to the front-end layer entirely —
  requires an Instagram Business account, Facebook App, and a server-held
  access token, but no client-side framework decision.

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

1. ~~Clean up `go.mod` (drop `mattn/go-sqlite3` and logrus; settle on
   `modernc.org/sqlite` + zerolog).~~ Done.
2. ~~Fix `internal/db/db.go` and `contact/contact.go` so they compile and
   match the schema.~~ Done.
3. Everything remaining is broken out into individual implementation plans in
   [tasks/](tasks/README.md): schema migrations, app wiring, admin auth, the
   photo pipeline, the public gallery, the admin UI, the contact form, and
   Raspberry Pi deployment — in that dependency order.
