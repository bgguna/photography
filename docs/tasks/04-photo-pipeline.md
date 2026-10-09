# 04 — Photo upload & processing pipeline

## Goal

Implement the `photo` package (currently just `package photo`, empty): upload
an original, extract EXIF into the DB, generate the thumb/web derived sizes,
and store everything per the design doc's
[storage layout](../design.md#photo-storage-layout-external-drive).

## Dependencies

[01-schema-migrations](01-schema-migrations.md),
[02-app-wiring](02-app-wiring.md) (for the storage path config and DB handle).

## Scope

- `server/photo/photo.go`
- new dependencies: an EXIF reader and an image resizer — use pure-Go
  libraries (e.g. `github.com/rwcarlsen/goexif` for EXIF,
  `github.com/disintegration/imaging` for resizing) to keep the cgo-free,
  easy-cross-compile property from the `modernc.org/sqlite` decision.

## Steps

1. **Upload handler**: accept `multipart/form-data`, validate content type
   (JPEG/PNG/etc. — reject anything else) and a sane max size (`gin`'s
   `MaxMultipartMemory` / explicit check) before touching disk.
2. **Insert-then-write ordering**: insert a `photos` row first (to get the
   auto-increment `id`), then write files named by that ID — avoids
   generating IDs separately from the DB and keeps filenames and rows in
   lockstep. If file writes fail after the insert, delete the row (or mark it
   failed) rather than leaving an orphan DB row.
3. **Save original**: write to
   `{PhotoStoragePath}/originals/{id}.{ext}` unmodified (full EXIF intact).
4. **Extract EXIF**: pull `datetime_original`, `camera_make`, `camera_model`,
   `gps_lat`/`gps_lng`, `iso`, `aperture`, `shutter_speed` where present (not
   all cameras/exports include all fields — treat every field as optional,
   `NULL` when absent, no error on a missing tag) and `UPDATE` the row.
5. **Generate derived sizes**: `imaging.Resize` down to ~400px (thumb) and
   ~1600px (web, longest edge, preserve aspect ratio), save to
   `{PhotoStoragePath}/derived/{id}_thumb.jpg` and `{id}_web.jpg`. Re-encode
   as JPEG regardless of original format, for consistent serving/caching.
6. **Delete handler**: remove `originals/{id}.*`, both derived files, and the
   DB row; tolerate already-missing files (log, don't fail the whole
   operation) since manual filesystem cleanup shouldn't desync from the DB.
7. **Listing/query functions** the gallery and admin packages will call:
   list public photos ordered by `sort_order`, list all photos for admin
   (including hidden), get one by ID, and set a photo's `is_public` flag
   (hide / publish).
8. **Image-serving handlers** (see the design doc's
   [Image serving](../design.md#image-serving)): `GET /photos/:id/{thumb,web}`
   returns 404 for a missing *or hidden* photo (identical response for both),
   otherwise `c.File` on the derived file with a short public `Cache-Control`
   max-age. `GET /admin/photos/:id/{thumb,web,original}` serves any photo and
   sits behind the admin middleware (task 03), with `Cache-Control: private,
   no-store`. Look the file path up from the DB id; never build it from
   user-supplied path segments.
9. **Regeneration path**: a function to re-run step 5 from the stored
   original — useful if you change thumb/web target sizes later without
   re-uploading everything.

## Acceptance criteria

- `curl -F file=@photo.jpg localhost:8080/admin/photos` (once wired into
  routes in task 06) creates one DB row and three files on disk.
- A photo with no EXIF data (e.g. a screenshot) uploads successfully with
  EXIF columns left `NULL`, not an error.
- Deleting a photo removes all three files and the DB row.
- A hidden photo's `/photos/:id/thumb` and `/web` return 404 (same body as a
  nonexistent id), including after having been public and fetched before; the
  same URLs work again once it's published. `/admin/photos/:id/original`
  returns 401/redirect when not logged in.
- Thumb and web files are valid, correctly oriented (watch for EXIF
  orientation flag — rotate before resizing if the library doesn't handle it
  automatically) JPEGs at the target sizes.
