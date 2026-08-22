# Getting started with Chatz

## Run Chatz with Docker Compose

Docker Compose is the normal self-hosted path from a Chatz checkout. It builds
the application image, starts Postgres, and serves Chatz at
`http://localhost:8080`.

```bash
cp .env.example .env
touch chatz.log
docker compose up --build
```

The first run works without an LLM. Open `/setup` to create the admin, then add
an upstream later from `.env`. Postgres data lives in the `pgdata` Docker volume
and `chatz.log` is the host-side diagnostic log.

```bash
docker compose down
```

Set `CHATZ_DB_DRIVER=sqlite` in `.env` and use this command for a single Chatz
process backed by the persistent `chatzdata` Docker volume:

```bash
docker compose up --build --no-deps chatz
```

SQLite does not migrate an existing Postgres database and does not support
shared storage. Use Postgres for replicas, networked storage, or an existing
database. See the root [README](../README.md#deploy) for a hardened `docker run`
deployment of a published image.

## Develop from a checkout

Chatz is already a Servicepack-based application. Do not run `make own` in this
repository. That command is for a disposable clone of Servicepack and replaces
its Git and module setup.

Docker is the required development toolchain. Make targets run Go, Node, and
Python tooling in containers.

## Work on one area

| You are changing | Start here |
| --- | --- |
| HTTP API contract | `api/api.yml`, then `make generate` |
| HTTP handlers and service wiring | `internal/pkg/http/server/` and `internal/pkg/services/http-server/` |
| Chat turns, history, streaming, or showcase replies | `internal/pkg/core/chats/` |
| MCP configuration or tool transport | `internal/pkg/mcp/` |
| Database schema or repositories | `internal/pkg/db/migrations/`, `internal/pkg/db/models/`, and `internal/pkg/db/repositories/` |
| Svelte UI | `web/src/` |
| Generative UI catalog | `web/src/lib/render/`, then `make genui-prompt` |
| Runtime configuration | `.env.example` and `internal/pkg/config/config.go` |

The generated boundaries are intentional. `api/api.yml` generates the strict
server/client types and web API types. Gorm repository generation and the
embedded GenUI prompt also have declared generators. Change the source, then
run `make generate`; never hand-edit generated output.

## Test the right layer

```bash
make test              # Go and web unit tests
make test-integration  # Postgres-backed integration tests
make test-coverage     # Integration-tagged Go coverage gate plus web tests
make test-api           # Production image + browser against both databases
make test-real          # Opt-in live-provider and MCP tests using .env
make lint               # Go, shell, and web checks
```

`make test` does not require a Docker socket and never contacts a live model.
`make test-integration`, `make test-coverage`, and `make test-api` start their
own Testcontainers fixtures. `make test-real` is the only test target that
uses the provider settings from `.env`; it skips the live cases without a
usable upstream.

See [development](development.md) for the exact test tiers, focused test
variables, dependency targets, and build behavior.

## Framework-owned code

Chatz extends Servicepack but does not modify its updateable implementation
layer for application behavior. Project-owned extensions belong in `Makefile`,
`scripts/make/`, `internal/pkg/services/`, and the application Dockerfiles.
Framework files, including `Makefile.servicepack`, `pkg/runner/`,
`internal/pkg/service-manager/`, and `scripts/make/servicepack/`, are replaced
by `make servicepack-update`.

Read [framework updates](framework-updates.md) before updating Servicepack.
Read [architecture](architecture.md) for runtime ownership and
[services and lifecycle](services-and-lifecycle.md) when changing service
startup or shutdown behavior.
