# 15 — Make email a required field on the contact form

## Goal

A contact message can't be sent without a valid email address, so every
message can be replied to.

## Current behavior

- `contact.html`: the email input has no `required` attribute, and the label
  reads "Email" without the `*`.
- `contact/contact.go` (`HandleNewMsg`): email is optional. It is only
  validated by `isValidEmail` when it's non-empty.

## Dependencies

None. Coordinate with [12](12-contact-turnstile.md) and
[14](14-contact-spacing.md), which also edit `contact.html`.

## Scope

- `server/internal/web/templates/contact.html`
- `server/contact/contact.go`
- `server/contact/contact_test.go` and any integration tests that post the
  form without an email

## Steps

1. **Client side**: add `required` to the email input and change the label to
   "Email *", matching Name and Message.
2. **Server side** (the actual guarantee): in `HandleNewMsg`, reject an empty
   email with `400` (e.g. "Email is required") before the format check. Keep
   the existing `isValidEmail` check for non-empty values.
3. **Error feedback (optional)**: `handleContactResponse` currently shows the
   same generic "Something went wrong" banner for every non-200 response.
   Consider showing the server's validation message, so a visitor who gets
   past the browser check knows what to fix.
4. **Existing data**: older rows in `contact_messages` may have an empty
   email. That's fine. No migration is needed, and the admin messages view
   should keep handling them.
5. **Tests**: add a case for a missing email returning 400 with no row
   stored. Update any test that currently posts without an email and expects
   success.

## Acceptance criteria

- The browser blocks a submit with an empty email field.
- A direct POST with an empty or missing email returns 400 and stores no row.
- An invalid email format is still rejected, as it is now.
- `go test ./...` passes.
