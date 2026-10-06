# 02 — App bootstrap & wiring

## Goal

Turn `main.go` from a stub into a real entry point: load config, open the DB,
stand up the Gin router with the route skeleton from the design doc, serve
static assets, and shut down cleanly.

## Dependencies

[01-schema-migrations](01-schema-migrations.md) (needs the DB to open against
the final schema).

## Scope

- `server/main.go`
- new `server/internal/config` package
- new `server/internal/web/static` (empty dir for now, vendored htmx/Alpine
  land here in task 05)

## Steps

1. **Config package**: a `config.Load()` that reads env vars (already using
   `godotenv` in `main.go`) into a typed struct:
   - `Port` (default `8080`)
   - `DBPath` (default `../db/gallery.sqlite`)
   - `PhotoStoragePath` (the external drive mount point, e.g.
     `/mnt/photo-drive`)
   - `SessionSecret` (required in production; fail fast if missing when
     `MODE_ENV=production`)
   - `Mode` (reuse existing `MODE_ENV` local/development/production logic
     already in `main.go`'s `init()`)
2. **Startup checks**: verify `PhotoStoragePath` exists and is writable before
   serving traffic — fail fast with a clear log message rather than failing
   silently on first upload. This is the "drive not mounted" check from the
   design doc.
3. **DB**: open via `internal/db.Open(cfg.DBPath)`, pass the `*sql.DB` into
   whatever wires the route handlers (either directly or via `db.NewRepos`).
4. **Router**: `gin.New()` + `gin.Recovery()` + a small zerolog request-logging
   middleware (method, path, status, latency) — don't pull in `gin.Default()`'s
   built-in logger since it bypasses zerolog.
5. **Static files**: `router.Static("/static", "./internal/web/static")`.
6. **Route skeleton**: register every route from the design doc's
   [Routes](../design.md#routes) section now, even if most handlers are
   placeholders (`c.String(http.StatusNotImplemented, "TODO")`) to be filled
   in by tasks 03–07. This gives later tasks a fixed contract to implement
   against instead of guessing paths.
7. **Health check**: `GET /healthz` returning 200 — useful for systemd/manual
   checks in task 08.
8. **Graceful shutdown**: run the server via `http.Server` + goroutine, listen
   for `SIGINT`/`SIGTERM`, call `Shutdown(ctx)` with a timeout, close the DB
   after the server stops accepting requests.

## Acceptance criteria

- `make build && make run` starts the server and logs the configured port.
- `curl localhost:8080/healthz` returns 200.
- `curl` against every route in the design doc's route table gets a response
  (even if "not implemented" for most), confirming the route table is fully
  wired.
- `Ctrl+C` shuts down without a panic or hung process.
