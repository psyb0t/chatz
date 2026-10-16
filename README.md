# chatz

[![CI](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/chatz/badges/coverage.svg)](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/chatz/badges/version.svg)](https://github.com/psyb0t/chatz/releases)
[![license](https://raw.githubusercontent.com/psyb0t/chatz/badges/license.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/psyb0t/chatz?style=flat-square)](https://hub.docker.com/r/psyb0t/chatz)

Self-hosted AI chat with OpenAI-compatible and Anthropic models, MCP tools, and
an embedded web app. Run one Docker image with SQLite, or use Postgres when needed.

## Features

- **Streaming conversations.** Messages, reasoning, and tool calls arrive as
  the model produces them. Stop or refresh without losing the submitted prompt.
- **Models you already use.** Connect OpenAI-compatible or Anthropic endpoints.
  Chatz discovers their models and keeps provider-specific settings in the chat.
- **MCP tools.** Admins add stdio or HTTP MCP servers, then users enable their
  tools per chat. Stored MCP settings are encrypted. Authorization headers stay
  masked in admin responses.
- **Useful chat controls.** Set temperature, reasoning, output size, and the
  history budget per chat. Create, rename, search, and delete chats.
- **Generative UI.** Model and tool output can render tables, status panels,
  charts, log viewers, and other live components in the conversation. See the
  [rendering guide](web/src/lib/render/README.md) for the component catalog.
- **Admin-managed access.** The first user is the admin. That admin creates the
  other accounts and configures upstreams and MCP servers.
- **Demo mode.** Set `CHATZ_SHOWCASE_MODE=true` to return fixed streamed replies
  for exact demo prompts. Normal chats keep using the selected model.

## Quickstart

Chatz ships as the `psyb0t/chatz` Docker image. The fastest path uses SQLite in
a persistent Docker volume. It needs no LLM configuration for the first run.

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

Open **http://localhost:8080**, then visit **`/setup`** to create the admin.
The chat works before you configure an LLM, but the model picker stays empty.

`latest` is for a first look. Pin an immutable release tag such as
`psyb0t/chatz:v0.7.10` for deployment. Copy `.env.example` to `.env`, set
`CHATZ_DB_DRIVER=sqlite`, and add `--env-file .env` to configure upstreams,
MCP secret storage, or other settings. Chatz exposes `GET /healthz`.

### Try fixed demo replies

Start the normal stack with fixed-response mode:

```bash
CHATZ_SHOWCASE_MODE=true docker compose up --build
```

After completing `/setup`, select a configured model and send, for example:

- `Show me what's happening across the production platform right now.`
- `Where are we losing deals in the sales pipeline?`
- `Which customers are at risk and who should the team contact first?`

Each returns a paced reply with thinking, tool calls, and a dashboard. Any
other message uses the selected model and normal MCP-enabled chat path.

### Connect a model

Copy `.env.example` to `.env` and set `CHATZ_UPSTREAMS`. Each upstream chooses
the `openai` driver, which also works with OpenAI-compatible APIs such as
[AIGate](https://github.com/psyb0t/aigate), or the `anthropic` driver.
`apiKeyEnv` names a variable in `.env`; never put the key in the JSON.

```json
[
  {"name":"openai","provider":"openai","baseUrl":"https://api.openai.com/v1","apiKeyEnv":"OPENAI_API_KEY"},
  {"name":"anthropic","provider":"anthropic","apiKeyEnv":"ANTHROPIC_API_KEY"},
  {"name":"aigate","provider":"openai","baseUrl":"http://aigate:4000","apiKeyEnv":"AIGATE_TOKEN"},
  {"name":"ollama","provider":"openai","baseUrl":"http://localhost:11434/v1"}
]
```

Docker Compose loads `.env` automatically. Add `--env-file .env` to the Docker
command above. The AIGate example assumes both containers share a Docker
network. Use AIGate's reachable root URL otherwise. The [example environment
file](.env.example) documents model metadata, timeouts, and every other
setting.

Generate a 32-byte base64 key before adding MCP credentials, then set it as
`CHATZ_SECRETS_KEY`:

```bash
openssl rand -base64 32
```

Without this key, Chatz refuses to store MCP HTTP headers or stdio environment
variables.

## Configuration

All configuration comes from environment variables. Start with [`.env.example`](.env.example). The usual settings are:

- `CHATZ_DB_DRIVER=sqlite` for one local container, or `postgres` for Compose
  and shared deployments.
- `CHATZ_UPSTREAMS` and the key variables named by `apiKeyEnv` for models.
- `CHATZ_SECRETS_KEY` before storing MCP credentials. Set
  `CHATZ_AUTH_PASSWORDLESS=true` for a single-user installation.
- `LOG_LEVEL=debug` for local prompt troubleshooting. Debug logs may contain
  ordinary chat text, so do not send them to a shared log service.

## Architecture

The Go service serves the API and embedded web app, stores chat data in Postgres
or SQLite, calls models through Elelem, and connects MCP servers on demand. See
[the architecture guide](docs/architecture.md) for the details.

## HTTP API

The versioned JSON and SSE API is under `/api/v1`. Read
[`api/api.yml`](api/api.yml) for the contract. `/healthz` is available for
container health checks.

## Deploy

Every push to `main` publishes `psyb0t/chatz:latest`. A `v<semver>` release tag
publishes the matching immutable `psyb0t/chatz:v<semver>` image. Deploy a tag,
not `latest`. Use SQLite for one local volume and Postgres for replicas,
networked storage, or an existing Postgres installation. For Docker Compose and
development, read [getting started](docs/getting-started.md) and
[development](docs/development.md).

## License

See [`LICENSE`](LICENSE).

---
