# Photography Portfolio Deployment Guide

This guide covers deploying the photography portfolio application to a Raspberry Pi with systemd, Caddy, and an external drive for photo storage.

## Prerequisites

- Raspberry Pi 3/4/5 with 64-bit OS (aarch64) or 32-bit OS (armv7l)
- External USB drive (formatted with ext4 recommended)
- Caddy web server installed on the Pi
- Go 1.21+ on your development machine (for cross-compilation)

## Step 1: Determine Pi Architecture

On the Pi, run:
```bash
uname -m
```

- `aarch64` (64-bit): Use `GOARCH=arm64`
- `armv7l` (32-bit): Use `GOARCH=arm GOARM=7`

## Step 2: Cross-Compile the Binary

On your development machine:

```bash
# For 64-bit Pi (aarch64)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server-arm64 ./server

# For 32-bit Pi (armv7l)
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server-arm -v ./server
```

This uses:
- `CGO_ENABLED=0` to ensure pure Go compilation (important for cross-compilation with sqlite)
- `-ldflags="-s -w"` to strip symbols and reduce binary size
- `-o bin/server-{arch}` to create an architecture-specific binary

## Step 3: Set Up Service Account on Pi

On the Raspberry Pi (via SSH):

```bash
# Create a system user to run the app
sudo useradd --system --shell /bin/false --home-dir /opt/photography photography

# Create the app directory
sudo mkdir -p /opt/photography/bin
sudo chown -R photography:photography /opt/photography
```

## Step 4: Mount External Drive

On the Raspberry Pi:

### Find the Drive UUID

```bash
sudo blkid
```

Look for your USB drive (e.g., `/dev/sda1`), note the UUID.

### Create Mount Point

```bash
sudo mkdir -p /mnt/photo-drive
sudo chown photography:photography /mnt/photo-drive
```

### Add to fstab

Edit `/etc/fstab` and add:

```
UUID=your-uuid-here /mnt/photo-drive ext4 defaults,nofail 0 2
```

Replace `your-uuid-here` with the actual UUID from step 1.

### Mount the Drive

```bash
sudo mount /mnt/photo-drive
```

### Create Photo Storage Directories

```bash
sudo mkdir -p /mnt/photo-drive/originals /mnt/photo-drive/derived
sudo chown photography:photography /mnt/photo-drive/originals /mnt/photo-drive/derived
sudo chmod 750 /mnt/photo-drive/originals /mnt/photo-drive/derived
```

## Step 5: Deploy Application Files

Copy the binary and config to the Pi:

```bash
# From your dev machine
scp bin/server-arm64 pi@your-pi-ip:/opt/photography/bin/
scp deploy/photography.service pi@your-pi-ip:~/

# Create .env file on the Pi
ssh pi@your-pi-ip

# Copy deploy files to systemd
sudo cp ~/photography.service /etc/systemd/system/

# Create app directory structure
cd /opt/photography
sudo mkdir -p db
sudo chown photography:photography db
```

## Step 6: Configure Application Environment

On the Pi, create `/opt/photography/.env`:

```bash
MODE_ENV=production
PORT=8080
DB_PATH=./db/gallery.sqlite
PHOTO_STORAGE_PATH=/mnt/photo-drive/originals
SESSION_SECRET=your-random-secret-here
```

**IMPORTANT**: Generate a secure `SESSION_SECRET`:

```bash
openssl rand -base64 32
```

Set permissions:

```bash
sudo chmod 600 /opt/photography/.env
sudo chown photography:photography /opt/photography/.env
```

## Step 7: Configure Caddy

Copy the Caddyfile to the Pi:

```bash
scp deploy/Caddyfile pi@your-pi-ip:~/
ssh pi@your-pi-ip "sudo mv ~/Caddyfile /etc/caddy/Caddyfile"
```

Edit `/etc/caddy/Caddyfile` if you want a different configuration (see comments in the file for public internet setup).

For LAN-only access via Tailscale/VPN, the default configuration uses Caddy's internal CA.

## Step 8: Start the Service

On the Pi:

