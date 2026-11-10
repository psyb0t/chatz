# chatz

[![CI](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/chatz/badges/coverage.svg)](https://github.com/psyb0t/chatz/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/chatz/badges/version.svg)](https://github.com/psyb0t/chatz/releases)
[![license](https://raw.githubusercontent.com/psyb0t/chatz/badges/license.svg)](https://github.com/psyb0t/chatz/blob/main/LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/psyb0t/chatz?style=flat-square)](https://hub.docker.com/r/psyb0t/chatz)

Self-hosted AI chat with OpenAI-compatible and Anthropic models, MCP tools, and an embedded web app. Run one Docker image with SQLite, or use Postgres when needed.

## Features

- **Streaming conversations.** Messages, reasoning, and tool calls arrive as the model produces them. Stop or refresh without losing the submitted prompt.
- **Models you already use.** Connect OpenAI-compatible or Anthropic endpoints. chatz discovers their models and keeps provider-specific settings in the chat.
- **MCP tools.** Admins add stdio or HTTP MCP servers, then users enable their tools per chat. Stored MCP credentials are encrypted.
- **Useful chat controls.** Set temperature, reasoning, output size, and the history budget per chat. Create, rename, search, and delete chats.
- **Generative UI.** Models can answer with tables, status panels, charts, log viewers, and other live components rendered in the conversation.
- **Admin-managed access.** The first user is the admin. That admin creates the other accounts and manages MCP servers.
- **Demo mode.** Set `CHATZ_SHOWCASE_MODE=true` to return fixed streamed replies for exact demo prompts. Normal chats keep using the selected model.

## Quickstart

chatz ships as the `psyb0t/chatz` Docker image. The fastest path uses SQLite in a persistent Docker volume. It needs no LLM configuration for the first run.

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

Open **http://localhost:8080**, then visit **`/setup`** to create the admin. The chat works before you configure an LLM, but the model picker stays empty.

To connect a model, copy [`.env.example`](https://github.com/psyb0t/chatz/blob/main/.env.example) to `.env`, keep `CHATZ_DB_DRIVER=sqlite`, set `CHATZ_UPSTREAMS` and the key variable it names, and add `--env-file .env` to the command:

```bash
CHATZ_UPSTREAMS=[{"name":"openai","provider":"openai","baseUrl":"https://api.openai.com/v1","apiKeyEnv":"OPENAI_API_KEY"}]
OPENAI_API_KEY=sk-...
```

The `openai` provider also covers OpenAI-compatible APIs such as Ollama or [AIGate](https://github.com/psyb0t/aigate). Set `CHATZ_SECRETS_KEY` (`openssl rand -base64 32`) before adding MCP servers with credentials. The [getting started guide](https://psyb0t.github.io/chatz/getting-started/) covers Docker Compose with Postgres and demo mode, and [models and upstreams](https://psyb0t.github.io/chatz/models/) has the full upstream format.

Deploy a `v<semver>` release tag rather than `latest`. Pull the image again and recreate the container to upgrade; migrations run on start.

## Documentation

The full documentation lives at **[psyb0t.github.io/chatz](https://psyb0t.github.io/chatz/)**.

| Section | What it covers |
| --- | --- |
| [Getting started](https://psyb0t.github.io/chatz/getting-started/) | Docker and Docker Compose installs, the first admin, the first model, demo mode |
| [Configuration](https://psyb0t.github.io/chatz/configuration/) | Every environment variable with its default |
| [Models and upstreams](https://psyb0t.github.io/chatz/models/) | The `CHATZ_UPSTREAMS` format, model metadata, fallbacks, timeouts and health |
| [Chats](https://psyb0t.github.io/chatz/chats/) | Chat settings, the history budget, streaming status, retries and search |
| [MCP servers](https://psyb0t.github.io/chatz/mcp/) | Adding, importing and toggling MCP servers, and credential encryption |
| [Generative UI](https://psyb0t.github.io/chatz/generative-ui/) | The component catalog and how models put UI into a reply |
| [HTTP API](https://psyb0t.github.io/chatz/api/) | Authentication, endpoints, the SSE stream and error codes |
| [Deployment](https://psyb0t.github.io/chatz/deployment/) | Image tags, SQLite or Postgres, upgrades, backups and TLS |
| [Monitoring](https://psyb0t.github.io/chatz/monitoring/) | Health, admin readiness, Prometheus metrics and logs |
| [Security](https://psyb0t.github.io/chatz/security/) | What chatz protects and what is left to you |
| [Troubleshooting](https://psyb0t.github.io/chatz/troubleshooting/) | Startup refusals and common errors with their fixes |
| [Architecture](https://psyb0t.github.io/chatz/architecture/) | How the binary is put together |
| [Development](https://psyb0t.github.io/chatz/development/) | Building, testing and changing chatz from source |

## Development

Docker, Make and Bash are enough: every target runs its tools in a development container.

```bash
make help            # every target
make run             # Docker Compose stack from the checkout
make lint            # Go, shell and web checks
make test            # Go and web unit tests
make test-coverage   # integration-tagged Go coverage gate plus web tests
make docs            # build the documentation site
```

The [development guide](https://psyb0t.github.io/chatz/development/) covers the test tiers, code generation and CI. Rules for coding agents and contributors are in [`AGENTS.md`](https://github.com/psyb0t/chatz/blob/main/AGENTS.md).

## Sponsor

<a href="https://www.buymeacoffee.com/psyb0t"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-blue.png" alt="Buy Me a Coffee" height="60" width="217"></a>

## License

MIT, see [`LICENSE`](https://github.com/psyb0t/chatz/blob/main/LICENSE).
