---
title: "Development"
description: "Build, test and change chatz from a checkout: Docker-only Make targets, test tiers, code generation, dependencies, the docs site and the CI pipeline."
nav_order: 14
has_children: true
---

# Development

Docker is the only toolchain you need. Every Make target runs Go, Node and Python tooling inside the project's development image, built from [`Dockerfile.dev`](https://github.com/psyb0t/chatz/blob/main/Dockerfile.dev), so a clean machine needs Docker, Make and Bash rather than a curated Go and Node setup.

chatz is already a servicepack-based application. Do not run `make own` in this repository: that command is for a fresh clone of servicepack, and it replaces the Git and module setup.

The Makefile has two execution shapes:

- ordinary build, lint, formatting, dependency and security work runs in the development container with the repository mounted at `/work`;
- `make test` and `make test-unit` use that same socketless container, while the integration, coverage, browser and live-provider targets also mount the Docker socket and use the host network for testcontainers.

The script lookup and override mechanism is described in the [framework Make-script README](https://github.com/psyb0t/chatz/blob/main/scripts/make/servicepack/README.md). Contributor rules for coding agents and people live in [`AGENTS.md`](https://github.com/psyb0t/chatz/blob/main/AGENTS.md) and [`.agents/rules/`](https://github.com/psyb0t/chatz/tree/main/.agents/rules).

## Common targets

| Target | Runs where | Purpose |
| --- | --- | --- |
| `make dev-image` | Docker build | Build the development image used by every other target. |
| `make run` | Docker Compose | Build and start the stack: Postgres and chatz, or chatz alone on SQLite when `.env` says so. |
| `make run-showcase` | Docker Compose | The same stack with demo mode on. |
| `make test` | Docker | Race-enabled Go unit tests plus the web unit tests. |
| `make test-unit` | Docker | Race-enabled Go unit tests only, optionally narrowed with `PKG=./path/...`. |
| `make test-integration` | Docker + socket | Uncached, race-enabled integration tests with a ten-minute timeout. |
| `make test-coverage` | Docker + socket | Integration-tagged Go coverage gate plus the web unit tests. The floor is `MIN_TEST_COVERAGE=85`. |
| `make test-api` | Docker + socket | The production image driven by a headless browser against Postgres and SQLite. |
| `make test-real` | Docker + socket | Opt-in live-provider and MCP tests using `.env`. |
| `make lint` | Docker | Go and shell lint (shfmt, shellcheck, `go fix`, golangci-lint) plus prettier and svelte-check for the web app. |
| `make lint-fix` | Docker | Apply the Go and shell lint fixes. Review its diff. |
| `make lint-fix-web` | Docker | Format the web app, then run svelte-check. |
| `make format` | Docker | Format Go source files. |
| `make sec` | Docker | `govulncheck` and semgrep, merged into `sec.sarif`. Fails on any finding. |
| `make generate` | Docker | Run every `go:generate` directive and build the web app. |
| `make web-dev` | Docker | The web dev server on http://localhost:5173. Set `CHATZ_API_TARGET` to point it at a running chatz. |
| `make web-embed` | Docker | Build the web app and copy it into the `go:embed` directory for a local `go build`. |
| `make build` | Docker | Build a static binary into `build/`. |
| `make docker-build` | Docker | Build the production image. |
| `make docs` | Docker | Build the documentation site into `.testing/docs-site`. |

Use `make help` for the full, current list. Do not replace these calls with a host `go test` or host linter in project automation: that gives two toolchains two chances to disagree.

## Where a change goes

| You are changing | Start here |
| --- | --- |
| HTTP API contract | [`api/api.yml`](https://github.com/psyb0t/chatz/blob/main/api/api.yml), then `make generate` and `make web-gen-api` |
| HTTP handlers and service wiring | [`internal/pkg/http/server/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/http/server) and [`internal/pkg/services/http-server/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/services/http-server) |
| Chat turns, history, streaming or demo replies | [`internal/pkg/core/chats/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/core/chats) |
| MCP configuration or tool transport | [`internal/pkg/mcp/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/mcp) |
| Database schema or repositories | [`internal/pkg/db/migrations/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/db/migrations) (both the Postgres files and the `sqlite/` ones), [`internal/pkg/db/models/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/db/models) and [`internal/pkg/db/repositories/`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/db/repositories) |
| Svelte UI | [`web/src/`](https://github.com/psyb0t/chatz/tree/main/web/src) |
| Generative UI catalog | [`web/src/lib/render/`](https://github.com/psyb0t/chatz/tree/main/web/src/lib/render), then `make genui-prompt` |
| Runtime configuration | [`.env.example`](https://github.com/psyb0t/chatz/blob/main/.env.example), [`internal/pkg/config/config.go`](https://github.com/psyb0t/chatz/blob/main/internal/pkg/config/config.go) and the [configuration](configuration.md) page |

The generated boundaries are intentional. `api/api.yml` generates the strict server types, the Go client and the web API types. The gorm repositories and the embedded GenUI prompt have their own generators. Change the source, then run `make generate`; never hand-edit generated output. CI fails when committed generated files differ from a fresh `make generate`.

## Tests, integration tests and coverage

`make test` is the fast default: race-enabled Go unit tests plus the web unit tests, with no Docker socket and no test infrastructure. `make test-integration` adds the `integration` build tag, disables the Go test cache and applies a ten-minute timeout. Use it for changes that touch the database, HTTP server, MCP manager or other real-infrastructure paths. `make test-unit` runs only the Go unit suite and accepts `PKG` when a narrow package run is useful.

`make test-coverage` also runs the race detector. It instruments every module package (`-coverpkg=<module>/...`) with the integration build tag, so a test under `tests/` credits coverage to the production package it drives. It merges coverage emitted by the production app container through `SERVICEPACK_COVDATA_DIR`. The gate excludes command wiring, test harnesses, generated `*.gen.go` and service-manager mocks. It then runs `make test-web`. Override the threshold deliberately:

```bash
make test-coverage MIN_TEST_COVERAGE=95
```

`MIN_TEST_COVERAGE` in the Makefile is a ratchet: raise it when the measured number clears the next step, never lower it to make a build pass. The target writes `coverage-percent.txt` for the badge workflow and removes the temporary coverage profiles. Testcontainers needs access to the Docker daemon; if Docker cannot create containers, fix Docker access rather than working around the integration tests.

Integration tests live under [`tests/integration/`](https://github.com/psyb0t/chatz/tree/main/tests/integration) and [`tests/boot/`](https://github.com/psyb0t/chatz/tree/main/tests/boot) and use the shared [`tests/testinfra/`](https://github.com/psyb0t/chatz/tree/main/tests/testinfra) harness. They exercise real Postgres, the HTTP server, chat turn persistence, the MCP lifecycle, model discovery and context selection. Under `go test` every upstream resolves to a scripted fake, so no test reaches a real provider by accident.

[`tests/api/`](https://github.com/psyb0t/chatz/tree/main/tests/api) is a separate browser tier. It builds the production image and drives it through a headless browser against both Postgres and SQLite, with a fake upstream and an MCP fixture server. These variables narrow or extend it:

| Variable | Default | Effect |
| --- | --- | --- |
| `API_RUN` | empty | Go test-name regex for a focused flow. |
| `API_DB_DRIVERS` | `postgres sqlite` | The databases to run against. |
| `API_PARALLEL` | `2` | Full stacks running at once. Each is a database, the app and a browser. |
| `API_TIMEOUT` | `1800s` | Covers the one-time image build plus every driver. |
| `API_REAL` | empty | `1` loads `.env` so the live-chat driver runs a real streamed turn. |

[`tests/real/`](https://github.com/psyb0t/chatz/tree/main/tests/real) runs only through `make test-real`, against the providers in your `.env` with `CHATZ_FORCE_REAL_LLM=true`. It skips the live cases without a usable upstream. `tests/` is listed in `.servicepackupdateignore`, so a framework update never overwrites it.

## Build outputs and runtime identity

`make build` compiles `./cmd` with `CGO_ENABLED=0` in the development container and writes `build/chatz`. It embeds whatever is in `internal/pkg/webassets/dist/`: the committed placeholder page unless you ran `make web-embed` first.

The production [`Dockerfile`](https://github.com/psyb0t/chatz/blob/main/Dockerfile) is a separate path. It builds the web app with pnpm, copies it into the `go:embed` directory, and compiles the binary with linker flags that stamp the `APP_VERSION` and `GIT_COMMIT` build arguments into `internal/pkg/buildinfo` and set the binary name. Those values appear on every log line and on the admin readiness page. `make build` stamps only the binary name.

## Dependencies and code generation

Use the Make targets so dependency metadata and `vendor/` stay in step:

```bash
make pkg-lock
make pkg-add PKG=example.com/module@v1.2.3
make pkg-update PKG=example.com/module
make pkg-upgrade
make pkg-remove PKG=example.com/module
```

The Go targets check a release's age before taking it. The repository commits Go's `vendor/` tree; do not edit vendored sources by hand.

The web app uses pnpm only, with `--ignore-scripts`. Its dependency targets (`make web-pkg-add PKG=name`, `web-pkg-update`, `web-pkg-upgrade`, `web-pkg-remove`) first move `minimumReleaseAge` in `web/pnpm-workspace.yaml` to a 7-day floor. Node modules live in the `chatz-web-node-modules` Docker volume.

`make service` regenerates service registration; use `make service-registration` after manual service changes. Treat `internal/pkg/services/services.gen.go` as generated output.

## Override, do not fork framework plumbing

`Makefile` includes `Makefile.servicepack`. Define the same target in the project `Makefile` to replace a framework target, or add new targets next to it. Framework script lookup prefers `scripts/make/<script>.sh` over `scripts/make/servicepack/<script>.sh`.

```bash
cp scripts/make/servicepack/test.sh scripts/make/test.sh
# edit scripts/make/test.sh for project-specific behavior
```

`Dockerfile`, `Dockerfile.dev`, `cmd/init.go` and `cmd/commands.go` belong to chatz. The framework-owned files are updated by `make servicepack-update`; see [framework updates](framework-updates.md).

## Documentation site

The site you are reading is the [`docs/`](https://github.com/psyb0t/chatz/tree/main/docs) directory, built by GitHub Pages from `main` with Jekyll and the Just the Docs theme. `make docs` builds it with the same image GitHub uses, into `.testing/docs-site`.

- Every page needs front matter with a unique `title` and a `description` of 50 to 170 characters, and one `#` heading.
- Link between pages with the relative `.md` path. Link files outside `docs/` with the full `https://github.com/psyb0t/chatz/blob/main/<path>` URL.
- A change to a setting, endpoint or behavior updates its page in the same change.

## CI

[`.github/workflows/pipeline.yml`](https://github.com/psyb0t/chatz/blob/main/.github/workflows/pipeline.yml) runs on every push, to any branch:

1. The code job runs `make lint`, `make test-coverage`, `make sec` (uploading `sec.sarif` to code scanning) and `make generate`, failing on any drift in generated files. It needs no secrets.
2. On `main`, the image is built for `linux/amd64` and `linux/arm64` and pushed as `psyb0t/chatz:latest` and `psyb0t/chatz:sha-<commit>`, then scanned with Grype. The coverage, version and license badges are updated.
3. A `v<semver>` tag waits for the `main` run of the same commit to pass, points the release tag at that run's `sha-<commit>` image without rebuilding, and creates the GitHub release.

Other workflows in `.github/workflows/` come from servicepack and handle issue syncing, mirroring and collaborator checks.

## Before you hand off a change

For a normal code change, run the narrowest useful target first, then the relevant broader check. For example, run `make test` and `make lint` for a unit-sized change. Run `make test-coverage` after changing backend behavior, and `make test-api` after changing an end-to-end user flow. `pre-commit.sh` runs `make lint && make test-coverage`.

Read [architecture](architecture.md) for where a change should live and [services and lifecycle](services-and-lifecycle.md) when changing service startup or shutdown behavior.
