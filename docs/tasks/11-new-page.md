# 11 — New page between "Home" and "Contact"

## Goal

Add a third public page, linked in the nav between "Home" and "Contact".

## Dependencies

Do after [09](09-site-title.md) and [10](10-rename-gallery-tab.md), which
edit the same nav markup.

## Open questions (resolve before starting)

- **What is the page?** Its name, URL slug and content aren't decided yet. An
  "About" page (bio, gear, portrait photo) is the usual choice for a
  portfolio, but this task doesn't assume that.
- **Static or admin-editable?** A hard-coded template is the simplest option.
  If the content should be editable from the admin UI, that needs a DB table
  and an admin form, so it becomes its own larger task.

## Scope (assuming a static page)

- New template `server/internal/web/templates/<page>.html`, built on
  `layout.html` like `home.html` is
- `server/main.go`: register `GET /<slug>`
- `layout.html` and `contact.html`: add the nav link

## Steps

1. **Optional prerequisite: share the header.** The header and nav are
   currently duplicated in `layout.html` and in the standalone
   `contact.html`, so every nav change has to be made twice. Moving
   `contact.html` onto `layout.html`, or pulling the header into a shared
   `{{define "header"}}` partial, would make this task and any future nav
   edits one-place changes.
2. **Template**: create the page with `title`, `og_tags` and `content` blocks,
   following `home.html`.
3. **Route**: add `router.GET("/<slug>", ...)` next to the existing `/` and
   `/contact` handlers in `main.go`.
4. **Nav**: insert `<a href="/<slug>">…</a>` between Home and Contact in
   every copy of the header.
5. **Theme**: use the existing CSS variables (`--primary-text`,
   `--muted-text`, `--border-color` and so on) so the page works in light and
   dark mode without extra rules.
6. **Test**: add a route test like the existing `/contact` one: 200 status,
   expected heading present.

## Acceptance criteria

- The nav order is Home → [new page] → Contact on every public page.
- `GET /<slug>` returns 200 and renders correctly in light and dark mode and
  at phone width.
- `go test ./...` passes, including the new route test.
