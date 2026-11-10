---
title: "Deployment"
description: "Run chatz in production: image tags, choosing SQLite or Postgres, upgrades and migrations, the backup marker, TLS behind a reverse proxy and hardening."
nav_order: 9
---

# Deployment

## Image tags

The image is `psyb0t/chatz` on Docker Hub, built for `linux/amd64` and `linux/arm64`.

| Tag | What it is |
| --- | --- |
| `latest` | The newest build of `main`. Every push to `main` that passes CI replaces it. |
| `sha-<commit>` | The build of one `main` commit. |
| `v<semver>` | A release. The release pipeline points it at the `sha-<commit>` image that already passed CI on `main`, without rebuilding. |

Deploy a `v<semver>` tag, or pin a digest, so a restart never picks up a different build. The [CHANGELOG](https://github.com/psyb0t/chatz/blob/main/CHANGELOG.md) lists what each release changed.

The container must be started with the `run` command. The image's default command prints the help text and exits.

## SQLite or Postgres

| | SQLite | Postgres |
| --- | --- | --- |
| Set | `CHATZ_DB_DRIVER=sqlite` | `CHATZ_DB_DRIVER=postgres` (default) and the `CHATZ_DB_*` connection settings |
| Data | One file, `/data/chatz.sqlite` by default, in a Docker volume | Your Postgres server |
| Fits | One chatz container on one host | Several chatz containers, networked storage, an existing Postgres |

SQLite rules, all checked at startup:

- `CHATZ_DB_SQLITE_PATH` must be an absolute path to a file directly under `/data`, with no `?` or `#`.
- `/data` must be a real, writable directory, not a symlink. The image creates it owned by the app user, and a fresh named volume copies that ownership.
- An existing database file must be a regular file, not a symlink.
- Use a local volume. chatz opens one connection in WAL mode, which is not safe on networked filesystems or from two containers at once.

Switching drivers starts an empty database. chatz does not move data between them.

## Upgrades and migrations

Pull the new tag and recreate the container. On start chatz runs any pending migrations before it serves requests, for both drivers. To run them on their own, for example before switching traffic:

```bash
docker run --rm \
  --env-file .env \
  --volume chatz-data:/data \
  psyb0t/chatz:<tag> migrate
```

The volume is needed only for SQLite. `migrate` applies the migrations, logs the resulting version and exits. The admin readiness page shows the current migration version and whether the last migration left the database dirty.

Read the CHANGELOG before upgrading across several releases. To go back to an older release after its migrations ran, restore a backup taken before the upgrade.

## Backups

chatz does not back itself up. Back up the SQLite file (stop chatz or use SQLite's online backup) or dump Postgres with your usual tools.

After each successful backup, have the job write a marker file so the admin readiness page can show how fresh the last backup is. The file goes at `CHATZ_BACKUP_STATUS_PATH`, `/data/chatz-backup-status.json` by default, and holds exactly one JSON object:

```json
{
  "completedAt": "2026-01-31T03:00:00Z",
  "driver": "sqlite"
}
```

- `completedAt` is an RFC 3339 time, not in the future.
- `driver` must match `CHATZ_DB_DRIVER`.
- No other fields, at most 4 KiB, and a regular file, not a symlink.

| State shown | Meaning |
| --- | --- |
| `current` | The marker is valid and younger than `CHATZ_BACKUP_MAX_AGE` (`24h`). |
| `stale` | The marker is valid but older than that. |
| `not_recorded` | There is no marker file. |
| `invalid` | The file breaks one of the rules above. |
| `unavailable` | The file could not be read, or is a symlink or not a regular file. |

A bad or missing marker never breaks the page or the app.

## TLS and reverse proxies

chatz speaks plain HTTP. The session cookie is not marked `Secure`, so run chatz behind a reverse proxy that terminates TLS, and keep the container port bound to `127.0.0.1` or a private network as the examples do.

The proxy must pass Server-Sent Events through as they arrive. Turn off response buffering for `/api/v1/chats` (in nginx, `proxy_buffering off`), and allow long responses: a turn may run up to `CHATZ_UPSTREAM_TURN_TIMEOUT` (`5m`) per model request, with tool rounds on top. Serve chatz on its own host name: the web app and API expect to be at the root path.

## Hardening

The image runs as a non-root user and needs no write access outside `/data`, `/tmp` and the optional `/app/chatz.log` bind mount. The `docker run` command in [Getting started](getting-started.md#run-the-docker-image) and the repository's [`docker-compose.yml`](https://github.com/psyb0t/chatz/blob/main/docker-compose.yml) apply:

- a read-only root filesystem with a small `noexec` tmpfs at `/tmp`;
- all Linux capabilities dropped and `no-new-privileges`;
- memory, CPU and process limits (512 MB, one CPU, 256 processes);
- ports published on `127.0.0.1` only, and Postgres on an internal network.

Keep the metrics (`9091`) and pprof (`6060`) ports unpublished, or turn them off by setting their addresses to empty values. [Security](security.md) covers the rest.

## Health checks

`GET /healthz` returns 200 once the HTTP server is up, and is what the Compose health check calls. The image has BusyBox `wget` for that; it has no `curl`. Admins get a fuller picture from the readiness page; see [Monitoring](monitoring.md).
