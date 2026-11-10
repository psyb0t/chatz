---
title: "Home"
description: "chatz is a self-hosted AI chat app in one Docker image: streaming OpenAI-compatible and Anthropic models, MCP tools and generative UI."
nav_order: 1
permalink: /
seo:
  type: WebSite
---

# chatz

Self-hosted AI chat in one Docker image. The `psyb0t/chatz` image runs one Go binary that serves the chat web app and its API from the same port, streams replies from OpenAI-compatible or Anthropic models, calls tools on MCP servers that the admin connects, and stores everything in SQLite or Postgres.

## What you get

| Part | What it does |
| --- | --- |
| Streaming chat | Text, reasoning and tool calls arrive as the model produces them. A stopped or refreshed turn keeps the submitted prompt. |
| Models | Any OpenAI-compatible endpoint (OpenAI, Ollama, vLLM, gateways) or Anthropic. chatz asks each endpoint for its models and merges the lists. |
| MCP tools | The admin adds stdio or HTTP MCP servers. Users turn each server on or off per chat. Stored credentials are encrypted. |
| Generative UI | Models can answer with tables, charts, status panels, log viewers and other components that render inline in the conversation. |
| Accounts | The first user becomes the admin and creates the other accounts. A single-user install can skip the login. |
| Storage | SQLite in one Docker volume, or Postgres for shared and replicated setups. Migrations run on every start. |
| Demo mode | Fixed, paced replies for a set of exact demo prompts, for showing the UI without spending tokens. |

## Quick start

SQLite in a Docker volume is the shortest path, and it needs no model for the first run:

```bash
docker volume create chatz-data

docker run --name chatz --detach --restart unless-stopped \
  --publish 127.0.0.1:8080:8080 \
  --env CHATZ_DB_DRIVER=sqlite \
  --volume chatz-data:/data \
  psyb0t/chatz:latest run
```

Open http://localhost:8080/setup and create the admin account. The chat works right away, but the model picker stays empty until you connect a model. [Getting started](getting-started.md) has the hardened version of this command, the Docker Compose setup and the first-model steps.

## Where to go next

| Page | What it covers |
| --- | --- |
| [Getting started](getting-started.md) | Docker and Docker Compose installs, the first admin, the first model, demo mode |
| [Configuration](configuration.md) | Every environment variable with its default |
| [Models and upstreams](models.md) | The `CHATZ_UPSTREAMS` format, model metadata, fallbacks, timeouts and health |
| [Chats](chats.md) | Chat settings, the history budget, streaming status, retries and search |
| [MCP servers](mcp.md) | Adding, importing and toggling MCP servers, and how their credentials are kept |
| [Generative UI](generative-ui.md) | The component catalog and how models put UI into a reply |
| [HTTP API](api.md) | Authentication, endpoints, the SSE stream and error codes |
| [Deployment](deployment.md) | Image tags, SQLite or Postgres, upgrades, backups and TLS |
| [Monitoring](monitoring.md) | Health, admin readiness, Prometheus metrics, pprof and logs |
| [Security](security.md) | What chatz protects, and what is left to you |
| [Troubleshooting](troubleshooting.md) | Startup refusals and common errors with their fixes |
| [Architecture](architecture.md) | How the binary is put together |
| [Development](development.md) | Building, testing and changing chatz from source |
