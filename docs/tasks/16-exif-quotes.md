# 16 — Remove the extra quotes from displayed EXIF data

## Goal

The lightbox's camera-settings panel shows values without stray quotes, for
example `NIKON Z 6` instead of `"NIKON Z 6"`.

## Cause (confirmed)

The extraction code is already correct. Commit `4fd7ee1` ("fix EXIF
extraction") changed `extractEXIF` in `server/photo/photo.go` from
`tag.String()` to `tag.StringVal()` / `tag.Rat()`. `tag.String()` returns
the value JSON-quoted.

Photos uploaded **before** that fix still have the quoted values stored in
the database. For example, the local DB has:

| column | stored value |
|---|---|
| `camera_make` | `"NIKON CORPORATION"` |
| `camera_model` | `"NIKON Z 6"` |
| `shutter_speed` | `"10/1600"` |

The template prints the stored value as-is, so the quotes reach the page. The
shutter speed also isn't reduced: `10/1600` should read `1/160`. New uploads
are fine.

## Dependencies

None.

## Scope

- A one-off data fix for existing `photos` rows, run locally and on the Pi
- Optionally, a defensive cleanup in `server/photo/photo.go`

## Steps

1. **Pick the backfill approach.**
   - **Preferred: re-extract from the originals.** Write a small one-off
     command, alongside `cmd/createadmin`, that runs `extractEXIF` on each
     photo's original file and updates its EXIF columns. This fixes the
     quotes and the unreduced shutter speed in one pass, using the same code
     as new uploads.
   - **Fallback: SQL only.** Strip the surrounding `"` from `camera_make`,
     `camera_model` and `shutter_speed`. This does **not** reduce `10/1600`,
     so it would need extra handling.
2. **Back up first**: run `make backup-db` before touching the Pi database.
3. **Make it idempotent**: rows that are already clean shouldn't change if
   the fix runs twice.
4. **Optional hardening**: trim whitespace and trailing NUL bytes from
   `StringVal()` results. Some cameras pad these fields.
5. **Tests**: cover the backfill on a row with quoted values, and on a row
   that's already clean.

## Acceptance criteria

- No EXIF value in the lightbox is wrapped in quotes, for old or new photos.
- Shutter speeds show in reduced form, e.g. `1/160 s`.
- Running the fix a second time changes nothing.
- The Pi database is backed up before the fix is run there.
