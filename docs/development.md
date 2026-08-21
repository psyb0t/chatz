# Development

The supported everyday workflow runs Go tooling inside the project's Docker
development image. That removes host-version drift and means a clean machine
needs Docker rather than a separately curated Go toolchain.

The Makefile has two execution shapes:

- ordinary build, lint, formatting, dependency, and vulnerability work runs
  in the development container with the repository mounted at `/work`;
- `make test` and `make test-unit` use the ordinary socketless container;
  integration, coverage, and browser API targets mount the Docker socket for
  Testcontainers.

The exact scripts and override lookup live in the
[framework Make-script deep dive](../scripts/make/servicepack/README.md).

## Common targets

| Target | Runs where | Purpose |
| --- | --- | --- |
| `make dev-image` | Docker build | Build the development image used by normal tooling. |
| `make test` | Docker | Race-enabled Go unit tests plus web unit tests. |
| `make test-unit` | Docker | Race-enabled Go unit tests only, optionally narrowed with `PKG=./path/...`. |
| `make test-integration` | Docker + socket | Uncached, race-enabled integration tests with a ten-minute timeout. |
| `make test-coverage` | Docker + socket | Integration-tagged Go coverage gate plus web unit tests. The default floor is `MIN_TEST_COVERAGE=85`. |
| `make test-api` | Docker + socket | Full app-image browser suite against Postgres and SQLite. |
| `make test-real` | Docker + socket | Opt-in live-provider and MCP tests using `.env`. |
| `make lint` | Docker | Go/shell lint plus Prettier and Svelte checks. |
| `make lint-fix` | Docker | Apply supported Go and shell formatting fixes. Review its diff. |
| `make lint-fix-web` | Docker | Format the web app, then run Svelte checks. |
| `make format` | Docker | Run gofumpt and shfmt. |
| `make sec` | Docker | Full security scan: `govulncheck` + semgrep, merged into `sec.sarif` for the Security tab. Fails on any finding. |
| `make generate` | Docker | Execute package-local `go:generate` directives. |
| `make build` | Docker | Build a static executable under `build/`. |
| `make docker-build` | Docker | Build the production image. |
| `make run-dev` | Docker | Build/run the development image with the race detector. |

Use `make help` as the current target list. Do not replace these calls with a
host `go test` or host linter in project automation: that gives two different
toolchains two chances to disagree.

## Tests, integration tests, and coverage

`make test` is the fast default: race-enabled Go unit tests plus web unit tests,
with no Docker socket and no test infrastructure. `make test-integration` adds
the integration build tag, disables the Go test cache, and applies a ten-minute
timeout. Use it for changes that touch the database, HTTP server, MCP manager,
or other real-infrastructure paths. `make test-unit` runs only the Go unit suite
and accepts `PKG` when a narrow package run is useful.

`make test-coverage` also runs the race detector. It instruments every module
package (`-coverpkg=<module>/...`) with the integration build tag, so a test
under `tests/` credits coverage to the production package it drives. It merges
coverage emitted by the production app container through
`SERVICEPACK_COVDATA_DIR`. The gate excludes command wiring, test harnesses,
generated `*.gen.go`, and service-manager mocks. It then runs `make test-web`.
Override the threshold deliberately:

```bash
make test-coverage MIN_TEST_COVERAGE=95
```

The target writes `coverage-percent.txt` for the badge workflow and removes
the temporary coverage profiles. Testcontainers needs access to the Docker
daemon; if Docker cannot create containers, fix Docker access rather than
working around the integration tests.

Integration tests live under `tests/integration/` and use the shared
`tests/testinfra/` harness. They exercise real Postgres, the HTTP server, chat
turn persistence, MCP lifecycle, model discovery, and context selection.
`tests/api/` is a separate browser tier. It builds the production image and
drives it through a headless browser against both Postgres and SQLite. The
Docker runner uses the host network, so testcontainers' host-published ports
are reachable from the test process. `tests/` is listed in
`.servicepackupdateignore`, so a framework update never overwrites it.

## Build outputs and runtime identity

`make build` uses the pinned Go build image, installs the static-build
requirements inside that temporary container, and writes `build/<module-tail>`
back with your host UID/GID. It injects three values with linker flags:

- `main.appName`: the final segment of the module path; it sets the binary
  name and root Cobra command name;
- `main.buildCommit`: the checked-out `HEAD` commit when available.
- `main.buildVersion`: the exact Git tag at `HEAD`, or `dev` for an untagged
  source build.

`cmd/main.go` puts those into the global log scope as `binary`, `commit`, and
`version`.
Build output is therefore traceable without hardcoding identity in service
code. A source tree without a resolvable Git `HEAD` builds, but has no commit
field to inject.

`make docker-build` is a separate production-image path. Do not assume a
Dockerfile build and `make build` have the same customization hooks; inspect
the relevant script before changing either.

## Dependencies and code generation

Use the Make targets so dependency metadata and `vendor/` stay synchronized:

```bash
make pkg-lock
make pkg-add PKG=example.com/module@v1.2.3
make pkg-update PKG=example.com/module
make pkg-upgrade
make pkg-remove PKG=example.com/module
```

The framework commits Go's `vendor/` tree. Do not edit vendored sources by
hand. `make service` already regenerates service registration; use
`make service-registration` after manual service changes. Treat
`internal/pkg/services/services.gen.go` as generated output.

## Override, do not fork framework plumbing

`Makefile` includes `Makefile.servicepack`. Define the same target in your
project Makefile to replace a framework target, or add your own targets next
to it. Framework script lookup prefers `scripts/make/<script>.sh` over
`scripts/make/servicepack/<script>.sh`.

```bash
cp scripts/make/servicepack/test.sh scripts/make/test.sh
# edit scripts/make/test.sh for project-specific behavior
```

Likewise, project `Dockerfile`, `Dockerfile.dev`, `cmd/init.go`, and
`cmd/commands.go` are yours. The framework-owned versions are updated by
`make servicepack-update`; see [framework updates](framework-updates.md).

## Before you hand off a change

For a normal code change, run the narrowest useful target first, then the
relevant broader check. For example, run `make test` and `make lint` for a
unit-sized change. Run `make test-coverage` after changing backend behavior,
and `make test-api` after changing an end-to-end user flow.

Read [getting started](getting-started.md) for ownership boundaries and
[architecture](architecture.md) for where a change should live.
