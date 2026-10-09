# 05 — Public gallery templates & UI

## Goal

Build the visitor-facing pages: gallery home and photo lightbox (a single gallery, no albums) —
server-rendered, mobile-first, using htmx + Alpine per
[Front-end: htmx + Alpine.js](../design.md#front-end-htmx--alpinejs).

## Dependencies

[04-photo-pipeline](04-photo-pipeline.md) (nothing to display without it),
[02-app-wiring](02-app-wiring.md) (template/static serving already set up).

## Scope

- `server/internal/web/templates/` (layout, home, contact)
- `server/internal/web/static/` (CSS, vendored htmx.min.js / alpine.min.js)
- new `server/internal/gallery` package (query + render logic)

## Steps

1. **Vendor htmx and Alpine**: download pinned versions into
   `internal/web/static/vendor/` and reference them locally — no CDN
   dependency, keeps the Pi deployment fully self-contained and working even
   if internet access to a CDN is briefly down.
2. **Base layout template**: header/nav/footer, viewport meta tag, links the
   vendored JS/CSS. Keep it to one shared `layout.html` that other templates
   extend via Go's `html/template` block/define pattern.
3. **Mobile-first CSS**: CSS Grid for the gallery (`auto-fill`/`minmax` so it
   reflows from one column on a phone to many on desktop without media-query
   breakpoints doing the heavy lifting), `max-width: 100%` on images.
4. **Responsive images**: `<img srcset>` pointing at `/photos/:id/thumb` and
   `/photos/:id/web` (task 04's serving handlers, never raw file paths) so phones don't download the 1600px version for a grid
   thumbnail.
5. **Home route** (`GET /`): list public photos ordered by
   `sort_order` via the `gallery` package, render the grid.
6. **Lightbox**: Alpine `x-data` holding the open/closed state and current
   photo index, keyboard (arrow keys/Escape) and swipe-friendly on mobile;
   loads `/photos/:id/web`, not the original.
7. **Page metadata**: per-page `<title>`, basic Open Graph tags (useful later
   if you ever link out to a photo, including for the Instagram cross-posting
   idea raised earlier).

## Acceptance criteria

- The home page renders correctly at a 400px-wide viewport and at
  desktop width, with no horizontal scroll.
- Lightbox opens on click, closes on Escape/click-outside, keyboard
  navigation moves between photos.
- No requests to any external CDN — check the network tab with internet
  disabled, the page still renders and works.
- A hidden (`is_public = 0`) photo never appears on the home page or in the
  lightbox, and `/photos/:id/thumb|web` for it returns 404 even if the URL was
  known beforehand.
