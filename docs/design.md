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
[Browser] → [Caddy: TLS + reverse proxy]
                    │
                    ▼
            [Go/Gin API server]  ← systemd service
                    │
         ┌──────────┼──────────────┐
         ▼                         ▼
   [SQLite DB on SD card]   [External drive: originals + generated web/thumb sizes]
```

Caddy terminates TLS and reverse-proxies everything to the Go process. Image
bytes are **also served by Go**, never directly off disk by Caddy: every image
request is checked against `photos.is_public` (or an admin session) first, so
a hidden photo can't be fetched by guessing or reusing its URL. Files are sent
with `c.File` (sendfile), so the cost of going through Go is small at this
scale.

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
- **More gallery control** (reordering, hide/publish): more admin
  screens following the same CRUD + htmx pattern already planned.
- **Instagram cross-posting**: a backend integration (an admin "publish"
  action calls Meta's Graph API), unrelated to the front-end layer entirely —
  requires an Instagram Business account, Facebook App, and a server-held
  access token, but no client-side framework decision.

## Package layout

```
server/
  main.go              # wiring: config, db, router, public routes, graceful shutdown
  admin_routes.go      # /admin routes (login, photos, messages, about)
  cmd/
    createadmin/       # creates an admin user, applies the schema
    backfill-exif/     # re-reads EXIF from stored originals (one-off fixes)
  contact/             # contact form handler + storage
  photo/               # upload, EXIF extraction, derived sizes, ordering, visibility
  internal/
    config/            # settings from the environment
    db/                # db.Open()
    auth/              # sessions, login/logout, CSRF, rate limiting
    admin/             # admin queries (counts, contact messages)
    gallery/           # public photo list (visible photos only)
    about/             # About page text: save, read, split into paragraphs
    web/
      templates/       # .html templates: public pages, shared header, admin pages
      static/          # theme.css, theme.js, htmx and Alpine vendor files
```

## Data model

Base schema (`db/schema.sql`) already has `users`, `gallery_settings`, `photos`,
`contact_messages`. Additions planned on top of that:

- **No albums (for now).** All photos live in a single gallery. Albums can be
  added later as a new table plus a nullable `album_id` on `photos`.
- **`photos.sort_order INTEGER`** and **`photos.is_public INTEGER DEFAULT 1`** —
  lets a photo be uploaded and held back, and lets the admin hide a published
  photo and publish it again without deleting it. Hidden photos never appear
  in public queries.
- **Generated variants are not DB rows.** They're derived files on disk, named
  by convention from the photo id, regenerable from the original at any time:
  - `derived/{photo_id}_thumb.jpg` (grid thumbnail, ~400px)
  - `derived/{photo_id}_web.jpg` (lightbox size, ~1600px)
- **`sessions`**: `id, user_id, expires_at` — cookie holds only the session id.
- **`about_page`**: a single row (`id = 1`) holding the About page text. Plain
  text with blank lines between paragraphs; it is HTML-escaped on output.

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
  GET  /                      home: gallery of all public photos
  GET  /about                 About page
  GET  /photos/:id/thumb      thumbnail (404 if hidden or missing)
  GET  /photos/:id/web        lightbox size (404 if hidden or missing)
  GET  /contact               contact form
  POST /contact               submit (email required, honeypot field, rate limit by IP)

Admin (session-gated, CSRF token on every change):
  GET  /admin/login, POST /admin/login, POST /admin/logout
  GET  /admin                dashboard
  GET  /admin/photos, POST /admin/photos           list/upload
  GET  /admin/photos/:id/thumb|web|original   any photo, hidden included
  DELETE /admin/photos/:id
  POST /admin/photos/:id/visibility    hide / publish
  POST /admin/photos/:id/move          reorder
  GET  /admin/messages         view contact submissions
  POST /admin/messages/:id/read|archive, DELETE /admin/messages/:id
  GET, POST /admin/about       edit the About page text

The full table, with the rate limits, is in [server/readme.md](../server/readme.md#routes).
```

## Image serving

The files on the external drive are never exposed by path. The disk layout is
an implementation detail; URLs use the photo id and go through Go:

- `GET /photos/:id/thumb` and `/web`: look up the row; if it doesn't exist or
  `is_public = 0`, respond with the same 404 either way (don't reveal that a
  hidden photo exists). Otherwise stream the derived file.
- `GET /admin/photos/:id/{thumb,web,original}`: behind the admin auth
  middleware, serves any photo including hidden ones. Originals are
  admin-only.
- Responses for public photos use a short `Cache-Control` max-age (minutes,
  not days), so a photo that gets hidden stops being served by browser/proxy
  caches soon after. Admin responses use `Cache-Control: private, no-store`.

## Deployment (Raspberry Pi)

- Cross-compile: `GOOS=linux GOARCH=arm64 go build`.
- `systemd` unit runs the binary, `WorkingDirectory` pointed at the repo, env
  file for secrets (the session secret), `Restart=on-failure`. The admin
  password is stored hashed in the database, set with `cmd/createadmin`.
- External drive mounted via `/etc/fstab` using a stable UUID (not `/dev/sda1`);
  the app reads the mount path from config/env and treats "drive not mounted"
  as a startup check, not a silent failure.
- Caddy: automatic HTTPS if exposed publicly, reverse-proxies everything
  to Go. It does not serve photo files off disk (see Image serving).

## Implementation status

The implementation plans are in [tasks/](tasks/README.md), and their status is
tracked in [plan.md](plan.md). Tasks 01–08 (schema, app wiring, admin auth,
photo pipeline, public gallery, admin UI, contact form, Raspberry Pi
deployment) are marked done, except task 05 (public gallery), which plan.md
still lists as in progress. Tasks 09–11 and 14–16 (site title, navigation, the
About page, contact form changes, EXIF fixes) are done. Still open: the
Cloudflare Turnstile check on the contact form (task 12) and the decision on
site-wide Cloudflare protection (task 13).
