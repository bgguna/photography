# 03 — Admin authentication

## Goal

Session-cookie auth for the single admin user, per the design doc's decision
to avoid JWT.

## Dependencies

[01-schema-migrations](01-schema-migrations.md) (needs the `sessions` table),
[02-app-wiring](02-app-wiring.md) (needs the router/DB wiring to hang routes
and middleware off).

## Scope

- new `server/internal/auth` package
- a one-off admin-creation path (CLI flag or small helper command — this app
  has no public signup, so there's no "create admin" route to build)

## Steps

1. **Password hashing**: use `golang.org/x/crypto/bcrypt` (already a
   transitive dependency via go.mod) for `users.password_hash`.
2. **Admin bootstrap**: add a `make create-admin EMAIL=... PASSWORD=...`
   target that runs a small Go program (`cmd/createadmin/main.go` or a
   `-create-admin` flag on the main binary) to insert the single admin row —
   there's exactly one admin, so a full signup flow would be wasted work.
3. **Login handler** (`POST /admin/login`):
   - look up `users` by email, `bcrypt.CompareHashAndPassword` against
     `password_hash`
   - on success: generate a random session ID (`crypto/rand`, not
     `math/rand`), insert into `sessions` with an expiry (e.g. 7 days), set an
     `HttpOnly`, `Secure` (in production), `SameSite=Lax` cookie containing
     only the session ID
   - on failure: generic "invalid credentials" (don't reveal whether the
     email existed)
4. **Logout handler** (`POST /admin/logout`): delete the session row, clear
   the cookie.
5. **Auth middleware**: reads the cookie, looks up the session, checks
   `expires_at`, loads the user, stores it on `gin.Context`; redirects to
   `/admin/login` (or 401 for htmx requests — check the `HX-Request` header to
   decide between a redirect and a fragment) when missing/expired. Apply to
   every `/admin/*` route except `/admin/login`.
6. **Session cleanup**: a `DELETE FROM sessions WHERE expires_at < now` run
   lazily on each login, or a simple ticker goroutine — no need for a cron job
   at this scale.
7. **Brute-force mitigation**: basic per-IP rate limit on `POST /admin/login`
   (in-memory, e.g. a small token bucket keyed by IP) since this endpoint will
   be internet-reachable per the deployment plan.

## Acceptance criteria

- `make create-admin EMAIL=you@example.com PASSWORD=...` creates a working
  login.
- Logging in sets a cookie and redirects to `/admin`; visiting any `/admin/*`
  route without that cookie redirects to `/admin/login`.
- Logging out invalidates the session (same cookie no longer works).
- An expired session is rejected and cleaned up.
- Repeated failed logins from one IP get throttled.
