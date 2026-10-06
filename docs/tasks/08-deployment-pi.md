# 08 — Raspberry Pi deployment

## Goal

Get the app running persistently on the Raspberry Pi: cross-compiled binary,
systemd service, Caddy in front, external drive mounted, and a backup plan —
per the design doc's [Deployment](../design.md#deployment-raspberry-pi)
section.

## Dependencies

Everything else — deploying before the app does anything useful isn't worth
doing. In practice you'll likely want to do a first rough deploy once
[02-app-wiring](02-app-wiring.md) and [03-admin-auth](03-admin-auth.md) work,
then redeploy as later tasks land.

## Scope

- new `deploy/` directory: systemd unit, Caddyfile
- Pi system configuration (fstab, user accounts) — documented here, applied
  directly on the Pi, not committed as code

## Steps

1. **Confirm Pi architecture**: `uname -m` on the Pi — `aarch64` (64-bit OS,
   most current Pi 3/4/5 images) means `GOARCH=arm64`; `armv7l` (32-bit OS)
   means `GOARCH=arm` + `GOARM=7`. Don't assume; check before building.
2. **Cross-compile**:
   ```sh
   GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/server-arm64 ./server
   ```
   (confirm `modernc.org/sqlite` cross-compiles cleanly with `CGO_ENABLED=0`
   — it should, that's the whole point of having chosen it over
   `mattn/go-sqlite3`.)
3. **Service account**: run the app as a dedicated non-root user
   (`useradd --system photography`), not `pi`/root — standard least-privilege
   practice for anything internet-facing.
4. **External drive mount**: find the drive's UUID (`blkid`), add an
   `/etc/fstab` entry keyed by UUID (not `/dev/sda1`, which can shift across
   reboots/USB re-enumeration), mount at e.g. `/mnt/photo-drive`, set
   ownership so the service account can write to `originals/`/`derived/`.
5. **systemd unit** (`deploy/photography.service`):
   ```ini
   [Unit]
   Description=Photography portfolio server
   After=network.target mnt-photo\x2ddrive.mount
   Requires=mnt-photo\x2ddrive.mount

   [Service]
   Type=simple
   User=photography
   WorkingDirectory=/opt/photography
   EnvironmentFile=/opt/photography/.env
   ExecStart=/opt/photography/bin/server-arm64
   Restart=on-failure
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
   The `Requires=`/`After=` on the drive's mount unit means systemd won't even
   try to start the app before the drive is mounted — a cleaner version of the
   "fail fast if the drive isn't there" check from task 02.
6. **Caddyfile** (`deploy/Caddyfile`):
   ```
   your-domain.example {
       reverse_proxy 127.0.0.1:8080
       handle_path /derived/* {
           root * /mnt/photo-drive/derived
           file_server
       }
   }
   ```
   Decide LAN-only vs. public internet first (see below) — it changes this
   block (automatic HTTPS needs a public domain + port 80/443 reachable;
   LAN-only can use Caddy's internal CA or a self-signed cert instead).
7. **Exposure decision**: if this goes on the public internet, also plan for:
   port-forwarding only 80/443 to the Pi, `fail2ban` (or similar) watching the
   admin login rate-limit logs from task 03, and keeping the Pi's OS patched.
   If LAN-only (e.g. accessed over Tailscale/VPN), skip the public TLS
   complexity entirely.
8. **Backups**: two separate things need backing up, not one —
   - **Database**: the existing `make backup-db` target, run on a cron
     schedule, output copied off the Pi (not just to the same SD card).
   - **Original photos**: the irreplaceable asset. A periodic `rsync` from the
     external drive to a second location (another disk, a relative's NAS,
     cloud storage) — the SQLite DB is regenerable from photo metadata in a
     pinch, the original photo files are not.
9. **Smoke test after deploy**: health check reachable, login works, upload a
   test photo end-to-end, confirm it's served from `/derived/*` via Caddy (not
   proxied through Go), reboot the Pi and confirm the service comes back up on
   its own.

## Acceptance criteria

- `systemctl status photography` shows active/running after a fresh boot with
  no manual steps.
- Killing the process (`systemctl kill photography`) results in automatic
  restart.
- Photos load over HTTPS (or your chosen LAN scheme) with image requests
  served directly by Caddy, not proxied through the Go process.
- A simulated drive-unmounted state (unplug/unmount in a test) prevents the
  service from starting, with a clear log message, rather than starting and
  failing confusingly on first upload.
- A restore test: take a DB backup, delete the live DB, restore from backup,
  confirm the app comes back with the expected data.
