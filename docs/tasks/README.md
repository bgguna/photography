# Remaining work

Task breakdown of everything left to build, per [../design.md](../design.md).
Two steps from the design doc's original implementation order are already
done: `go.mod` cleanup (dropped `mattn/go-sqlite3`/logrus, added
`modernc.org/sqlite`) and fixing `internal/db/db.go` + `contact/contact.go` to
compile and match the schema. Everything below is what's left.

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
