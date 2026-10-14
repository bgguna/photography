# 11 — New page between "Home" and "Contact"

## Goal

Add a third public page, linked in the nav between "Home" and "Contact".

## Dependencies

Do after [09](09-site-title.md) and [10](10-rename-gallery-tab.md), which
edit the same nav markup.

## Decisions

- **Page:** "About", at `/about`.
- **Content:** admin-editable. The text is stored in a new `about_page` table
  (single row) and edited from a new "About" tab in the admin UI.
- **Header:** shared first. The header and nav markup now live in one
  `site_header` partial used by the gallery and contact pages.

## Implemented as

- `db/schema.sql`: `about_page` table. Existing databases pick it up with
  `make init-db`, which is idempotent.
- `server/internal/about/`: `Get`, `Save` (10,000-character limit) and
  `Paragraphs` (splits on blank lines). Text is HTML-escaped on output, and
  single line breaks are kept.
- `server/main.go`: `GET /about`. Pages now receive `Title`, `Description` and
  `Page` from their route, and `layout.html` picks the page content by `Page`.
  This replaced the per-page `title`, `og_tags` and `content` blocks, which
  two pages can't both define.
- `server/admin_routes.go`: `GET /admin/about` (form) and `POST /admin/about`
  (save, CSRF-protected).
- Tests: `internal/about/about_test.go`, plus routes, escaping and the admin
  save flow in `main_test.go` and `admin_flow_test.go`.

## Steps

1. [x] Share the header as a `site_header` partial.
2. [x] Template: `about.html`, rendered through `layout.html`.
3. [x] Route: `GET /about` in `main.go`.
4. [x] Nav: Home → About → Contact on every public page.
5. [x] Theme: uses the existing CSS variables.
6. [x] Tests: route, escaping, empty state, and the admin save flow.

## Acceptance criteria

- The nav order is Home → [new page] → Contact on every public page.
- `GET /<slug>` returns 200 and renders correctly in light and dark mode and
  at phone width.
- `go test ./...` passes, including the new route test.
