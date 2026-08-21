# chatz

[![CI](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/chatz/badges/coverage.svg)](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/chatz/badges/version.svg)](https://github.com/psyb0t/chatz/releases)
[![license](https://raw.githubusercontent.com/psyb0t/chatz/badges/license.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/psyb0t/chatz?style=flat-square)](https://hub.docker.com/r/psyb0t/chatz)

A lean, self-hosted AI chat app that runs without a cluster. One
Go binary talks to OpenAI-compatible and Anthropic upstreams, streams responses
over Anthropic-style SSE, and serves a Svelte SPA it embeds at build time
(`go:embed`), so there's one artifact to deploy and one origin to reason
about. Persistence is Postgres by default, with a single-process SQLite mode
for a local persistent volume. Tool calling is via MCP servers (stdio + HTTP).
Assistant replies can emit fenced ` ```spec ` blocks that render inline as live
UI components (generative UI, via json-render).

Streaming chat, MCP tools, generative UI, and admin-provisioned auth. It ships
as one binary plus the database you pick, and that's the whole deploy.

---

## Contents

- [Features](#features)
- [Quickstart](#quickstart)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [HTTP API](#http-api)
- [Development](#development)
- [Deploy](#deploy)
- [Security notes](#security-notes)
- [License](#license)
- [Changelog](CHANGELOG.md)

---

## Features

- **Streaming chat** over Anthropic-style SSE (one wire, backend to browser).
  A turn renders as an ordered list of blocks in **arrival order**: prose
  (markdown + inline generative UI), reasoning, and tool calls interleave exactly
  as the model emits them (`text → tool → text → tool → …`), never clumped into
  one undifferentiated wall at the end. During a live turn, a small progress
  indicator reflects actual provider and tool lifecycle states and elapsed
  time rather than a made-up timer. A terminal provider failure shows a concise
  explanation and can retry the same prompt and model as a new durable turn.
- **Durable sends.** The submitted user message is saved before its stream
  begins, so hitting stop or refreshing doesn't eat what you just typed.
  While a real assistant streams, its text/reasoning is checkpointed as an
  explicitly interrupted partial response; refresh preserves it, but it is
  never replayed to a later model request. The completed assistant/tool
  transcript atomically replaces that checkpoint.
- **Chat organization that does not turn into a landfill.** Create an empty
  chat before you have a prompt, rename it, and search titles with a literal
  case-insensitive filter. Each row has a hover-revealed ⋮ menu with Edit and
  Delete; deletion is soft deletion.
- **Per-chat generation controls.** The composer settings panel controls
  temperature, top-p, reasoning effort, output tokens, and history tokens.
  Unset generation values use the provider default; unset history uses 100,000.
- **Thinking + tool cards.** Reasoning deltas from upstreams that expose them
  (`reasoning` / `reasoning_content`, e.g. gpt-oss) stream as their own
  collapsible thinking block ahead of the answer. Each tool call renders an
  expandable card (name, streamed args, result) that goes `CALLING → DONE/ERROR`
  as the result lands.
- **Generative UI.** Assistant (or MCP tool) output can include fenced
  ` ```spec ` blocks, json-render JSONL (one RFC-6902 patch per line), that
  the browser detects **client-side** and renders as live components from a
  26-component shared catalog. Actual live components, not a screenshot of a
  chart the model hallucinated. Alongside text, layout, status, and table
  primitives, the catalog includes time-series, area, sparkline, bar, donut,
  funnel, gauge, scatter, heatmap, histogram, box-plot, treemap, network, and
  large log-viewer components. Charts use responsive native SVG with no charting
  runtime dependency. Fence detection and progressive assembly live in the web
  app (`web/src/lib/render/`), on top of `@json-render/*` 0.19.0.
  See the [rendering deep-dive](web/src/lib/render/README.md) for the streaming
  contract, catalog workflow, and responsive rendering guarantees.
- **Recording-ready showcase mode.** Demos that depend on a live model going
  off-script are unreliable. `make run-showcase` keeps
  the normal model list, MCP setup, and chat behavior intact, but intercepts
  exact catalog prompts with deterministic thinking, synthetic tool activity,
  then embedded dashboards. It uses deliberate model- and tool-like pauses, and
  every displayed business metric, action, and entity is grounded in the
  visible synthetic tool results. Those
  replies are persisted like normal chats, so a recording can refresh or
  continue them.
- **Auth: admin-provisioned, no public registration.** Nobody signs themselves
  up. On first run the app is in *setup* state (no users yet); `/setup` creates
  the sole admin, and the admin provisions everyone else. Sessions are opaque
  tokens in a cookie.
  Optional **passwordless single-user mode** auto-logs-in the sole admin while
  exactly one user exists (`CHATZ_AUTH_PASSWORDLESS=true`).
- **Model discovery across providers.** `CHATZ_UPSTREAMS` is a JSON array of
  OpenAI-compatible or Anthropic endpoints. Each entry selects a `provider`;
  the app uses that driver's model discovery and merges the results into the
  picker. A picked model routes back to its owning upstream. Optional per-model
  metadata adds a friendly alias and known context/output/capability limits
  without inventing models; `CHATZ_DEFAULT_MODEL` chooses one advertised model
  as the instance default.
- **MCP tool servers.** Add stdio or HTTP MCP servers (admin), or import a
  Claude-style `.mcp.json`. Their tools are aggregated into the chat's tool
  catalog. HTTP header secrets and stdio env are **encrypted at rest**
  (AES-256-GCM AEAD) using `CHATZ_SECRETS_KEY`. Not base64, not "we'll do it
  later", actually encrypted. Stored `Authorization` values stay masked when an
  MCP server is listed or edited.
- **Bounded chat history.** Each chat has an outbound-history cap (100,000
  tokens by default), so a long conversation doesn't quietly turn into a
  four-figure invoice. The earliest system message stays sticky and counts toward
  the cap; the current user turn stays pinned too. Older history gets added
  newest-first until the next complete message unit would blow past the cap.
  Whole units only, so a tool result never gets orphaned from its call.
- **Honest context controls.** The composer shows the backend's actual next-turn
  selection: sticky system, retained history, current draft, total/budget/free
  tokens, and any omitted complete turns. Friendly model aliases never replace
  the executable model ID, and reasoning controls disable themselves when the
  selected model does not advertise support. The settings popover includes
  Precise, Balanced, and Creative presets plus named browser-local presets;
  applying one still uses the same per-chat settings save path.
- **Outbound-prompt trace.** `LOG_LEVEL=debug` records the exact ordered,
  post-trimming messages sent to the model, including token accounting,
  reasoning, and tool arguments, so when the model does something stupid you
  can see precisely what you fed it. Secret-shaped values are redacted, but
  ordinary user content is not. Keep those logs local.
- **Usage accounting + metrics.** Every upstream call goes through a usage
  decorator that records Prometheus counters/histograms in-process and writes
  one `llm_usage` row per call, best-effort, on a detached ctx, so the slow
  and failed calls still get recorded instead of vanishing from cost analysis.
- **Admin readiness.** The sidebar exposes the running app version and commit,
  selected database driver, applied migration position, redacted upstream
  health, and backup-marker freshness. It never guesses a backup succeeded
  from a database file or volume; the backup job records completion explicitly.

---

## Quickstart

One command brings up the whole thing, Postgres plus the Go backend, which
serves both the API and the embedded SvelteKit SPA on a single origin:

```bash
make run
```

Then open **http://localhost:8080**. First run needs no secrets and no LLM
config whatsoever: the SPA loads, you hit **`/setup`** to create the admin, and
the chat list works (the model list is just empty until you wire an upstream).
With no LLM configured at all you can still drive the render pipeline
end-to-end via showcase mode: `make run-showcase`, then send one of the
embedded catalog prompts.

Tear it down with:

```bash
make stop
```

`make run` builds the image (the `Dockerfile`'s node stage builds the SPA and the
Go stage embeds it into the binary), starts Postgres by default, waits for it to
be healthy, then starts the backend on `127.0.0.1:8080`. Health check: `GET
/healthz`. To change the host port, set `CHATZ_HTTP_PORT` in `.env`.

### SQLite single-instance mode

Set `CHATZ_DB_DRIVER=sqlite` in `.env`, then use the same command:

```bash
make run
```

That starts only Chatz, not the Postgres dependency. Its database is a regular
file at `CHATZ_DB_SQLITE_PATH` (default `/data/chatz.sqlite`) on the persistent
named `chatzdata` volume. SQLite mode is for one Chatz process and one local
Docker volume only: do not place it on NFS, share it between replicas, or expect
it to convert an existing Postgres database. Keep Postgres for shared or
multi-process deployments.

`make run` also creates a host-side JSON diagnostic file, `chatz.log`, and the
app truncates/reopens it on boot. Watch it from another terminal with
`tail -f chatz.log`.

### Record a deterministic showcase

For a screen recording, start the normal stack with exact-message showcase
interception enabled:

```bash
make run-showcase
```

After completing `/setup`, select a normally configured model and send one of
these exact human-style prompts:

- `Show me what's happening across the production platform right now.`
- `Where are we losing deals in the sales pipeline?`
- `Which customers are at risk and who should the team contact first?`

Each produces a durable turn that streams a thinking block, synthetic tool
calls, then a dashboard-shaped reply. Any other message uses the selected model
and the normal MCP-enabled chat path.

### Configure a real LLM

Copy `.env.example` to `.env`, then tell Chatz which Elelem driver talks to
which endpoint. `CHATZ_UPSTREAMS` is the whole provider configuration:
`provider` selects the `openai` (also OpenAI-compatible) or `anthropic`
driver; `baseUrl` points it somewhere; `apiKeyEnv` names the credential variable
in the same `.env`. The key itself never goes inside the JSON. Leave the array
empty when you want the UI, setup, chat history, and MCP management without a
model yet. Every upstream names its driver explicitly. `baseUrl` is optional
for the official OpenAI and Anthropic endpoints. Chatz isolates every driver
from the SDK's process-environment fallback, so an `apiKeyEnv` is the only
credential that upstream may use and a keyless local endpoint stays keyless.

  ```json
  [
    {"name":"openai","provider":"openai","baseUrl":"https://api.openai.com/v1","apiKeyEnv":"OPENAI_API_KEY"},
    {"name":"anthropic","provider":"anthropic","apiKeyEnv":"ANTHROPIC_API_KEY"},
    {"name":"ollama","provider":"openai","baseUrl":"http://localhost:11434/v1"}
  ]
  ```

  To add public picker metadata to a discovered model, nest a `models` entry
  under its owning upstream. This does not create or authorize a model: the
  upstream must still advertise its `id` during discovery.

  ```json
  {
    "name":"gateway",
    "provider":"openai",
    "baseUrl":"https://llm.example.test/v1",
    "apiKeyEnv":"GATEWAY_API_KEY",
    "models":[
      {
        "id":"analysis-model",
        "alias":"Deep analysis",
        "contextWindow":128000,
        "maxOutputTokens":8192,
        "supportsTools":true,
        "supportsReasoning":true,
        "supportsVision":false,
        "supportsFiles":true,
        "expectedFirstTokenLatencyMs":900,
        "inputPricePerMillionTokens":{
          "amountSmallestUnit":15,
          "currency":"USD"
        },
        "outputPricePerMillionTokens":{
          "amountSmallestUnit":60,
          "currency":"USD"
        }
      }
    ]
  }
  ```

  Set `CHATZ_DEFAULT_MODEL=analysis-model` to make an advertised model the
  initial picker choice. Omit capability booleans when unknown; Chatz will not
  pretend that a provider supports a control it has not declared.

`make run` picks up `.env` automatically. `.env` is gitignored. Don't be the
guy who commits real keys anyway.

### Secrets key (for MCP secrets at rest)

Generate a 32-byte base64 key and set `CHATZ_SECRETS_KEY`:

```bash
openssl rand -base64 32
```

Without it the app still boots, but storing an MCP secret (HTTP header / stdio
env) fails outright rather than quietly writing your token to the database in
plaintext.

### Passwordless single-user mode

Set `CHATZ_AUTH_PASSWORDLESS=true`. While exactly one user exists, the app
auto-issues that admin's session. No login screen. Adding a second user
re-enables normal login for everyone.

---

## Configuration

All backend configuration is environment variables (parsed via `gonfiguration`).
See [`.env.example`](.env.example).

| Variable | Default | Purpose |
|---|---|---|
| `CHATZ_HTTP_LISTENADDRESS` | `:8080` | Public HTTP listen address (API + embedded SPA). |
| `CHATZ_HTTP_PORT` | `8080` | Host port published by the local docker compose stack. |
| `CHATZ_METRICS_LISTENADDRESS` | `:9091` | Internal `/metrics` (Prometheus) listen address; empty disables it. |
| `CHATZ_PROFILING_LISTENADDRESS` | `:6060` | Internal pprof listen address; empty disables it. |
| `CHATZ_DB_DRIVER` | `postgres` | Persistence driver: `postgres` (default) or `sqlite`. |
| `CHATZ_DB_SQLITE_PATH` | `/data/chatz.sqlite` | SQLite-only absolute database file directly under `/data`. |
| `CHATZ_DB_SQLITE_BUSY_TIMEOUT` | `5s` | SQLite-only bounded wait for a locked database. |
| `CHATZ_BACKUP_STATUS_PATH` | `/data/chatz-backup-status.json` | Direct `/data` file containing the successful-backup completion marker shown to admins. |
| `CHATZ_BACKUP_MAX_AGE` | `24h` | Age at which a valid completion marker becomes stale on the admin readiness page. |
| `CHATZ_DB_HOSTNAME` | `localhost` | Postgres host. |
| `CHATZ_DB_PORT` | `5432` | Postgres port. |
| `CHATZ_DB_USERNAME` | `chatz` | Postgres user. |
| `CHATZ_DB_PASSWORD` | `chatz` | Postgres password. |
| `CHATZ_DB_NAME` | `chatz` | Postgres database name. |
| `CHATZ_DB_ISSSL` | `false` | Use SSL for the Postgres connection. |
| `CHATZ_SESSION_SECRET` | *(empty)* | Reserved for future cookie signing / CSRF; not required yet. |
| `CHATZ_SECRETS_KEY` | *(empty)* | Base64 32-byte AES-256-GCM key sealing MCP secrets at rest. Unset ⇒ secret storage refused (no plaintext). |
| `CHATZ_AUTH_PASSWORDLESS` | `false` | Auto-login the sole admin while exactly one user exists. |
| `CHATZ_SHOWCASE_MODE` | `false` | Intercept exact recording-showcase prompts with deterministic thinking, synthetic tool cards, and embedded dashboards. Model discovery, MCP setup, and all other chat turns remain normal. `make run-showcase` sets it for that run. |
| `CHATZ_UPSTREAMS` | *(empty)* | JSON array of Elelem-driver upstreams with `provider` set to `openai` or `anthropic`. `baseUrl` is optional for the providers' official endpoints; `apiKeyEnv` is the only credential source for its upstream. Optional `models` entries attach aliases; token limits; tools/reasoning/vision/files support; expected first-token latency; and input/output prices only to IDs discovered from that upstream. Prices are integer currency-smallest-units per million tokens (`amountSmallestUnit` + uppercase ISO currency), never floats. Empty leaves the app up with no LLM models. |
| `CHATZ_DEFAULT_MODEL` | *(empty)* | Discovered model ID used as the instance default for new chats. Startup rejects an ID no configured upstream advertises. |
| `CHATZ_UPSTREAM_CONNECT_TIMEOUT` | `10s` | Bounds model discovery and TCP/TLS connection setup for each outbound provider request. |
| `CHATZ_UPSTREAM_FIRST_TOKEN_TIMEOUT` | `45s` | Maximum provider time from acquiring its bounded request slot to the first streamed delta. |
| `CHATZ_UPSTREAM_TURN_TIMEOUT` | `5m` | Maximum provider time from acquiring its bounded request slot through completion. |
| `CHATZ_UPSTREAM_CONCURRENCY` | `8` | Maximum simultaneous provider requests per configured upstream. |
| any `apiKeyEnv` name | *(empty)* | Credential named by one `CHATZ_UPSTREAMS` entry, for example `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or a gateway-specific name. Put it in gitignored `.env`; Compose forwards it to Chatz without putting the value in the JSON. |
| `CHATZ_FORCE_REAL_LLM` | `false` | Under `go test` every upstream resolves to a scripted test double, so a test cannot reach a provider by accident. `true` uses the real providers instead, what `make test-real` does. No effect outside tests. |
| `LOG_LEVEL` | `info` | Backend log threshold; use `debug` locally for the outbound-prompt trace. |
| `LOG_FORMAT` | `json` | Backend log format. |
| `LOG_ADD_SOURCE` | `true` | Include source locations in backend log records. |
| `CHATZ_APP_VERSION` | `dev` | Version embedded by Compose when it builds a local image; release images embed their release version. |
| `GIT_COMMIT` | *(empty)* | Revision embedded at image build and included in logs/readiness. At runtime it is only a fallback for a locally built binary. |

Logging is configured via `slogging/slogconf` env vars. `LOG_LEVEL` defaults to
`info`; set it to `debug` in the gitignored local `.env` only while
troubleshooting. Debug records describe the prompt actually sent after history
trimming; secret-shaped content and tool arguments are redacted, but ordinary
chat text may remain. Do not use debug in a shared log sink. In docker compose,
`CHATZ_HTTP_PORT` sets the published host port (default `8080`).

> **Metrics / pprof.** The usage decorator records Prometheus metrics
> in-process; they're served at `/metrics` on `CHATZ_METRICS_LISTENADDRESS`
> (`:9091`) with pprof on `CHATZ_PROFILING_LISTENADDRESS` (`:6060`). Both run
> as internal-only listeners alongside the API. **Do not** hang them off your
> public ingress, pprof will happily dump your process to anyone who asks. A
> bind failure only warns, because observability has no business taking the
> API down with it. An empty address disables the listener. Usage is also
> persisted per call to the `llm_usage` table.

---

## Architecture

- **One origin, one binary.** No CORS configuration or separate frontend
  deployment.
  The Go binary serves the JSON API under
  `/api/v1`, plus the embedded SvelteKit static SPA at `/` with an SPA fallback
  (any non-API path → `index.html`). The SPA is built into
  `internal/pkg/webassets/dist` and embedded via `go:embed`; `make web-embed`
  syncs the real build in (a committed placeholder keeps `go build` green
  without a fresh SPA build).
- **servicepack service.** The HTTP server runs as a
  [`servicepack`](https://github.com/psyb0t/servicepack) service
  (`internal/pkg/services/http-server`): it parses config, opens the DB (runs
  migrations, wires the generated repositories), builds collaborators (secrets
  box, upstream registry, MCP manager, auth), serves until ctx cancel, and
  releases resources (MCP subprocesses, DB) on stop.
- **DB layer.** gorm + gorm-gen generated repositories on Postgres or SQLite.
  SQLite starts from its own embedded baseline rather than replaying the
  Postgres migration history; later schema changes need a migration for each
  dialect. `.gen.go` files are never hand-edited. Change the source and
  regenerate. Generated repository code is part of the database contract, not
  scratch space.
- **Spec-first API.** `api/api.yml` (OpenAPI 3) is the source of truth, not a
  doc somebody updates when they remember. `oapi-codegen` generates the strict
  Echo server + a Go client; the web app generates its typed client from that
  same spec via `openapi-typescript` + `openapi-fetch`. Backend and frontend
  drift apart at compile time or not at all.
- **SSE wire.** `POST /api/v1/chats` and `POST /api/v1/chats/{chatId}` return
  `text/event-stream`; the new chat id for a freshly created chat is carried in
  the stream's `message_start` event. Ephemeral `chat_status` frames report
  `connecting`, `waiting_first_token`, `streaming`, `running_tool`, or
  `retrying`; they contain no model output, tool arguments, or provider errors.
  A custom oapi-codegen strict template streams (flushes per write) instead of
  buffering.
- **Chat turn contract.** The chat service serializes turns, builds the bounded
  upstream prompt, saves the user turn before streaming, checkpoints partial
  assistant content, then atomically records completed assistant/tool output.
  See [`internal/pkg/core/chats/README.md`](internal/pkg/core/chats/README.md).
- **Provider-neutral LLM engine.** [`elelem`](https://github.com/psyb0t/elelem)
  owns request snapshots, history limiting, streaming assembly, retries,
  structured output, and tool-loop safety. Its OpenAI and Anthropic drivers
  keep their SDKs locked in their own packages; chatz keeps persistence, MCP
  discovery, and usage records. It lives in its own module rather than in this
  application.
- **SSE protocol library.** [`essessey`](https://github.com/psyb0t/essessey)
  owns the event model, the publisher, the text/thinking streamers, the SSE
  framing, and reassembly of a stream back into text plus tool calls. Its
  `elelemstream` subpackage translates elelem's callbacks into
  correctly-indexed content blocks. This used to live here as
  `internal/pkg/sse` and now lives in its own module. Chatz keeps the stall
  heartbeat and per-round log lines.
- **MCP lifecycle.** Admin-managed servers connect asynchronously, expose
  namespaced tools, and retry transient failures. See
  [`internal/pkg/mcp/README.md`](internal/pkg/mcp/README.md).
- **Generative UI path.** The backend streams assistant text verbatim (including
  any ` ```spec ` fences); the browser detects the fences **client-side**
  (`web/src/lib/render/`) and renders them through a json-render custom catalog.

---

## HTTP API

The browser uses the versioned JSON/SSE API at `/api/v1`; its checked-in
[OpenAPI contract](api/api.yml) is the exact client-facing reference and drives
both generated clients. It covers setup/login/logout, admin user provisioning,
model and redacted upstream health, streamed chat creation and continuation,
message/history settings and context preview, rename/search,
and MCP server import, lifecycle, and per-chat enablement. Normal user routes
are ownership-checked; user and MCP administration require an admin session.
`/healthz` stays unversioned for container health checks.

---

## Development

The frontend lives in [`web/`](web/) (SvelteKit static SPA, `adapter-static`,
typed against `api/api.yml`). See [`web/README.md`](web/README.md).

For detailed handoff, read [`internal/pkg/core/chats/README.md`](internal/pkg/core/chats/README.md),
[`internal/pkg/mcp/README.md`](internal/pkg/mcp/README.md), and
[`web/src/lib/render/README.md`](web/src/lib/render/README.md). The reusable LLM
engine and driver contract are documented in the
[elelem repository](https://github.com/psyb0t/elelem).

Key make targets ([`Makefile`](Makefile), [`Makefile.servicepack`](Makefile.servicepack)):

| Target | What it does |
|---|---|
| `make run` | Build + start Chatz via docker compose; Postgres by default, or one-process SQLite when `CHATZ_DB_DRIVER=sqlite` is in `.env`. |
| `make run-showcase` | Build + start the normal stack with exact-message recording showcase interception. |
| `make stop` | Stop the stack (`docker compose down`). |
| `make build` | Build the app binary (in Docker). |
| `make test` | Go and web unit tests. It does not start test infrastructure or a browser. |
| `make test-coverage` | Integration-tagged Go coverage gate plus web unit tests. CI runs this target. |
| `make test-integration` | Just the integration tests (testcontainers / DIND). |
| `make lint` | Lint Go, shell, and web code. |
| `make lint-fix` / `make lint-fix-web` | Apply Go/shell or web formatting fixes, respectively. |
| `make generate` | Run all code generation: `go generate ./...` plus the static web build. Each generated package declares its own generator in a `gen.go`, so new ones are picked up automatically. |
| `make generate-repos` | Regenerate just the gorm repositories. |
| `make generate-api` | Regenerate just the HTTP server + client. |
| `make migrate` | Run DB migrations. |
| `make web-dev` | Run the Vite dev server (proxies `/api` + `/healthz` to the backend). |
| `make web-build` | Build the static SPA (`web/build`). |
| `make web-embed` | Build the SPA and sync it into the `go:embed` dist dir. |
| `make web-gen-api` | Regenerate the web API types from `api/api.yml`. |
| `make genui-prompt` | Regenerate the backend's embedded GenUI prompt from the web catalog. |
| `make lint-web` | Lint the web app: `prettier --check` + `svelte-check` (strict). Runs as part of `make lint`. |
| `make test-web` | Web unit tests (vitest: SSE parser + render pipeline). Runs as part of `make test` and `make test-coverage`. |
| `make test-api` | API suite (Go testcontainers: pg + prod app image + browser). See below. |

Web dependencies go through the age-gated `web-pkg-*` targets only
(`web-pkg-add`, `web-pkg-remove`, `web-pkg-update`, `web-pkg-lock`): pnpm
only, never raw `npm` / `pnpm add`. Yes, really. Use the targets.

Go dependencies and tools go through the age-gated `pkg-*` targets only
(`pkg-add`, `pkg-add-tool`, `pkg-remove`, `pkg-update`, `pkg-upgrade`, and
`pkg-lock`). They refresh `go.sum` and the committed `vendor/` tree inside the
dev container, so nobody's laptop-specific toolchain gets to decide what ships.

### API tests

`make test-api` runs the full-stack API suite entirely in Go, under the `api`
build tag in `tests/api/`. Each test is self-contained: [testcontainers](https://golang.testcontainers.org/)
stands up the prod app image (built from the repo `Dockerfile`, embedded SPA)
against both throwaway Postgres and temporary in-container SQLite databases,
plus a fake upstream (and, when a driver needs it, an MCP fixture server) on
one shared network, then drives a real headless
browser via [`psyb0t/stealthy-auto-browse`](https://hub.docker.com/r/psyb0t/stealthy-auto-browse)
action-by-action through its JSON HTTP API. Every step is asserted, so failures
identify the action that failed, and the whole stack is torn down on cleanup.
The shared fixtures live in
`tests/testinfra/` (`api.go`, `browser.go`).

The drivers cover: showcase dashboard render + reload durability (`showcase`),
theme toggle / tool-card collapse / settings popover / model-picker filter
(`smoke`), the admin users page (`users`), the per-chat and admin MCP flows
(`chat_mcp`, `mcp_admin`), and the mobile off-canvas drawer geometry
(`mobile_drawer`). They assert on rendered DOM plus the structured `CHATZ_LOG`
console lines the client logger emits (via `?log=debug`), proving the embed +
serve + SSE chain round-trips same-origin. `make test-api` runs it in the dev
container with the host Docker socket (DIND). Focus one flow without bypassing
the harness with, for example,
`API_RUN='^TestSmoke$' API_PARALLEL=1 make test-api`; `API_TIMEOUT` controls the
package timeout. It runs each selected flow against both stores by default;
set `API_DB_DRIVERS=sqlite` or `API_DB_DRIVERS=postgres` to focus one.

The `real_chat` driver exercises a genuine streamed turn against a live
configured OpenAI-compatible or Anthropic upstream; it self-skips unless
`CHATZ_UPSTREAMS` is configured. It validates that configuration and forwards
only the provider-key environment variables named by its upstream entries. Run
it with `API_REAL=1 make test-api`, which loads the same `.env` `make run` uses.

Real-LLM tool calling is covered by a build-tagged test that drives the chat loop
against the configured provider plus the Python MCP server in
`tests/mcpserver/`, asserting the model calls a tool and the `tool_use` /
`tool_result` blocks reach the wire. It is opt-in and uses the same `.env`
`make run` uses. The endpoint and provider resolve from `CHATZ_UPSTREAMS`;
`CHATZ_REAL_MODEL` can override the provider-aware default. Run it with
`make test-real` (dev container, host network, `--env-file .env`); it skips
cleanly without a key.

---

## Deploy

### Release image

Each push to `main` publishes `psyb0t/chatz:latest`; a release tag named
`v<semver>` publishes the matching immutable `psyb0t/chatz:v<semver>` image.
Both build `Dockerfile` for `linux/amd64` and `linux/arm64`, run the reusable
workflow's high-severity vulnerability scan, and publish SBOM and provenance
attestations. Pull the exact release tag for a deployment rather than the
mutable `latest` tag.

For a single-instance SQLite deployment, populate a gitignored `.env` from
`.env.example`, set `CHATZ_DB_DRIVER=sqlite`, then run the immutable app image
with a persistent `/data` volume and an explicit host-side diagnostic log:

```bash
docker volume create chatz-data
test ! -e chatz.log || test -f chatz.log
touch chatz.log
sudo chown 1000:1000 chatz.log
chmod 0644 chatz.log
docker run --name chatz \
  --env-file .env \
  --publish 127.0.0.1:8080:8080 \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --memory 512m \
  --cpus 1.0 \
  --pids-limit 256 \
  --mount type=bind,src="$(pwd)/chatz.log",dst=/app/chatz.log \
  --volume chatz-data:/data \
  psyb0t/chatz:<version> run
```

The image runs as a non-root user. The app needs no Linux capabilities; `/data`
and the explicit `chatz.log` bind mount are its only writable locations. Use
the Compose Postgres mode for replicas, networked storage, or an existing
Postgres installation.

### Backup and restore

Before restoring either store, stop Chatz so it cannot write while its database
is being replaced. Keep a copy of the prior backup until the restored instance
has started and passed its health check.

For the Compose PostgreSQL store, write a logical backup to the host:

```bash
docker compose exec -T postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  > chatz-postgres.sql

backup_completed_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
docker compose exec -T -e BACKUP_COMPLETED_AT="$backup_completed_at" chatz sh -ec \
  'printf "{\"completedAt\":\"%s\",\"driver\":\"postgres\"}\\n" "$BACKUP_COMPLETED_AT" > /data/chatz-backup-status.json'
```

To restore it, stop the app, replace the database, load the dump, then start
the app again. This deliberately destroys the current PostgreSQL data:

```bash
docker compose stop chatz
docker compose exec -T postgres sh -c \
  'dropdb -U "$POSTGRES_USER" --if-exists "$POSTGRES_DB" &&
   createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
docker compose exec -T postgres sh -c \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  < chatz-postgres.sql
docker compose start chatz
```

For SQLite, stop the single app container first. SQLite may have active WAL and
shared-memory sidecar files, so archive `/data` as one unit; never copy only
`chatz.sqlite` from a running container:

```bash
docker stop chatz
docker run --rm --entrypoint /bin/tar --user 0:0 \
  --volume chatz-data:/data:ro \
  --volume "$PWD":/backup \
  psyb0t/chatz -C /data -czf /backup/chatz-sqlite-data.tar.gz .
docker start chatz

backup_completed_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
docker exec -e BACKUP_COMPLETED_AT="$backup_completed_at" chatz sh -ec \
  'printf "{\"completedAt\":\"%s\",\"driver\":\"sqlite\"}\\n" "$BACKUP_COMPLETED_AT" > /data/chatz-backup-status.json'
```

To restore that SQLite archive, stop the app, replace the full data directory,
restore ownership for Chatz's non-root user, and start it again. This destroys
the current SQLite database:

```bash
docker stop chatz
docker run --rm --entrypoint /bin/sh --user 0:0 \
  --volume chatz-data:/data \
  --volume "$PWD":/backup:ro \
  psyb0t/chatz -ec '
    rm -f /data/chatz.sqlite /data/chatz.sqlite-shm /data/chatz.sqlite-wal
    tar -C /data -xzf /backup/chatz-sqlite-data.tar.gz
    chown -R appuser:appuser /data
  '
docker start chatz
```

---

## Security notes

`make sec` runs the full security scan (govulncheck plus semgrep) against every
pinned version, merging the results into `sec.sarif`. Its govulncheck half
reports reachability, not merely advisories in modules present in the
dependency graph. Rerun it before a release; do not treat a past clean report
as a permanent security status. Dependency versions are pinned in `go.mod`,
checksummed in `go.sum`, and vendored.
`scripts/check_go_age.sh` rejects third-party releases younger than seven days,
giving maintainers time to identify and remove a compromised package.

---

## License

See [`LICENSE`](LICENSE).

---
