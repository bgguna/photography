# 01 — Database schema migrations

## Goal

Extend `db/schema.sql` with the tables/columns the design calls for but that
don't exist yet: per-photo ordering, admin-controlled visibility (hide /
publish), and sessions.

There are no albums (for now): every photo belongs to one single gallery.

## Dependencies

None — this goes first. Everything else queries these columns/tables.

## Scope

- `db/schema.sql`
- `db/gallery.sqlite` (local dev DB — safe to reset, no real data in it yet)

## Decision: no albums yet

All photos live in a single gallery, so there is no `albums` table and no
`album_id` on `photos`. If albums are added later it is a backwards-compatible
migration (new `albums` table + nullable `album_id`, or a join table).

## Decision: visibility

`photos.is_public` (0/1, default 1) is the publish flag. Admins can hide a
photo (`is_public = 0`) and publish it again (`is_public = 1`) without
deleting it. This sits alongside upload and delete. Hidden photos must be
excluded from every public query; only admin queries see them. The column
name `is_public` is kept to match the design doc.

## Steps

1. Add to `db/schema.sql`:
   ```sql
   CREATE TABLE IF NOT EXISTS sessions (
     id          TEXT PRIMARY KEY,
     user_id     INTEGER NOT NULL,
     expires_at  TEXT NOT NULL,
     created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
     FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
   );
   CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
   ```
2. Add two columns to the `photos` `CREATE TABLE` (and matching indexes):
   ```sql
   sort_order  INTEGER NOT NULL DEFAULT 0,
   is_public   INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0,1)),
   ```
   ```sql
   CREATE INDEX IF NOT EXISTS idx_photos_is_public ON photos(is_public);
   ```
3. Regenerate the local dev database: `make reset-db` (there's no real data
   yet, so a drop-and-recreate is simpler than writing a migration runner at
   this stage).
4. Once there's real, non-disposable data in production (i.e. after first
   real deploy), stop editing `schema.sql` in place for future changes — switch
   to numbered migration files (plain `.sql` files applied in order, or a tool
   like `golang-migrate`) so upgrades don't require a destructive reset. Not
   needed yet; note it here so it isn't forgotten later.

## Acceptance criteria

- `make reset-db` applies the schema with no errors.
- `sqlite3 db/gallery.sqlite ".schema photos"` shows `sort_order` and
  `is_public`, and no `album_id`.
- `sqlite3 db/gallery.sqlite ".schema sessions"` shows the new table.
- There is no `albums` table.
