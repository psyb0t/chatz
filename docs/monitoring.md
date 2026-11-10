---
title: "Monitoring"
description: "Watch a running chatz: the /healthz probe, the admin readiness snapshot, Prometheus LLM metrics, the pprof listener, stdout logs and chatz.log."
nav_order: 10
---

# Monitoring

## Health

`GET /healthz` on the public port answers `{"status":"ok"}` with 200 once the HTTP server runs. It needs no login and checks nothing beyond that, so use it for liveness and container health checks.

## Admin readiness

Admins get an operational snapshot on the readiness page of the admin area in the web app, or from `GET /api/v1/admin/readiness`:

| Field | Meaning |
| --- | --- |
| `appVersion`, `commit` | The build. `commit` is left out when the build has none. |
| `databaseDriver` | `postgres` or `sqlite`. |
| `migrationVersion`, `migrationDirty` | The applied migration and whether the last one failed halfway. |
| `backup` | The backup marker state: `current`, `stale`, `not_recorded`, `invalid` or `unavailable`, with the completion time. See [Deployment](deployment.md#backups). |
| `upstreams` | Health of every model upstream. See [Models and upstreams](models.md#upstream-health). |

The snapshot holds no secrets. Only a failure to read the migration status fails the request; a bad backup marker shows up as a state.

## Prometheus metrics

`/metrics` is served on a separate listener, `CHATZ_METRICS_LISTENADDRESS` (`:9091`), which neither example publishes. Scrape it from inside the Docker network, or publish it to a private address.

| Metric | Type | Labels |
| --- | --- | --- |
| `llm_requests_total` | counter | `service`, `stage`, `model`, `outcome` |
| `llm_request_duration_seconds` | histogram | `service`, `stage`, `model`, `outcome` |
| `llm_tokens_total` | counter | `service`, `stage`, `model`, `kind` |

- `outcome` is `ok` or `error`. `kind` is `input`, `output` or `cache_read`.
- `service` is `chatz` and `stage` is `chat`. `model` falls back to `unknown`.
- The duration buckets are 0.25, 0.5, 1, 2, 5, 10, 20, 30, 60 and 120 seconds.
- The Go runtime and process collectors are registered too.

Every model call also writes a row to the `llm_usage` table with its token counts and duration.

## pprof

Go's profiler is served under `/debug/pprof/` on `CHATZ_PROFILING_LISTENADDRESS` (`:6060`). It exposes command lines and memory, so never publish it. Set the variable to an empty value to turn the listener off, and do the same for metrics if you do not scrape them. A listener that fails to bind logs a warning and leaves the API running.

## Logs

chatz logs to standard output in the format and level set by `LOG_FORMAT` and `LOG_LEVEL`. Every line carries `service`, `version` and, when known, `commit`. Lines logged while handling a request carry its `request_id`, the same value as the `X-Request-ID` response header.

The app also writes `chatz.log` in its working directory, `/app/chatz.log` in the image:

- It is cleared on every start, so it holds the current run only.
- It is always JSON at debug level, whatever `LOG_LEVEL` says.
- Bind-mount a host file there to read it from the host, as the examples do. Without a writable file chatz logs a warning and runs without it.

Both outputs pass through redaction: attributes named like `password`, `token`, `secret`, `api_key`, `authorization` or `cookie` become `[REDACTED]`, and secret-shaped `key=value` pairs and bearer tokens in text are masked. Ordinary chat text is not redacted, and debug logs include the full prompt of each model round. Keep debug logs local.

Long waits log a heartbeat, so a model stream or tool call that is still running is visible in the log instead of looking like a hang.
