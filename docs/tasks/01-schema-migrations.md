# 01 — Database schema migrations

## Goal

Extend `db/schema.sql` with the tables/columns the design calls for but that
don't exist yet: albums, per-photo ordering/visibility, and sessions.

## Dependencies

None — this goes first. Everything else queries these columns/tables.

## Scope

- `db/schema.sql`
- `db/gallery.sqlite` (local dev DB — safe to reset, no real data in it yet)

## Decision: FK vs. join table for albums

Design doc left this open. Recommend starting with a simple `album_id` FK
directly on `photos` (one album per photo) rather than a `photo_albums` join
table — it's the common case for a personal portfolio, it's less to query/join
for every gallery page render, and it's a backwards-compatible migration to a
join table later if you ever need a photo in multiple albums (add the join
table, backfill from `album_id`, drop the column).

## Steps

1. Add to `db/schema.sql`:
   ```sql
   CREATE TABLE IF NOT EXISTS albums (
     id              INTEGER PRIMARY KEY,
     title           TEXT NOT NULL,
     slug            TEXT NOT NULL UNIQUE,
     description     TEXT NULL,
     cover_photo_id  INTEGER NULL,
     sort_order      INTEGER NOT NULL DEFAULT 0,
     is_public       INTEGER NOT NULL DEFAULT 1,
     created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
     FOREIGN KEY (cover_photo_id) REFERENCES photos(id) ON DELETE SET NULL
   );

   CREATE TABLE IF NOT EXISTS sessions (
     id          TEXT PRIMARY KEY,
     user_id     INTEGER NOT NULL,
     expires_at  TEXT NOT NULL,
     created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
     FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
   );
   CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
   ```
2. Add to the existing `photos` table:
   ```sql
   ALTER TABLE photos ADD COLUMN album_id INTEGER NULL REFERENCES albums(id) ON DELETE SET NULL;
   ALTER TABLE photos ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
   ALTER TABLE photos ADD COLUMN is_public INTEGER NOT NULL DEFAULT 1;
   CREATE INDEX IF NOT EXISTS idx_photos_album_id ON photos(album_id);
   CREATE INDEX IF NOT EXISTS idx_photos_is_public ON photos(is_public);
   ```
   (SQLite doesn't support adding a column with a non-constant default *and* a
   foreign key in one `ALTER TABLE ADD COLUMN ... REFERENCES` reliably across
   versions — if `modernc.org/sqlite` rejects the inline `REFERENCES` on
   `ALTER TABLE`, add the column plain and create the FK relationship only in
   the fresh-create `CREATE TABLE` path; verify by running `make reset-db`.)
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
- `sqlite3 db/gallery.sqlite ".schema photos"` shows `album_id`, `sort_order`,
  `is_public`.
- `sqlite3 db/gallery.sqlite ".schema albums"` and `".schema sessions"` show
  the new tables.
