# Bogdan Guna Photography

[![Go version](https://img.shields.io/github/go-mod/go-version/bgguna/photography?filename=server%2Fgo.mod&label=Go)](server/go.mod)
[![Last commit](https://img.shields.io/github/last-commit/bgguna/photography?label=last%20commit)](https://github.com/bgguna/photography/commits/main)

A self-hosted photography portfolio: a public site with a home gallery, an
About page and a contact form, plus an admin area for publishing photos,
editing the About text and reading contact messages. It runs on a Raspberry
Pi, with the photos stored on an external drive.

The server is a single Go binary that renders server-side HTML (htmx and
Alpine.js add small interactions). There is no separate front-end build.

## Where to look

- [server/](server/readme.md): the Go application, how to run and test it,
  and the package layout
- [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md): setting up and updating the
  Raspberry Pi
- [docs/design.md](docs/design.md): architecture, data model, routes and the
  reasons behind the main choices
- [docs/plan.md](docs/plan.md): status of each task, with the task breakdown
  in [docs/tasks/](docs/tasks/README.md)

## Quick start (local development)

Requires Go and the `sqlite3` command-line tool. Commands run from the
`server/` directory.

```sh
cd server
make init-db                                    # create the database from db/schema.sql
go run ./cmd/createadmin -email you@example.com -password "a-long-password"
go run .                                        # or: make build && make run
```

Then open http://localhost:8080. The admin area is at `/admin`.

Settings come from `.env` at the repository root and from environment
variables. See [server/readme.md](server/readme.md#configuration) for the full
list.

## Common tasks

| Task | Command (from the repo root) |
|------|------------------------------|
| Run the tests | `make test` |
| Build for the Pi (64-bit) | `make build-arm64` |
| Build for the Pi (32-bit) | `make build-arm` |
| Start the server from source | `make server-run` |
| Show deployment steps | `make deploy-info` |

Database helpers (`init-db`, `backup-db`, `restore-db`, `reset-db`,
`shell-db`) live in `server/Makefile`. Run them from `server/`.
