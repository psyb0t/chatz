---
title: "Architecture"
description: "How the chatz binary is put together: the servicepack runtime, the HTTP service and its parts, source ownership and lazy service construction."
nav_order: 13
has_children: true
---

# Architecture

chatz is a [servicepack](https://github.com/psyb0t/servicepack)-based application. Servicepack supplies the process lifecycle and the in-process service manager. chatz registers one HTTP service that owns the API, the embedded web app, the database connection, the MCP manager and the model upstream registry.

## The HTTP service

[`internal/pkg/services/http-server`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/services/http-server) builds everything when the `run` command starts it, in this order: parse the configuration, connect the database and run migrations, build the secrets box and the metrics registry, discover models on every upstream, set up the readiness snapshot, start connecting the enabled MCP servers in the background, and load the embedded web app. A configuration error at any step stops startup; an unreachable upstream or MCP server does not.

| Package | Role |
| --- | --- |
| [`internal/pkg/http/server`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/http/server) | echo router, session middleware, handlers over the generated strict interface, the error envelope and the web app fallback. |
| [`internal/pkg/core/auth`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/core/auth) | Accounts, bcrypt passwords, session tokens, setup and passwordless login. |
| [`internal/pkg/core/chats`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/core/chats) | Durable turns: model or demo selection, per-chat locks, history selection, SSE streaming, checkpoints and persistence. |
| [`internal/pkg/core/chat`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/core/chat) | Adapts elelem's tool-calling loop to the SSE stream. |
| [`internal/pkg/upstreams`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/upstreams) | Model discovery, routing a model ID to its upstream, concurrency slots, timeouts and health. |
| [`internal/pkg/mcp`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/mcp) | MCP connections, retries, tool routing and `.mcp.json` import. |
| [`internal/pkg/db`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/db) | Postgres and SQLite connections, embedded migrations, models and generated repositories. |
| [`internal/pkg/secrets`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/secrets) | AES-256-GCM sealing of stored MCP credentials. |
| [`internal/pkg/operations`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/operations), [`metrics`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/metrics), [`usage`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/usage), [`logging`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/logging) | Readiness, Prometheus metrics, per-call usage rows, the log file sink and redaction. |
| [`web/`](https://github.com/psyb0t/chatz/tree/main/web) | The SvelteKit web app, embedded into the binary through [`internal/pkg/webassets`](https://github.com/psyb0t/chatz/tree/main/internal/pkg/webassets). |

A chat message travels: browser, session cookie, handler, `core/chats` (history and settings), elelem driver with the chat's MCP tools, upstream. The reply streams back as SSE frames while checkpoints are saved, and the finished turn is written in one step. The turn rules are in the [chat orchestration README](https://github.com/psyb0t/chatz/blob/main/internal/pkg/core/chats/README.md).

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
control to the runner. The [runner README](https://github.com/psyb0t/chatz/blob/main/pkg/runner/README.md) explains
the signal and deadline rules.

`internal/app` owns application-level hooks and delegates service execution to
the [service manager](https://github.com/psyb0t/chatz/blob/main/internal/pkg/service-manager/README.md). `App` is the
place for whole-process behavior; individual services should not reach across
into siblings to create their own lifecycle graph.

## Deployment shape

One Chatz container runs one binary. The binary serves the versioned JSON/SSE
API and the static SPA from the same origin. Postgres is the default store;
SQLite is available for a single process using one local Docker volume. See
[deployment](deployment.md) for running it.

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
[service-manager README](https://github.com/psyb0t/chatz/blob/main/internal/pkg/service-manager/README.md).

## Observability and configuration

Logging starts with the `slogging` handler setup and flows through `ctxscope`.
The binary and build commit are global scope; the service manager adds a
`service` field while it runs or stops a service. Preserve and extend the
context passed into `Run`; it carries cancellation and those fields together.

Configuration is typed and parsed at each service boundary with
`gonfiguration`. Every chatz setting is on the [configuration](configuration.md)
page, and [`.env.example`](https://github.com/psyb0t/chatz/blob/main/.env.example) has the usual ones.

## Build and test topology

The Makefile runs tooling in Docker. Test targets additionally receive Docker
access for Testcontainers. The development model, coverage boundary, and
override mechanism are documented in [development](development.md), with the
exact script behavior in the [Make-script README](https://github.com/psyb0t/chatz/blob/main/scripts/make/servicepack/README.md).
