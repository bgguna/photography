# Tasks

Task breakdown for the portfolio, per [../design.md](../design.md). Status for
each task is in [../plan.md](../plan.md). The original build (01–08) is
recorded here; the follow-up tasks (09–16) were added later.

Each file is a self-contained implementation plan: goal, scope, concrete
steps, and acceptance criteria.

## Order & dependencies

```
01-schema-migrations ──┬─→ 02-app-wiring ──┬─→ 03-admin-auth ──→ 06-admin-ui
                        │                    ├─→ 04-photo-pipeline ─┬─→ 05-public-gallery
                        │                    │                      └─→ 06-admin-ui
                        │                    └─→ 07-contact-form
                        │
                        └───────────────────────────────────────────→ 08-deployment-pi
                                                         (last, needs everything else working)
```

- **01** has to go first — everything else queries columns/tables it adds.
- **02** (router/config skeleton) unblocks all handler work; can be built with
  stub handlers before 03/04/05/06/07 are ready.
- **03** and **04** can be built in parallel once 01/02 are done.
- **05** and **06** both depend on **04** (nothing to show/manage without the
  upload pipeline); **06** also depends on **03** (admin routes need auth).
- **07** only needs **02** — the handler logic is already written.
- **08** is last: deploying before the app does anything useful isn't worth it.

## Tasks

1. [Database schema migrations](01-schema-migrations.md)
2. [App bootstrap & wiring](02-app-wiring.md)
3. [Admin authentication](03-admin-auth.md)
4. [Photo upload & processing pipeline](04-photo-pipeline.md)
5. [Public gallery templates & UI](05-public-gallery.md)
6. [Admin UI](06-admin-ui.md)
7. [Contact form](07-contact-form.md)
8. [Raspberry Pi deployment](08-deployment-pi.md)

## Follow-ups

Polish and hardening after the initial build.

- **09 → 10 → 11** edit the same header and nav markup (duplicated in
  `layout.html` and `contact.html`), so do them in order.
- **12**, **14** and **15** all edit `contact.html`. They're independent, but
  watch for merge conflicts.
- **13** is a decision, not code. It may produce a follow-up deployment task.
- **16** is a one-off data fix. Back up the Pi database first.

9. [Site title: "Bogdan Guna Photography", centered](09-site-title.md)
10. [Rename the "Gallery" tab to "Home"](10-rename-gallery-tab.md)
11. [New page between "Home" and "Contact"](11-new-page.md)
12. [Cloudflare Turnstile check on the contact form](12-contact-turnstile.md)
13. [Evaluate: Cloudflare protection for the whole site](13-site-wide-cloudflare.md)
14. [More space between "Get in Touch" and the form](14-contact-spacing.md)
15. [Make email a required contact field](15-contact-email-required.md)
16. [Remove extra quotes from displayed EXIF data](16-exif-quotes.md)
