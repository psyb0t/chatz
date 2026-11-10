---
title: "MCP servers"
description: "Connect stdio and HTTP MCP servers to chatz, import a .mcp.json file, turn servers on per chat, and keep their headers and environment encrypted."
nav_order: 6
---

# MCP servers

chatz can call tools on [Model Context Protocol](https://modelcontextprotocol.io) servers. The admin adds the servers, chatz keeps them connected, and every chat offers their tools to the model unless the user turns a server off for that chat. The developer view is the [MCP package README](https://github.com/psyb0t/chatz/blob/main/internal/pkg/mcp/README.md).

## Before you add a server

Set `CHATZ_SECRETS_KEY` first if any server needs an HTTP header (such as `Authorization`) or stdio environment variables:

```bash
openssl rand -base64 32
```

chatz encrypts every stored header and environment map with AES-256-GCM under that key. Without it, saving a server that has headers or environment variables fails; chatz never stores them in plain text. Keep the key stable, because a new key cannot read what the old one sealed.

## Adding a server

Admins manage servers on the MCP page of the admin area in the web app, or through the API (`POST /api/v1/mcp/servers`). A server has a name and one of two transports:

| Transport | Fields | What chatz does |
| --- | --- | --- |
| `stdio` | `command`, `args`, `env` | Starts the command inside the chatz container and talks MCP over its standard input and output. The process lives as long as the session. |
| `http` | `url`, `headers` | Connects to a streamable HTTP MCP endpoint and sends the headers with every request. |

Names must be unique. Do not put `__` in a name: tool names are built as `<server>__<tool>` and split on the first `__`.

A stdio command runs inside the chatz container as the same non-root user, with the same read-only filesystem and network. The stock image is Alpine with the chatz binary and CA certificates, and no Node.js or Python, so a stdio server needs a command that exists in the image, or a custom image built on top of `psyb0t/chatz`. HTTP servers have no such limit.

Saving a server stores it and starts connecting in the background, so a slow or unreachable endpoint never blocks the request. Editing a server (`PATCH /api/v1/mcp/servers/{serverId}`) reconnects it. An edit can also set `enabled` to `false`, which disconnects the server for every chat.

## Importing a .mcp.json file

`POST /api/v1/mcp/import` (or the import box on the admin MCP page) takes the text of a Claude-style `.mcp.json` file:

```json
{
  "mcpServers": {
    "files": {
      "command": "mcp-files",
      "args": ["--root", "/data/files"],
      "env": {
        "FILES_TOKEN": "..."
      }
    },
    "search": {
      "url": "https://search.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ..."
      }
    }
  }
}
```

An entry with `command` becomes a stdio server and an entry with `url` becomes an HTTP server. If both are present, `command` wins. An entry with neither, or a file with no servers, is refused. The `type` field is ignored.

## Connection status

Each server shows a live status:

| Status | Meaning |
| --- | --- |
| `connecting` | An attempt is running. |
| `connected` | The session is up. The tool count is shown. |
| `failed` | The last attempt failed. `statusReason` is `unreachable`, `access_denied`, `not_responding` or `failed`, and `statusError` has the detail. |
| `disabled` | The admin turned the server off. |

Each attempt has a 10-second deadline, and a hung attempt is marked `failed` with `not_responding` five seconds after that. A failed connection, or a session that dies, is retried every 15 seconds. The admin view also shows the time of the last attempt, success and failure, the connection latency and the last error, so a current failure can be told apart from a reconnect in progress. `POST /api/v1/mcp/servers/{serverId}/reconnect` starts a new attempt at once, and `GET /api/v1/mcp/servers/{serverId}/tools` lists the tools of a connected server.

Enabled servers reconnect in the background when chatz starts. A server that cannot connect does not stop startup.

## Tools in chats

Every connected, enabled server offers its tools to every chat. The model sees them as `<server>__<tool>`, and chatz routes each call back to the server that owns it.

In the composer, a user can turn a server off for one chat. Its tools are then hidden from the model in that chat. This is a preference, not an access control: anyone who can chat can use every enabled server. The API for it is `GET /api/v1/chats/{chatId}/mcp-servers` and `PATCH /api/v1/chats/{chatId}/mcp-servers/{serverId}` with `{"enabled": false}`.

A tool that reports an error returns a visible error result to the model, which can try something else. A transport failure is an error for that call. chatz sets no extra timeout or size cap on tool calls; a call ends with the turn.

## Credentials in the admin view

- Every header is encrypted at rest.
- The `Authorization` header comes back masked as `<scheme> [SECRET]`, for example `Bearer [SECRET]`. Sending the mask back unchanged keeps the stored value, leaving the header out removes it, and any other value replaces it.
- Other headers come back as configured.
- Stdio environment values are decrypted for the admin's edit form.

Keep keys and tokens in `.env` or in chatz's encrypted store, never in files you commit.
