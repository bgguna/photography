PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id                INTEGER PRIMARY KEY,
  email             TEXT NOT NULL UNIQUE,
  password_hash    TEXT NOT NULL,
  role              TEXT NOT NULL,
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS gallery_settings (
  id                      INTEGER PRIMARY KEY,
  gallery_public         INTEGER NOT NULL DEFAULT 1, -- 0/1
  gallery_password_hash  TEXT NULL,
  updated_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS photos (
  id                    INTEGER PRIMARY KEY,
  uploader_user_id    INTEGER NOT NULL,
  original_mime_type   TEXT NOT NULL,
  original_filename     TEXT NOT NULL,
  original_path         TEXT NOT NULL,
  created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

  datetime_original    TEXT NULL,
  camera_make          TEXT NULL,
  camera_model         TEXT NULL,
  gps_lat              REAL NULL,
  gps_lng              REAL NULL,
  iso                   INTEGER NULL,
  aperture              REAL NULL,
  shutter_speed        TEXT NULL,

  FOREIGN KEY (uploader_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_photos_uploader_user_id ON photos(uploader_user_id);
CREATE INDEX IF NOT EXISTS idx_photos_datetime_original ON photos(datetime_original);
CREATE INDEX IF NOT EXISTS idx_photos_camera_make ON photos(camera_make);
CREATE INDEX IF NOT EXISTS idx_photos_camera_model ON photos(camera_model);
CREATE INDEX IF NOT EXISTS idx_photos_gps_lat ON photos(gps_lat);
CREATE INDEX IF NOT EXISTS idx_photos_gps_lng ON photos(gps_lng);
CREATE INDEX IF NOT EXISTS idx_photos_iso ON photos(iso);
CREATE INDEX IF NOT EXISTS idx_photos_aperture ON photos(aperture);

CREATE TABLE IF NOT EXISTS contact_messages (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  email      TEXT NULL,
  message    TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  status     TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','read','archived')),
  read_at    TEXT NULL
);

-- Initialize single-row settings
INSERT OR IGNORE INTO gallery_settings (id, gallery_public, gallery_password_hash, updated_at)
VALUES (1, 1, NULL, strftime('%Y-%m-%dT%H:%M:%fZ','now'));

