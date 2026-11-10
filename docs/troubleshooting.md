---
title: "Troubleshooting"
description: "Fix chatz startup refusals and common problems: database and SQLite path errors, upstream JSON mistakes, secrets keys, empty model lists and MCP failures."
nav_order: 12
---

# Troubleshooting

chatz checks its configuration before it serves anything, and a bad value stops the process with an error in the log. The messages below are quoted from the code, so you can search for them. Read the container log with `docker logs chatz` or `docker compose logs chatz`.

## The container exits right away

| Symptom | Cause and fix |
| --- | --- |
| The help text prints and the container exits | The container was started without the `run` command. Add `run` after the image name. |
| `invalid log level`, `invalid log format` or `invalid log add-source value` | `LOG_LEVEL` must be `debug`, `info`, `warn` or `error`; `LOG_FORMAT` must be `json` or `text`; `LOG_ADD_SOURCE` must be a boolean. |
| `parse config` | A variable has the wrong type, such as a word in `CHATZ_DB_PORT` or a duration without a unit. Durations need one: `45s`, not `45`. |

## Database

| Message | Fix |
| --- | --- |
| `CHATZ_DB_DRIVER must be postgres or sqlite, got "..."` | Use one of the two values. |
| `connect postgres` | chatz cannot reach Postgres. Check `CHATZ_DB_HOSTNAME`, the port, the credentials and that the server is up. Under Compose the host name is `postgres`. |
| `migrate postgres up` | A migration failed and chatz stopped. The log line carries the database error. Fix the cause, or restore the backup taken before the upgrade, then start again. |
| `CHATZ_DB_SQLITE_PATH must name a regular file directly under "/data", got "..."` | Use an absolute path whose directory is exactly `/data`, with no `?` or `#`. |
| `CHATZ_DB_SQLITE_PATH must be a regular file, got "..."` | Something other than a plain file, or a symlink, sits at that path. |
| `sqlite data directory must be a real directory, got "..."` | `/data` is missing, is a file or is a symlink. Mount a volume at `/data`. |
| `sqlite data directory is not writable: "..."` | The volume is not writable by the app user (UID 1000). A fresh named volume gets the right owner; a bind-mounted host directory needs `chown 1000:1000`. |
| `CHATZ_DB_SQLITE_BUSY_TIMEOUT must be positive, got ...` | Set a positive duration such as `5s`. |

## Upstream configuration

Every message here is wrapped in `invalid upstream` and stops startup.

| Message | Fix |
| --- | --- |
| `parse CHATZ_UPSTREAMS json` | The value is not valid JSON. In `.env` keep the array on one line. |
| `CHATZ_UPSTREAMS entry 0 has no name` | Give every entry a `name`. |
| `CHATZ_UPSTREAMS contains duplicate name "..."` | Names must be unique. |
| `CHATZ_UPSTREAMS entry "..." has no provider` | Add `"provider": "openai"` or `"anthropic"`. |
| `CHATZ_UPSTREAMS entry "..." has unsupported provider "..."` | Only `openai` and `anthropic` exist. Use `openai` for every OpenAI-compatible server. |
| `upstream "..." references missing required environment variable "..."` | The variable named in `apiKeyEnv` is unset or empty in the container. With `docker run`, pass it with `--env-file`. Leave `apiKeyEnv` out for keyless servers. |
| `upstream "..." has duplicate model metadata for "..."` | Two `models` entries share an `id`. |
| `upstream "..." model entry 0 has no id` | Every `models` entry needs an `id`. |
| `... limits and expected latency must not be negative` | Fix `contextWindow`, `maxOutputTokens` or `expectedFirstTokenLatencyMs`. |
| `... token price must not be negative` / `... token price currency must be a three-letter uppercase code` | Prices take a non-negative integer and a code such as `USD`. |
| `... fallback entry 0 is empty`, `... cannot fall back to itself`, `... has duplicate fallback "..."` | Fix the `fallbackModels` list. |

These come from model discovery, after the JSON is accepted:

| Message | Fix |
| --- | --- |
| `configured default model "..." was not advertised by an upstream` | `CHATZ_DEFAULT_MODEL` is not in any discovered list. Check the ID, or the log for a discovery failure of its upstream. |
| `configured fallback model "..." for "..." was not advertised by an upstream` | A `fallbackModels` entry is not discovered. |
| `invalid upstream runtime limits` | A `CHATZ_UPSTREAM_*` timeout or the concurrency is zero or negative, or the first-token timeout is longer than the turn timeout. |

## Secrets and backups

| Message | Fix |
| --- | --- |
| `decode base64 key` | `CHATZ_SECRETS_KEY` is not standard base64. Generate one with `openssl rand -base64 32`. |
| `secrets: key must be 32 bytes` | The key decodes to the wrong length. It must be 32 bytes before encoding. |
| `CHATZ_BACKUP_STATUS_PATH must name a file directly under "/data", got "..."` | Put the marker directly in `/data`. |
| `CHATZ_BACKUP_MAX_AGE must be positive, got ...` | Use a positive duration. |

## It starts, but something is missing

| Symptom | Cause and fix |
| --- | --- |
| The model picker is empty | `CHATZ_UPSTREAMS` is empty, or every upstream failed discovery. Look for `upstream models discovery failed` in the log and check the base URL, the key and the network. Models are discovered only at startup, so restart after fixing. |
| One provider's models are missing | That upstream failed discovery, or another upstream earlier in the array lists the same IDs and won (`model_conflict` in the log). |
| The log says `secrets key not configured; MCP secret sealing disabled` | `CHATZ_SECRETS_KEY` is empty. MCP servers without headers or environment variables still work. |
| Saving an MCP server fails with `secrets: no encryption key configured` | Set `CHATZ_SECRETS_KEY` and restart. |
| An MCP server stays `failed` | Read `statusReason` and `statusError` on the admin MCP page. A stdio command must exist inside the container; an HTTP URL must be reachable from it. chatz retries every 15 seconds, or press reconnect. |
| MCP credentials stopped working after a restart | `CHATZ_SECRETS_KEY` changed. Restore the old key, or enter the credentials again. |
| The page shows a placeholder instead of the app | The binary was built without the web app (`spa_not_built` in the log). Use the published image, or run `make web-embed` before a local `go build`. |
| `chatz.log file sink not started` | `/app/chatz.log` is not writable, which is normal without the bind mount. Create the host file and `chown 1000:1000` it if you want the file. |
| Replies arrive all at once behind a proxy | The proxy buffers responses. Turn buffering off for `/api/v1/chats`. |
| Every chat ends with `The model did not respond in time. Try again.` | The model took longer than `CHATZ_UPSTREAM_FIRST_TOKEN_TIMEOUT` (`45s`) to start or `CHATZ_UPSTREAM_TURN_TIMEOUT` (`5m`) to finish. Raise them for slow local models. |
| `This conversation is too large for the selected model.` | Lower `maxHistoryTokens` in the chat settings below the model's context window. |
| Setup answers `setup already completed` | An account exists. Log in, or delete the database to start over. |
