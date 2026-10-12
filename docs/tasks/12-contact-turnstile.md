# 12 — Cloudflare "Verify you are human" check on the contact form

## Goal

Require a passed Cloudflare Turnstile check before a contact message is
accepted. Turnstile is Cloudflare's "Verify you are human" widget. The
server rejects any submission without a valid token.

## Background

- Turnstile is free. It does **not** require the site to be proxied through
  Cloudflare. You create a widget in the Cloudflare dashboard (Turnstile →
  Add widget) for your hostname and get a **site key** (public, goes in the
  page) and a **secret key** (server only).
- The widget runs in the page. Usually it passes invisibly and only sometimes
  asks the visitor to click. On success it writes a token into a hidden
  `cf-turnstile-response` input inside the form.
- The server must verify that token by POSTing it, along with the secret key,
  to `https://challenges.cloudflare.com/turnstile/v0/siteverify`. A token is
  **single-use** and **expires after 5 minutes**.
- Cloudflare publishes test keys that always pass or always fail, for local
  development and automated tests. Use them rather than real keys outside
  production.

Check these details against the current Cloudflare docs when implementing.

## Dependencies

None. It builds on the existing contact form ([07](07-contact-form.md)).
Coordinate with [14](14-contact-spacing.md) and
[15](15-contact-email-required.md), which also edit `contact.html`.

## Scope

- `server/internal/config/config.go`: new `TURNSTILE_SITE_KEY` and
  `TURNSTILE_SECRET_KEY` settings
- `server/contact/contact.go`: token verification in `HandleNewMsg`
- `server/internal/web/templates/contact.html`: widget markup and script
- `server/main.go`: pass the site key to the contact template
- `.env`, `.env.test`, `deploy/` docs: document the new variables

## Steps

1. **Config**: add both keys to `config.Load()`. In production, refuse to
   start if either is missing. In development, default to Cloudflare's
   always-pass test keys.
2. **Widget**: in `contact.html`, load
   `https://challenges.cloudflare.com/turnstile/v0/api.js` and place
   `<div class="cf-turnstile" data-sitekey="{{.TurnstileSiteKey}}" data-theme="auto">`
   inside the form, above the submit button. `data-theme="auto"` follows the
   system theme. Check whether it should instead follow the site's manual
   dark-mode toggle (`data-theme` on `<html>`).
3. **Server verification**: in `HandleNewMsg`, after the honeypot check and
   before validation and insert:
   - read `cf-turnstile-response` from the form and reject an empty value
   - POST `secret`, `response` and `remoteip` (`c.ClientIP()`) to
     `siteverify` with a short timeout (about 5s), and reject unless
     `success` is true
   - if the call to Cloudflare fails, reject the submission. Failing closed
     is the safer default for a spam check, and the visitor can retry.
   - put the verifier behind a small interface so tests can stub it instead
     of calling Cloudflare.
4. **Client IP behind Caddy**: make sure Gin's trusted proxies are set so
   `c.ClientIP()` returns the visitor's IP and not Caddy's. The rate limiter
   needs this too.
5. **Retry after failure**: tokens are single-use, so after any failed
   submission (validation error, rate limit, server error),
   `handleContactResponse` must call `turnstile.reset()` so the visitor gets
   a fresh token.
6. **Keep the existing defenses**: the honeypot and per-IP rate limit stay.
   Turnstile is an extra layer, not a replacement.
7. **Tests**: missing token → 400, failed verification → 400, verification
   error → rejected, valid token → stored. Use the stubbed verifier.

## Acceptance criteria

- The contact page shows the Turnstile widget, and it fits light and dark
  mode.
- A POST to `/contact` without a valid token is rejected and stores no row,
  even when sent directly with `curl`, bypassing the page.
- A real visitor can send a message, and can retry after an error without
  reloading the page.
- The keys come from the environment, and no secret is committed.
- `go test ./...` passes without network access.
