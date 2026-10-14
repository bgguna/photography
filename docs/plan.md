# Implementation plan

Tracking status for the task breakdown in [tasks/](tasks/README.md). See each
linked file for the full implementation plan.

| # | Task | Summary | Status |
|---|------|---------|--------|
| 01 | [Database schema migrations](tasks/01-schema-migrations.md) | Add `sessions` table and `sort_order`/`is_public` (hide/publish) columns on `photos`. No albums. | DONE |
| 02 | [App bootstrap & wiring](tasks/02-app-wiring.md) | Turn `main.go` into a real entry point: config, DB, router skeleton, static files, graceful shutdown. | DONE |
| 03 | [Admin authentication](tasks/03-admin-auth.md) | Session-cookie login/logout for the single admin user, with auth middleware on `/admin/*`. | DONE |
| 04 | [Photo upload & processing pipeline](tasks/04-photo-pipeline.md) | Implement the `photo` package: save originals, extract EXIF, generate thumb/web derived sizes. | DONE |
| 05 | [Public gallery templates & UI](tasks/05-public-gallery.md) | Mobile-first single-gallery home and lightbox using htmx + Alpine. | IN PROGRESS |
| 06 | [Admin UI](tasks/06-admin-ui.md) | htmx-driven admin pages: upload, delete, hide/publish, reorder, contact messages. | DONE |
| 07 | [Contact form](tasks/07-contact-form.md) | Wire the existing contact handlers into routes, with a public form, honeypot, and rate limiting. | DONE |
| 08 | [Raspberry Pi deployment](tasks/08-deployment-pi.md) | Cross-compiled binary, systemd service, Caddy reverse proxy, external drive mount, backups. | DONE |
| 09 | [Site title](tasks/09-site-title.md) | Rename the header to "Bogdan Guna Photography" and center it. | DONE |
| 10 | [Rename "Gallery" tab](tasks/10-rename-gallery-tab.md) | Nav label "Gallery" becomes "Home". | DONE |
| 11 | [New page](tasks/11-new-page.md) | About page between Home and Contact, text editable from admin. | DONE |
| 12 | [Contact Turnstile check](tasks/12-contact-turnstile.md) | Require a Cloudflare "Verify you are human" token to send a message. | TODO |
| 13 | [Site-wide Cloudflare (evaluate)](tasks/13-site-wide-cloudflare.md) | Decide whether to put the whole site behind Cloudflare's proxy and challenges. | TODO |
| 14 | [Contact heading spacing](tasks/14-contact-spacing.md) | More space between "Get in Touch" and the form. | DONE |
| 15 | [Required contact email](tasks/15-contact-email-required.md) | Email becomes mandatory, enforced client- and server-side. | DONE |
| 16 | [EXIF quotes](tasks/16-exif-quotes.md) | Backfill photos stored with quoted EXIF values before the extraction fix. | DONE |
