# Architecture

Chatz is a Servicepack-based application. Servicepack supplies the process
lifecycle and in-process service manager. Chatz currently registers one HTTP
service that owns the API, embedded SPA, database connection, MCP manager, and
LLM upstream registry.

## Runtime shape

```
cmd/main.go
  ├─ set global log scope: binary, commit
  ├─ services.Init()                         generated factory registration
  └─ Cobra "run"
       └─ pkg/runner
            ├─ signal / parent-context handling
            └─ internal/app.App
                 ├─ pre-run hooks
                 ├─ internal/pkg/service-manager
                 │    └─ service factories → services running concurrently
                 └─ post-stop hooks
```

`cmd/main.go` is intentionally small: establish process identity, register
generated service factories, expose `run` and command namespaces, then pass
control to the runner. The [runner README](../pkg/runner/README.md) explains
the signal and deadline rules.

`internal/app` owns application-level hooks and delegates service execution to
the [service manager](../internal/pkg/service-manager/README.md). `App` is the
place for whole-process behavior; individual services should not reach across
into siblings to create their own lifecycle graph.

## Deployment shape

One Chatz container runs one binary. The binary serves the versioned JSON/SSE
API and the static SPA from the same origin. Postgres is the default store;
SQLite is available for a single process using one local Docker volume. See the
root [README](../README.md) for the deployment and backup commands.

The HTTP service constructs the database, generated repositories, auth service,
MCP manager, upstream registry, and web-asset server. They run in the same
process and share its cancellation path. Adding separately deployed workers or
other processes would require explicit network or queue contracts. A
Servicepack dependency declaration only orders in-process services.

## Source ownership

| Path | Role | Ownership after `make own` |
| --- | --- | --- |
| `cmd/main.go` | Process entry point and root CLI. | Framework |
| `cmd/init.go` | Extra handlers and application hooks. | Project |
| `cmd/commands.go` | App-level CLI commands. | Project |
| `internal/app/` | App lifecycle wrapper. | Framework |
| `internal/pkg/service-manager/` | Concurrency, dependency, retry, and stop semantics. | Framework |
| `internal/pkg/services/` | Business services and `services.gen.go`. | Project; generated registration is not hand-edited. |
| `pkg/runner/` | Signal-aware runner. | Framework |
| `scripts/make/servicepack/` | Updateable Make implementations. | Framework |
| `scripts/make/` | Project-specific target overrides. | Project |
| `Makefile.servicepack` | Framework Make target definitions. | Framework |
| `Makefile` | Project targets and overrides. | Project |

Framework-owned means an update may replace it. Project-owned means the
update's normal exclusion policy preserves it. See
[framework updates](framework-updates.md) before changing that boundary.

## Registration and lazy construction

`make service-registration` runs the generator that discovers `Service`
implementations and writes `internal/pkg/services/services.gen.go`. Chatz uses
that generated registration for its HTTP service. The generated code registers
factories rather than fully constructed services.

That distinction matters:

- `run` instantiates enabled factories;
- a per-service Cobra command instantiates only that one service;
- connections and config parsing happen in a service's `New`, not at package
  import time.

This keeps command execution from accidentally opening every database/client
in the project. Details are in the
[service-manager README](../internal/pkg/service-manager/README.md).

## Observability and configuration

Logging starts with the `slogging` handler setup and flows through `ctxscope`.
The binary and build commit are global scope; the service manager adds a
`service` field while it runs or stops a service. Preserve and extend the
context passed into `Run`; it carries cancellation and those fields together.

Configuration is typed and parsed at each service boundary with
`gonfiguration`. Chatz configuration is documented in the root
[README](../README.md) and [`.env.example`](../.env.example).

## Build and test topology

The Makefile runs tooling in Docker. Test targets additionally receive Docker
access for Testcontainers. The development model, coverage boundary, and
override mechanism are documented in [development](development.md), with the
exact script behavior in the [Make-script README](../scripts/make/servicepack/README.md).
