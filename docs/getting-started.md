---
title: "Getting started"
description: "Run chatz from the psyb0t/chatz Docker image or with Docker Compose, create the admin account, connect a first model and try demo mode."
nav_order: 2
---

# Getting started

## Run the Docker image

chatz ships as the `psyb0t/chatz` Docker image. This command runs it with SQLite in a persistent Docker volume, a read-only root filesystem, no Linux capabilities and resource limits. It needs no model configuration for the first run.

```bash
docker pull psyb0t/chatz:latest
docker volume create chatz-data
touch chatz.log
sudo chown 1000:1000 chatz.log

docker run --name chatz --detach --restart unless-stopped --init \
  --publish 127.0.0.1:8080:8080 \
  --env CHATZ_DB_DRIVER=sqlite \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --memory 512m \
  --cpus 1.0 \
  --pids-limit 256 \
  --mount type=bind,src="$(pwd)/chatz.log",dst=/app/chatz.log \
  --volume chatz-data:/data \
  psyb0t/chatz:latest run
```

- `run` is required. Without it the image prints the command help and exits.
- The SQLite database lives at `/data/chatz.sqlite` in the `chatz-data` volume.
- `chatz.log` is a diagnostic log the app clears and rewrites on every start. The bind mount makes it readable from the host. Leave the mount out if you do not want it; chatz starts without it and logs a warning.
- The port is published on `127.0.0.1` only. Put a reverse proxy with TLS in front of it before exposing it, as [Deployment](deployment.md#tls-and-reverse-proxies) explains.

Open http://localhost:8080 and go to `/setup` to create the admin account. The first account is always the admin, and setup works only once. The chat works before you configure a model, but the model picker stays empty.

`GET /healthz` answers `{"status":"ok"}` once the server is up.

To upgrade, pull the image again and recreate the container. [Deployment](deployment.md) covers release tags and what an upgrade does to the database.

## Run with Docker Compose

The repository's [`docker-compose.yml`](https://github.com/psyb0t/chatz/blob/main/docker-compose.yml) builds the image from a checkout and starts Postgres next to it, with the same hardening as the command above.

```bash
git clone https://github.com/psyb0t/chatz.git
cd chatz
cp .env.example .env
touch chatz.log
docker compose up --build
```

chatz answers on http://localhost:8080. Postgres data lives in the `pgdata` volume, and the database sits on an internal Docker network. Stop the stack with `docker compose down`.

For a single chatz container on SQLite instead, set `CHATZ_DB_DRIVER=sqlite` in `.env` and start only the app service. Its data goes to the `chatzdata` volume.

```bash
docker compose up --build --no-deps chatz
```

`make run` picks the right command for you: it creates `chatz.log`, reads `CHATZ_DB_DRIVER` from the environment or `.env`, and starts Postgres only when the driver is `postgres`. Switching drivers does not move data. SQLite does not import an existing Postgres database.

## Connect a first model

Models come from upstreams listed in `CHATZ_UPSTREAMS`, a JSON array. Each upstream names a provider (`openai` for OpenAI and every OpenAI-compatible API, or `anthropic`), an optional base URL, and the name of the environment variable that holds its API key. The key itself never goes into the JSON.

```bash
CHATZ_UPSTREAMS=[{"name":"openai","provider":"openai","baseUrl":"https://api.openai.com/v1","apiKeyEnv":"OPENAI_API_KEY"}]
OPENAI_API_KEY=sk-...
```

Put both lines in `.env`. Docker Compose loads it on its own; for the `docker run` command above, copy [`.env.example`](https://github.com/psyb0t/chatz/blob/main/.env.example) to `.env`, keep `CHATZ_DB_DRIVER=sqlite`, and add `--env-file .env`. Restart chatz after changing upstreams, because models are discovered at startup.

[Models and upstreams](models.md) has the full format, more examples (Anthropic, Ollama, a gateway), model aliases, prices and fallbacks.

## Store MCP credentials

MCP servers that need an HTTP header or environment variables, such as an API token, need an encryption key first. Generate one and set it as `CHATZ_SECRETS_KEY`:

```bash
openssl rand -base64 32
```

Without the key, chatz refuses to store MCP HTTP headers or stdio environment variables. [MCP servers](mcp.md) explains the rest.

## Try demo mode

Demo mode answers a fixed set of exact prompts with paced, prerecorded replies that show thinking, tool calls and a generative UI dashboard. It does not call a model for those prompts.

```bash
CHATZ_SHOWCASE_MODE=true docker compose up --build
```

After `/setup`, send one of these messages exactly as written:

- `Show me what's happening across the production platform right now.`
- `Where are we losing deals in the sales pipeline?`
- `Which customers are at risk and who should the team contact first?`
- `How is the platform infrastructure holding up under load right now?`
- `How did the new onboarding experiment perform against control?`
- `Where is our cloud spend going this quarter and are we on budget?`
- `Walk me through the active incident and its blast radius.`

Any other message goes to the selected model as usual, with its MCP tools. [Chats](chats.md#demo-mode) has the details.
