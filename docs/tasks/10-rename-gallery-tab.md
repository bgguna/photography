# 10 — Rename the "Gallery" tab to "Home"

## Goal

The first nav link reads "Home" instead of "Gallery". It still points at `/`.

## Dependencies

Do after [09](09-site-title.md), which edits the same header markup.

## Scope

- `server/internal/web/templates/layout.html`: nav link
- `server/internal/web/templates/contact.html`: its own copy of the nav
- `server/internal/web/templates/home.html`: `title` block, currently
  "Gallery — Photography"
- Any tests that assert on the link text (search `*_test.go` for `Gallery`)

## Steps

1. Change `<a href="/">Gallery</a>` to `<a href="/">Home</a>` in both
   templates.
2. Update the home page `<title>` so it no longer says "Gallery". Coordinate
   the wording with task 09.
3. Optional: the admin side has "View gallery" (`admin/layout.html`) and "Back
   to Gallery" (`admin/login.html`). These describe the public site rather
   than the tab, so they can stay as they are or become "View site" / "Back to
   site". Pick one and keep it consistent.
4. Update any tests that match on the old text.

## Acceptance criteria

- The nav shows "Home" on every public page, and it links to `/`.
- No public page still uses "Gallery" as the tab label.
- `go test ./...` passes.
