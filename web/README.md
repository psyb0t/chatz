# chatz web

Brutalist static-SPA frontend for chatz. SvelteKit + `adapter-static`
(`ssr = false`), typed against the OpenAPI spec via `openapi-typescript` +
`openapi-fetch`.

## Develop

The dev server proxies `/api` and `/healthz` to a locally-running Go backend,
so bring the backend up first.

```bash
# 1. run the backend (from the repo root), listens on :8080 by default
make run

# 2. run the web dev server (proxies to the backend)
make web-dev
# or, with a non-default backend address:
CHATZ_API_TARGET=http://localhost:9090 make web-dev
```

`CHATZ_API_TARGET` defaults to `http://localhost:8080` (see `vite.config.ts`).
The session cookie is HttpOnly and same-origin. The proxy makes the SPA and the
API share an origin in dev, so the browser ships the cookie automatically.

## Model and context controls

The model picker displays configured aliases but always submits the discovered
model ID. It starts at the instance default when one is configured. The settings
popover disables reasoning effort with an explanation unless the selected model
explicitly advertises reasoning support. It also offers Precise, Balanced, and
Creative presets. A user can save the current values under a name, apply it to
the open form, or delete it; named presets are validated browser-local
convenience data and only affect a chat when the existing **Save** action runs.

For an existing chat, the composer debounces `POST /chats/{chatId}/context-preview`
while the user types. The returned server-side selection renders the context
meter: total/budget/free tokens, sticky-system/history/draft breakdown, and
omitted complete turns. The browser never attempts to guess token counts from a
partial local transcript.

## Generative UI catalog

`src/lib/render/` contains the 26-component json-render catalog and registry.
It includes layout, text, status, table, and analytical components. The analysis
set covers trends, categorical comparisons, composition, funnels, bounded
metrics, correlations, matrices, distributions, five-number summaries,
hierarchies, relationships, and large scrollable logs. Charts are responsive
native SVG and accept only the documented structured props; arbitrary chart
configuration is not evaluated.

For a recording-ready set of human-style dashboard conversations that stream the
catalog through the same SSE and client-side fence-assembly path used by model
responses, run `make run-showcase` from the repository root. It keeps the normal
model list
and MCP setup, while exact operations, sales, and customer-risk prompts stream
deterministic thinking, paced synthetic tool cards, then dashboard replies
whose metrics and recommendations come from those visible tool results; all
other messages use the selected model.

## Streaming render behavior

The browser recognizes fenced ` ```spec ` blocks, preserves their place between
prose segments, and incrementally applies their RFC-6902 JSON Patch content.
The parser supports SSE chunk boundaries and multi-line patch JSON. Generated
content is constrained to its message column, so charts/tables cannot create a
page-level horizontal scrollbar; semantic design tokens keep it readable in
both light and dark themes. There is no separate “Generating …” label: a
component appears at its natural location once its patch-defined shape exists,
with a reduced-motion-aware reveal.

Stopping or disconnecting a real streamed turn does not discard useful text:
on reload, the browser renders the server's incomplete assistant checkpoint as
an **Interrupted response**. It remains visible for the user but is excluded
from future model context.

During an active turn, the chat pane displays the server's lifecycle status and
locally measured elapsed time. A terminal stream failure preserves the submitted
prompt and chosen model in browser memory and offers **Retry**; retry submits a
new durable turn rather than modifying the failed one.

See [`src/lib/render/README.md`](src/lib/render/README.md) for the catalog,
source map, streaming edge cases, and safe change workflow.

After changing the catalog, regenerate the prompt embedded by the backend:

```bash
make genui-prompt
```

This is a backend artifact generator: it turns the web catalog into the system
prompt embedded for every real model turn. Keep the catalog, registry, shared
names, generator, and generated prompt in lockstep; tests enforce that.

## Generate the API types

The typed client is bound to types generated from `../api/api.yml`. Never edit
`src/lib/api/schema.d.ts` by hand. Regenerate it:

```bash
make web-gen-api   # → src/lib/api/schema.d.ts
```

## Verify

```bash
make web-gen-api                    # regenerate types from api/api.yml
make genui-prompt                   # regenerate backend prompt from the catalog
make web-check                      # svelte-check, strict: 0 errors
make web-test                       # vitest renderer + stream unit tests
make web-build                      # static build/ with index.html + _app/
make web-format-check               # prettier, no modifications
make test-e2e                       # Go testcontainers browser e2e (repo root)
```

## Dependencies

Add/remove/update deps only through the age-gated Makefile targets
(`make web-pkg-add PKG=…`, `web-pkg-remove`, `web-pkg-update`, `web-pkg-lock`).
Never run raw `pnpm add`.

All web tooling runs in the dev Docker image. Its `node_modules` lives in the
`chatz-web-node-modules` Docker volume, keeping host Node installations out of
the workflow while preserving installed dependencies between commands.
