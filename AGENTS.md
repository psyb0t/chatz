# chatz

Instructions for coding agents (Codex, Claude Code and others) and for people changing this repository. People who want to run chatz want the documentation site, https://psyb0t.github.io/chatz/, not this file.

## Agent setup

`CLAUDE.md` is a committed symlink to `AGENTS.md`, and `.claude` is a committed symlink to `.agents`, so Claude Code reads the same instructions, rules and skills as Codex. Claude's Edit and Write tools refuse to write through a symlink, so edit `AGENTS.md` and `.agents/rules/*.md` directly.

## What it is

A self-hosted AI chat app published as the `psyb0t/chatz` Docker image. One Go binary serves a versioned JSON and SSE API under `/api/v1` (contract in `api/api.yml`) and an embedded SvelteKit single-page app from the same origin. Chats stream from OpenAI-compatible or Anthropic upstreams through [elelem](https://github.com/psyb0t/elelem), call tools on admin-managed MCP servers, and can render generative UI components from fenced `spec` blocks in the model output. Data lives in Postgres or SQLite.

The app is built on [servicepack](https://github.com/psyb0t/servicepack), a clone-and-own Go service framework. Servicepack owns part of the tree and overwrites it on `make servicepack-update`; `.agents/rules/servicepack-boundary.md` lists which part.

## Architecture

- `internal/pkg/services/http-server`: the one servicepack service. `New` parses config, connects the database (running migrations), builds the secrets box, metrics, the upstream registry, readiness, the MCP manager and the web assets, then serves the API plus the internal metrics and pprof listeners.
- `internal/pkg/http/server`: echo router and the handlers over the generated strict server interface (`handler_*.go`), session auth middleware, `requireAdmin`, the error envelope and SPA serving. The API mounts at `/api/v1`; `/healthz` sits at the root.
- `internal/pkg/core/auth`: bcrypt passwords, opaque session tokens (only the SHA-256 hash is stored), first-run admin setup and passwordless single-user login.
- `internal/pkg/core/chats`: durable turns. Model or fixed-response selection, per-chat locks, history selection, SSE streaming, checkpoints and persistence. `core/chat` adapts elelem's agent loop to the SSE stream. The turn contract is in `internal/pkg/core/chats/README.md`.
- `internal/pkg/upstreams`: model discovery across `CHATZ_UPSTREAMS`, routing a model ID to its client, and the redacted health snapshots.
- `internal/pkg/mcp`: stdio and streamable HTTP MCP connections, status, retries, tool routing as `<server>__<tool>`, and `.mcp.json` import. Details in `internal/pkg/mcp/README.md`.
- `internal/pkg/db`: Postgres and SQLite connections, embedded migrations (`migrations/` and `migrations/sqlite/`), gorm models and the generated repositories.
- `internal/pkg/secrets`: AES-256-GCM sealing of stored MCP headers and stdio environment maps with `CHATZ_SECRETS_KEY`.
- `internal/pkg/config`: the `CHATZ_*` settings and the `CHATZ_UPSTREAMS` JSON validation.
- `internal/pkg/operations`, `metrics`, `usage`, `logging`: the admin readiness snapshot and backup marker, the Prometheus registry, per-call LLM usage rows and metrics, and the `chatz.log` file sink with redaction.
- `web/`: the SvelteKit static SPA. `web/src/lib/render/` is the generative UI renderer and its component catalog.

Request path: browser -> session cookie -> handler -> `core/chats` -> elelem driver (with MCP tools from `internal/pkg/mcp`) -> upstream, streamed back as SSE frames and saved as one turn.

## Stack

- Go `1.26.9` (`go.mod`, `Dockerfile`, `Dockerfile.dev`), static build with `CGO_ENABLED=0` for the image. The dev image keeps cgo on for `-race`.
- HTTP: echo v4, oapi-codegen strict server and client from `api/api.yml`, aichteeteapee for header names and error codes.
- LLM: elelem with its `openai` and `anthropic` drivers. MCP: the official Go SDK.
- Storage: gorm with gorm/gen repositories, Postgres (default) or SQLite through `glebarez/sqlite`.
- Logging and errors: slogging, ctxscope, ctxerrors, common-go sentinels. Config: gonfiguration.
- Web: SvelteKit 2, Svelte 5, TypeScript, vitest, prettier, svelte-check, pnpm `10.15.0` only.
- Image: three-stage `Dockerfile` (node build of the SPA, Go build that embeds it, Alpine runtime as a non-root user with `/data` for SQLite).
- CI: `.github/workflows/pipeline.yml` calling `psyb0t/reusable-github-workflows`.

## Layout

| Path | What it holds |
| --- | --- |
| `cmd/` | `main.go` (servicepack), `init.go` and `commands.go` (project hooks), `repogen/` (the gorm repository generator) |
| `api/api.yml` | The OpenAPI contract for `/api/v1` |
| `internal/pkg/services/http-server/` | Service wiring, startup and shutdown |
| `internal/pkg/http/server/` | Router, middleware and handlers |
| `internal/pkg/http/api/`, `pkg/http/api/client/` | Generated server types and the generated public Go client |
| `internal/pkg/core/` | `auth`, `chat` and `chats` domain code, including fixed demo replies and the embedded GenUI prompt |
| `internal/pkg/db/` | Connections, both migration sets, models and generated repositories |
| `internal/pkg/` (other) | `config`, `mcp`, `upstreams`, `secrets`, `logging`, `metrics`, `operations`, `usage`, `heartbeat`, `tiktoken`, `webassets`, `buildinfo` |
| `pkg/rebound/` | Bounded exponential backoff retries |
| `internal/app/`, `internal/pkg/service-manager/`, `pkg/runner/` | Servicepack framework code |
| `web/` | SvelteKit SPA, embedded into the binary at image build time |
| `tests/` | `integration` and `api` tagged suites, `boot`, `real`, the shared `testinfra` harness and the fake upstream and MCP server fixtures |
| `scripts/make/` | Project overrides of framework Make scripts (`lib/dev.sh` holds the container helpers); `scripts/make/servicepack/` is framework code |
| `docs/` | The documentation site (GitHub Pages, Jekyll, Just the Docs) |
| `.agents/rules/` | Rules for changing this repository |
| `.github/workflows/` | `pipeline.yml` plus the servicepack housekeeping workflows |

## Commands

Docker, Make and Bash are enough. Every target runs its tools in the `chatz-dev` image built from `Dockerfile.dev`.

```bash
make help              # every target
make run               # docker compose stack (Postgres, or SQLite from .env)
make lint              # lint-go (shfmt, shellcheck, go fix, golangci-lint) + lint-web (prettier, svelte-check)
make test              # Go unit tests with -race plus the web unit tests, no Docker socket
make test-integration  # integration-tagged tests over a testcontainers Postgres
make test-coverage     # integration-tagged Go coverage gate (MIN_TEST_COVERAGE=85) plus web tests, what CI runs
make test-api          # the production image driven by a headless browser on Postgres and SQLite
make test-real         # opt-in live-provider tests using .env
make sec               # govulncheck and semgrep, writes sec.sarif
make generate          # go generate ./... (repositories, API server and client, GenUI prompt) plus the web build
make docs              # build the documentation site into .testing/docs-site
```

`pre-commit.sh` runs `make lint && make test-coverage`.

## Project-specific landmines

- Never edit servicepack-owned files. The list and the reasons are in `.agents/rules/servicepack-boundary.md`.
- Generated files are never hand-edited: `internal/pkg/http/api/api.gen.go`, `pkg/http/api/client/client.gen.go`, `internal/pkg/db/repositories/*.gen.go`, `internal/pkg/core/chats/prompts/genui_instructions.gen.txt`, `internal/pkg/services/services.gen.go` and `web/src/lib/api/schema.d.ts`. CI fails on drift after `make generate`.
- A schema change needs a migration under both `internal/pkg/db/migrations/` and `internal/pkg/db/migrations/sqlite/` with the same number. SQLite never replays the Postgres history.
- `internal/pkg/webassets/dist/` is gitignored except `index.html`, a placeholder that keeps `go build` green. `make web-embed` fills it locally; the image build copies the real SPA in.
- `make test` runs no tagged tests. The `tests/` suites need `make test-integration`, `make test-coverage` or `make test-api`.
- Under `go test` every upstream resolves to elelem's scripted test double unless `CHATZ_FORCE_REAL_LLM=true`, which only `make test-real` should see.
- The web app uses pnpm with `--ignore-scripts` and a 7-day `minimumReleaseAge`. Use the `web-pkg-*` targets, never npm.

## Rules

Rules for every change live in `.agents/rules/`, one file per topic. Read the ones that match what you are touching before editing.
