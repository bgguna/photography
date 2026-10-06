# 07 — Contact form

## Goal

Wire the already-fixed `contact.GetMessages`/`contact.HandleNewMsg` handlers
into real routes with a public-facing form, basic spam defenses, and
non-reload submit feedback.

## Dependencies

[02-app-wiring](02-app-wiring.md) only — the handler logic itself is already
written and compiles (`server/contact/contact.go`); this task is the
remaining route/template/anti-spam work around it.

## Scope

- `server/internal/web/templates/contact.html`
- `server/main.go` (route registration)
- `server/contact/contact.go` (small additions: honeypot + rate limit checks)

## Steps

1. **Register routes**: `GET /contact` renders the form template,
   `POST /contact` calls `contact.HandleNewMsg(db)` (already takes a
   `*sql.DB`, just needs wiring per task 02's route skeleton).
2. **Honeypot field**: add a hidden input (e.g. `name="website"`) that real
   visitors never fill in and bots often do; in `HandleNewMsg`, silently
   accept-but-discard (return success without inserting) any submission where
   it's non-empty — don't tell the bot it was caught.
3. **Rate limiting**: a simple per-IP limit (in-memory, e.g. N submissions per
   hour) on `POST /contact` — doesn't need to be sophisticated, just enough to
   stop a flood.
4. **Validation**: server-side — `name` and `message` non-empty, `email` (if
   provided) matches a basic email pattern. Client-side `required` attributes
   as a first line of feedback, but don't rely on them alone.
5. **Submit feedback**: `hx-post` on the form, swap in a success/error message
   in place of the form (or above it) instead of a full page redirect.
6. **Optional — email notification**: SMTP alert to the admin on new message.
   Flagged optional/future since it adds a credential (SMTP password) to
   manage; the admin messages view in task 06 is sufficient for v1.

## Acceptance criteria

- Submitting the form with valid data stores a row in `contact_messages` and
  shows a success message without a full page reload.
- Submitting with the honeypot field filled returns a success message but
  does **not** create a DB row.
- More than the configured rate limit from one IP in the window gets
  rejected with a clear (non-leaky) error.
- Messages submitted here appear in `GET /admin/messages` (task 06).
