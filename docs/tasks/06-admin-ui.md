# 06 — Admin UI

## Goal

Build the admin-only pages for publishing photos and managing the gallery:
upload, delete, hide/publish, reordering, and contact messages — htmx-driven
partial updates, no page-reload-per-action.

## Dependencies

[03-admin-auth](03-admin-auth.md) (these routes sit behind the auth
middleware), [04-photo-pipeline](04-photo-pipeline.md) (nothing to manage
without it).

## Scope

- `server/internal/web/templates/admin/`
- handlers in a new `server/internal/admin` package (or alongside `gallery` —
  your call when you get there, either is fine at this size)

## Steps

1. **Login page** (`GET /admin/login`): simple form, error message slot for
   failed attempts.
2. **Dashboard** (`GET /admin`): photo count, unread contact message count,
   links into the sections below.
3. **Photo upload** (`GET`/`POST /admin/photos`): a form posting to the
   pipeline from task 04; htmx `hx-post` with `hx-encoding="multipart/form-data"`
   and an `hx-indicator` for upload progress feedback, since a Pi + external
   drive write isn't instant.
4. **Photo list**: thumbnail grid (images from
   `/admin/photos/:id/thumb`, so hidden photos still display for the admin),
   each with:
   - hide/publish toggle (`hx-post /admin/photos/:id/visibility`, flips
     `is_public` and returns the updated row fragment; hidden photos are
     visibly marked in the admin list)
   - delete with a confirm step (`hx-confirm` is built into htmx — use it
     rather than a custom JS `confirm()`)
   - reordering: start with simple up/down buttons posting to
     `/admin/photos/:id/move` rather than a drag-and-drop JS library —
     matches the "don't reach for more tooling than the problem needs"
     reasoning behind picking htmx in the first place; revisit only if it's
     genuinely too slow to use.
5. **Contact messages** (`GET /admin/messages`): list from
   `contact.GetMessages`, mark read/archived via `hx-post`.
6. **CSRF protection**: every state-changing `POST` here needs a CSRF token
   (session-cookie auth is vulnerable to cross-site request forgery unlike
   token-header auth) — a hidden field in each form/htmx request, validated
   server-side against a value tied to the session.

## Acceptance criteria

- Can log in, upload a photo, see it in the admin list, hide it and see it vanish
  from the live public gallery (task 05), publish it again and see it return,
  without a full page reload for any of these steps.
- Deleting a photo removes it from both admin and public views.
- A `POST` to any admin mutation route without a valid CSRF token is
  rejected.
- Contact messages show up here after being submitted via the public form
  (task 07).
