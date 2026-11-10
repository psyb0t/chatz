---
title: "Security"
description: "What chatz protects and what it leaves to you: accounts and sessions, API keys, encrypted MCP credentials, stdio servers, logs and network exposure."
nav_order: 11
---

# Security

chatz is built for one person or a small trusted group behind their own front door. This page lists what it does and what you still have to do.

## Accounts and sessions

- The first account, created at `/setup`, is the admin. Only an admin creates accounts, manages MCP servers and sees upstream health and readiness. Setup cannot run a second time.
- Passwords are hashed with bcrypt.
- A session is a random 32-byte token in the `chatz_session` cookie (`HttpOnly`, `SameSite=Lax`, 30 days). The database stores only its SHA-256 hash. Deleting a user deletes their sessions.
- Login failures do not reveal whether the user name exists. There is no rate limit on login, so do not expose chatz to the internet without something in front that limits it.
- `CHATZ_AUTH_PASSWORDLESS=true` signs in anyone who reaches chatz as the only user. Use it only where the network already restricts who that is.
- Each chat is visible to its owner only.

## Network exposure

- chatz serves plain HTTP and the cookie is not marked `Secure`. Put a TLS reverse proxy in front of it; see [Deployment](deployment.md#tls-and-reverse-proxies).
- The examples publish the port on `127.0.0.1` only. Keep it that way, or bind to a private network, and let the proxy face the outside.
- There are no CORS headers, so browsers on other origins cannot read API responses.
- The metrics (`9091`) and pprof (`6060`) listeners have no authentication. Do not publish them; set their addresses to empty values to turn them off.

## API keys

- Model API keys stay in environment variables. `CHATZ_UPSTREAMS` holds only the variable name (`apiKeyEnv`), and each upstream sees only its own key.
- Keep `.env` out of version control. The repository ignores it.
- The upstream health and readiness responses carry no keys or URLs.

## MCP credentials

- MCP HTTP headers and stdio environment variables are encrypted at rest with AES-256-GCM under `CHATZ_SECRETS_KEY`. Without the key chatz refuses to store them rather than save plain text.
- The `Authorization` header comes back masked as `<scheme> [SECRET]` in admin responses. Stdio environment values are shown to the admin in the edit form.
- Losing or changing the key makes stored MCP credentials unreadable. Keep a copy of the key with your other secrets.

## Stdio MCP servers

A stdio MCP server is a command the admin configures, run inside the chatz container. It runs as chatz's user with chatz's network access and can read what chatz can read, including the environment, which holds the model API keys. Only add stdio servers you trust. HTTP MCP servers get only the headers you configure.

The per-chat MCP toggle hides a server's tools from one chat. It is a preference, not an access control: every user can turn on every enabled server.

## Model output

Generative UI renders only the components in the closed catalog, from declarative props. Models cannot inject HTML, scripts or event handlers through it. Markdown in replies is turned into components without raw HTML, so HTML in a reply is dropped rather than run, and links open in a new tab with `rel="noopener noreferrer nofollow"`. See [Generative UI](generative-ui.md).

## Logs

Log redaction masks secret-shaped keys and values. It does not hide chat text, and `LOG_LEVEL=debug` writes the full prompt of every model round. `chatz.log` is always at debug level. Keep both out of shared log systems. See [Monitoring](monitoring.md#logs).

## Container

The image runs as a non-root user, works with a read-only root filesystem and needs no Linux capabilities. The example `docker run` command and `docker-compose.yml` turn all of that on, with `no-new-privileges` and resource limits. Base images in the `Dockerfile` are pinned by digest.
