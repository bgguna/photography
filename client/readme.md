# Client

There's no separate client app here — the front end is server-rendered Go
`html/template` pages, served directly by the backend (`server/internal/web`),
enhanced with [htmx](https://htmx.org/) and [Alpine.js](https://alpinejs.dev/)
for interactivity. No separate build step, no client-side router, no JSON API
to maintain alongside the pages.

See [../docs/design.md](../docs/design.md#front-end-htmx--alpinejs) for the
reasoning behind this choice over a framework like Angular, React, or Vue.

[ [**back**](../readme.md) | [**server**](../server) | [**docs**](../docs) ]
