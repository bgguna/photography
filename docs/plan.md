# Implementation plan

Tracking status for the task breakdown in [tasks/](tasks/README.md). See each
linked file for the full implementation plan.

| # | Task | Summary | Status |
|---|------|---------|--------|
| 01 | [Database schema migrations](tasks/01-schema-migrations.md) | Add `sessions` table and `sort_order`/`is_public` (hide/publish) columns on `photos`. No albums. | DONE |
| 02 | [App bootstrap & wiring](tasks/02-app-wiring.md) | Turn `main.go` into a real entry point: config, DB, router skeleton, static files, graceful shutdown. | NEW |
| 03 | [Admin authentication](tasks/03-admin-auth.md) | Session-cookie login/logout for the single admin user, with auth middleware on `/admin/*`. | NEW |
| 04 | [Photo upload & processing pipeline](tasks/04-photo-pipeline.md) | Implement the `photo` package: save originals, extract EXIF, generate thumb/web derived sizes. | NEW |
| 05 | [Public gallery templates & UI](tasks/05-public-gallery.md) | Mobile-first single-gallery home and lightbox using htmx + Alpine. | NEW |
| 06 | [Admin UI](tasks/06-admin-ui.md) | htmx-driven admin pages: upload, delete, hide/publish, reorder, contact messages. | NEW |
| 07 | [Contact form](tasks/07-contact-form.md) | Wire the existing contact handlers into routes, with a public form, honeypot, and rate limiting. | NEW |
| 08 | [Raspberry Pi deployment](tasks/08-deployment-pi.md) | Cross-compiled binary, systemd service, Caddy reverse proxy, external drive mount, backups. | NEW |