```bash
# Enable and start the service
sudo systemctl daemon-reload
sudo systemctl enable photography
sudo systemctl start photography

# Check status
sudo systemctl status photography

# View logs
sudo journalctl -u photography -f
```

## Step 9: Configure Caddy to Start

On the Pi:

```bash
# If using systemd for Caddy
sudo systemctl enable caddy
sudo systemctl start caddy

# Check Caddy status
sudo systemctl status caddy
```

## Step 10: Smoke Test

### Health Check

```bash
curl -k https://localhost:8443/healthz
```

Should return `{"status":"ok"}`.

### Login and Photo Upload

1. Visit the site in your browser (ignore cert warning for self-signed cert)
2. Navigate to `/admin/login`
3. Create an admin user or verify login works
4. Upload a test photo
5. Verify it appears in the gallery
6. Hide the photo and confirm it returns 404 when logged out
7. Publish it and confirm it's visible again

### Reboot Test

```bash
sudo reboot
```

After reboot, verify:
- Service came back up: `sudo systemctl status photography`
- Site is accessible
- Photos are still visible

## Step 11: Set Up Backups

### Database Backups

Create a backup script at `/opt/photography/backup-db.sh`:

```bash
#!/bin/bash
DB_PATH="/opt/photography/db/gallery.sqlite"
BACKUP_DIR="/mnt/photo-drive/backups"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"
cp "$DB_PATH" "$BACKUP_DIR/gallery-$TIMESTAMP.sqlite"

# Keep only last 30 days of backups
find "$BACKUP_DIR" -name "gallery-*.sqlite" -mtime +30 -delete
```

Add to crontab:

```bash
# Backup database daily at 2 AM
0 2 * * * /opt/photography/backup-db.sh
```

### Photo Backups

Set up `rsync` to a remote location (NAS, cloud storage, etc.):

```bash
# Example: sync to a remote NAS every night
0 3 * * * rsync -av --delete /mnt/photo-drive/originals/ user@nas:/backups/photography-originals/
```

## Troubleshooting

### Service won't start

Check logs:
```bash
sudo journalctl -u photography -n 50
```

Common issues:
- External drive not mounted: Check `mount | grep photo-drive`
- Database locked: Remove stale lock files
- Permission denied: Check file ownership with `ls -l /opt/photography`

### Drive keeps unmounting

Ensure the drive is powered (some USB hubs don't provide enough power to external drives). Add `upower` handling if needed.

### Caddy certificate errors

For LAN access, the self-signed cert is normal. Accept the browser warning or trust the internal CA.

For public internet, ensure port 80/443 are accessible and domain resolves to the Pi.

### Photos not loading

Check:
- `is_public` flag in database
- File permissions on `/mnt/photo-drive/originals/`
- Check Go logs: `sudo journalctl -u photography`

## Security Considerations

If deploying to the public internet:

1. **Firewall**: Only expose ports 80/443, block SSH from the internet
2. **Fail2ban**: Monitor login attempts
   ```bash
   sudo apt-get install fail2ban
   # Configure to watch admin login logs
   ```
3. **OS Updates**: Keep the Pi updated
   ```bash
   sudo apt-get update && sudo apt-get upgrade
   ```
4. **HTTPS**: Let's Encrypt certificates (automatic with Caddy)
5. **Rate Limiting**: Caddy rate limiting rules (see Caddyfile comments)

For LAN-only access via VPN/Tailscale, security is much simpler — no public TLS complexity needed.

## Rollback Process

If something goes wrong:

```bash
# Stop the service
sudo systemctl stop photography

# Restore from database backup
cp /mnt/photo-drive/backups/gallery-YYYYMMDD-HHMMSS.sqlite /opt/photography/db/gallery.sqlite
sudo chown photography:photography /opt/photography/db/gallery.sqlite

# Restart
sudo systemctl start photography
```

## Reference

- [Raspberry Pi OS](https://www.raspberrypi.com/software/)
- [Go Cross-Compilation](https://golang.org/doc/install/source#environment)
- [Caddy Documentation](https://caddyserver.com/docs/)
- [systemd Manual](https://www.freedesktop.org/software/systemd/man/)
