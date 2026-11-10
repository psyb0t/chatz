---
title: "Configuration"
description: "Every chatz environment variable with its default: HTTP listeners, Postgres and SQLite, auth, secrets, upstream limits, backups and logging."
nav_order: 3
---

# Configuration

chatz reads all of its configuration from environment variables when it starts. There is no config file. Changing a variable needs a restart. The repository's [`.env.example`](https://github.com/psyb0t/chatz/blob/main/.env.example) lists the usual ones with comments; copy it to `.env`, which Docker Compose loads on its own and `docker run` takes with `--env-file .env`.

A value that does not parse, such as `CHATZ_DB_PORT=abc` or a malformed duration, stops startup. Durations use Go syntax: `500ms`, `45s`, `5m`, `24h`. The settings are defined in [`internal/pkg/config/config.go`](https://github.com/psyb0t/chatz/blob/main/internal/pkg/config/config.go).

## HTTP listeners

| Variable | Default | What it does |
| --- | --- | --- |
| `CHATZ_HTTP_LISTENADDRESS` | `:8080` | Address of the public listener: the web app, `/api/v1` and `/healthz`. |
| `CHATZ_METRICS_LISTENADDRESS` | `:9091` | Address of the internal Prometheus listener, which serves `/metrics`. An empty value turns it off. |
| `CHATZ_PROFILING_LISTENADDRESS` | `:6060` | Address of the internal pprof listener under `/debug/pprof/`. An empty value turns it off. |

Neither internal listener is published by the example `docker run` command or by Compose. A port that fails to bind logs a warning and leaves the API running. [Monitoring](monitoring.md) covers what they serve.

## Database

| Variable | Default | What it does |
| --- | --- | --- |
| `CHATZ_DB_DRIVER` | `postgres` | `postgres` or `sqlite`. Anything else stops startup. |
| `CHATZ_DB_SQLITE_PATH` | `/data/chatz.sqlite` | SQLite only. An absolute path to a regular file directly under `/data`. |
| `CHATZ_DB_SQLITE_BUSY_TIMEOUT` | `5s` | SQLite only. How long a statement waits for a lock. Must be positive. |
| `CHATZ_DB_HOSTNAME` | `localhost` | Postgres only. Server host name. Compose sets it to `postgres`. |
| `CHATZ_DB_PORT` | `5432` | Postgres only. Server port. |
| `CHATZ_DB_USERNAME` | `chatz` | Postgres only. User name. |
| `CHATZ_DB_PASSWORD` | `chatz` | Postgres only. Password. Change it for anything but a local stack. |
| `CHATZ_DB_NAME` | `chatz` | Postgres only. Database name. |
| `CHATZ_DB_ISSSL` | `false` | Postgres only. `true` connects with `sslmode=require`; `false` uses `sslmode=disable`. Certificate verification modes are not available. |

Migrations run on every start, before the server accepts requests. [Deployment](deployment.md#sqlite-or-postgres) helps choose a driver.

## Accounts and secrets

| Variable | Default | What it does |
| --- | --- | --- |
| `CHATZ_AUTH_PASSWORDLESS` | `false` | When `true` and exactly one user exists, a visitor without a session is signed in as that user. With a second user, everyone logs in normally. |
| `CHATZ_SECRETS_KEY` | empty | Base64 of exactly 32 random bytes. Encrypts stored MCP HTTP headers and stdio environment variables with AES-256-GCM. Generate one with `openssl rand -base64 32`. |
| `CHATZ_SESSION_SECRET` | empty | Reserved. Nothing reads it yet: sessions are random tokens stored in the database. |

Without `CHATZ_SECRETS_KEY`, chatz starts and logs a warning, and any attempt to save an MCP header or environment variable fails. A key that is set but is not valid base64 of 32 bytes stops startup. Keep the same key across restarts: a different key cannot decrypt what the old one stored.

## Models

| Variable | Default | What it does |
| --- | --- | --- |
| `CHATZ_UPSTREAMS` | empty | JSON array of model upstreams. Empty runs the app with no models. The format is on [Models and upstreams](models.md). |
| `CHATZ_DEFAULT_MODEL` | empty | A discovered model ID that new chats start with. Startup fails when no upstream lists it. |
| `CHATZ_UPSTREAM_CONNECT_TIMEOUT` | `10s` | Bounds model discovery and the connection setup of each provider request. |
| `CHATZ_UPSTREAM_FIRST_TOKEN_TIMEOUT` | `45s` | How long a request waits for the first streamed token. Must not exceed the turn timeout. |
| `CHATZ_UPSTREAM_TURN_TIMEOUT` | `5m` | The total time one model request may take. |
| `CHATZ_UPSTREAM_CONCURRENCY` | `8` | Requests in flight per upstream. Further requests queue for a slot. |
| `CHATZ_SHOWCASE_MODE` | `false` | Answers the exact demo prompts with fixed replies. See [Chats](chats.md#demo-mode). |

Each upstream names the variable that holds its API key with `apiKeyEnv`, for example `OPENAI_API_KEY`. That variable must be set and non-empty, or startup stops. Compose forwards the whole `.env` file to the container, so any name works there.

All timeouts and the concurrency must be greater than zero. The first-token and turn timers start after the request gets a concurrency slot, so time spent waiting in the queue does not count against them.

## Backups

| Variable | Default | What it does |
| --- | --- | --- |
| `CHATZ_BACKUP_STATUS_PATH` | `/data/chatz-backup-status.json` | Marker file your backup job writes after a successful backup. Must be an absolute path directly under `/data`. |
| `CHATZ_BACKUP_MAX_AGE` | `24h` | How old the marker may get before the admin readiness page calls the backup stale. Must be positive. |

chatz does not make backups. It reads the marker so the admin can see when the last one finished. The file format is on [Deployment](deployment.md#backups).

## Logging

| Variable | Default | What it does |
| --- | --- | --- |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `LOG_FORMAT` | `text` | `json` or `text`. `.env.example` and Compose set `json`. |
| `LOG_ADD_SOURCE` | `false` | `true` adds the source file and line to each log line. `.env.example` and Compose set `true`. |

An invalid logging value makes the binary exit as it starts. `LOG_LEVEL=debug` logs the full prompt sent to the model, including ordinary chat text, so keep it out of shared log systems. The separate `chatz.log` file always records at debug level; see [Monitoring](monitoring.md#logs).

## Process

| Variable | Default | What it does |
| --- | --- | --- |
| `RUNNER_SHUTDOWNTIMEOUT` | `10s` | How long chatz may take to stop after `SIGINT` or `SIGTERM`. |
| `SERVICES_ENABLED` | empty | Comma-separated services to run. chatz has one, `http-server`, so leave it empty. |
| `GIT_COMMIT` | empty | Commit shown in logs and on the readiness page when the binary was built without one. |

## Build and Compose variables

These are read by `docker-compose.yml` or the image build, not by the app.

| Variable | Default | What it does |
| --- | --- | --- |
| `CHATZ_HTTP_PORT` | `8080` | Host port Compose publishes on `127.0.0.1`. |
| `CHATZ_APP_VERSION` | `dev` | Version stamped into the binary when Compose builds the image. |
| `GIT_COMMIT` | empty | Commit stamped into the binary at build time. |

## Test only

`CHATZ_FORCE_REAL_LLM` (default `false`) matters only inside Go tests, where every upstream otherwise resolves to a scripted fake. `make test-real` sets it from your `.env` to reach real providers. It has no effect on a running server.
