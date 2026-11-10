---
title: "HTTP API"
description: "The chatz /api/v1 JSON and SSE API: session cookie auth, every endpoint, the chat event stream, pagination, error codes and the Go client."
nav_order: 8
---

# HTTP API

The web app uses the same API that is open to scripts: JSON over HTTP under `/api/v1`, with chat replies streamed as Server-Sent Events. The contract is the OpenAPI file [`api/api.yml`](https://github.com/psyb0t/chatz/blob/main/api/api.yml). Its `servers` entry says `/v1`; chatz mounts it at `/api/v1`.

`GET /healthz` sits at the root, outside `/api/v1`, and answers `{"status":"ok"}` without authentication. Any other path outside `/api/` serves the web app. An unknown path under `/api/` is a JSON 404.

## Authentication

The API uses a session cookie named `chatz_session`. Log in, keep the cookie, and send it with every request:

```bash
curl -s -c cookies.txt \
  -H 'Content-Type: application/json' \
  -d '{"username": "admin", "password": "..."}' \
  http://localhost:8080/api/v1/auth/login

curl -s -b cookies.txt http://localhost:8080/api/v1/models
```

- The cookie is `HttpOnly`, `SameSite=Lax`, path `/`, and lasts 30 days. It is not marked `Secure`; serve chatz behind TLS.
- The server keeps only a SHA-256 hash of the token. Logging out deletes the session.
- `POST /api/v1/auth/setup` creates the first user as an admin and logs them in. It works once; after that it answers 409.
- `GET /api/v1/auth/status` reports `needsSetup`, `authenticated` and the current user. With `CHATZ_AUTH_PASSWORDLESS=true` and exactly one user, it also signs the caller in.
- A wrong user name and a wrong password give the same 401 `invalid credentials`.
- There is no login rate limit and no CSRF token; the `SameSite=Lax` cookie is the cross-site protection.

Requests without a valid session get 401. Admin endpoints answer 403 to other users.

## Endpoints

| Method and path | Who | What it does |
| --- | --- | --- |
| `GET /healthz` | anyone | Liveness check. |
| `GET /api/v1/auth/status` | anyone | Setup and login state. |
| `POST /api/v1/auth/setup` | anyone, once | Create the first admin. |
| `POST /api/v1/auth/login` | anyone | Log in and set the cookie. |
| `POST /api/v1/auth/logout` | anyone | Delete the current session, if there is one. |
| `GET /api/v1/users` | admin | List users. |
| `POST /api/v1/users` | admin | Create a user (`username`, `password`, `isAdmin`). |
| `DELETE /api/v1/users/{userId}` | admin | Delete a user with their sessions and chats. Admins cannot delete themselves. |
| `GET /api/v1/models` | user | The merged model list, with aliases, limits, capabilities, prices and the default flag. |
| `GET /api/v1/upstreams` | admin | Upstream health snapshots. See [Models and upstreams](models.md#upstream-health). |
| `GET /api/v1/admin/readiness` | admin | Version, database, migrations, backup marker and upstream health. See [Monitoring](monitoring.md#admin-readiness). |
| `GET /api/v1/chats` | user | Your chats, paginated, with an optional `search` on titles. |
| `POST /api/v1/chats` | user | Create a chat from `model` and `message` and stream the first reply. |
| `POST /api/v1/chats/empty` | user | Return your one empty chat, creating it if needed. Repeated calls give the same chat until it gets a message. |
| `GET /api/v1/chats/{chatId}` | owner | Chat metadata: title, model and settings. |
| `POST /api/v1/chats/{chatId}` | owner | Send `message` (and optionally switch `model`) and stream the reply. |
| `PATCH /api/v1/chats/{chatId}` | owner | Rename the chat (`title`). |
| `DELETE /api/v1/chats/{chatId}` | owner | Delete the chat. |
| `GET /api/v1/chats/{chatId}/messages` | owner | Messages, oldest first, paginated. |
| `PATCH /api/v1/chats/{chatId}/settings` | owner | Replace the chat's settings. See [Chats](chats.md#settings). |
| `POST /api/v1/chats/{chatId}/context-preview` | owner | Token counts and omitted turns for an unsent draft. |
| `GET /api/v1/chats/{chatId}/mcp-servers` | owner | MCP servers with their status and whether this chat uses them. |
| `PATCH /api/v1/chats/{chatId}/mcp-servers/{serverId}` | owner | Turn one MCP server on or off for this chat. |
| `GET /api/v1/mcp/servers` | admin | List MCP servers. |
| `POST /api/v1/mcp/servers` | admin | Add an MCP server. |
| `PATCH /api/v1/mcp/servers/{serverId}` | admin | Edit a server and reconnect it. |
| `DELETE /api/v1/mcp/servers/{serverId}` | admin | Remove a server. |
| `POST /api/v1/mcp/servers/{serverId}/reconnect` | admin | Start a new connection attempt. |
| `GET /api/v1/mcp/servers/{serverId}/tools` | admin | Tools of a connected server. |
| `POST /api/v1/mcp/import` | admin | Import servers from `.mcp.json` text in `content`. |

A chat that is not yours answers 404, the same as one that does not exist.

## Pagination

List endpoints take `limit` and `offset` and return `{items, limit, offset, total}`.

| Endpoint | Default `limit` | Maximum |
| --- | --- | --- |
| `GET /api/v1/chats` | 100 | 100 |
| `GET /api/v1/chats/{chatId}/messages` | 50 | 200 |

A value out of range is refused with 400 `VALIDATION_FAILED`, never adjusted. `search` on the chat list allows up to 256 characters.

## The chat stream

`POST /api/v1/chats` and `POST /api/v1/chats/{chatId}` answer with `text/event-stream`. The frames follow Anthropic's Messages streaming format, and every frame's JSON repeats its event name in `type`:

```text
event: message_start
data: {"type":"message_start","message":{"id":"msg_...","stream_id":"<chat id>","model":"..."}}
```

| Event | Carries |
| --- | --- |
| `message_start` | The message ID, the chat ID in `stream_id` (how a new chat learns its ID) and the model. |
| `content_block_start` | A new block: `text`, `thinking`, `tool_use` (with tool ID and name) or `tool_result` (with the tool use ID and `is_error`). |
| `content_block_delta` | `text_delta`, `thinking_delta`, `input_json_delta` for tool arguments, or `json_partial` for a tool result. |
| `content_block_stop` | The end of the block at `index`. |
| `message_delta` | The stop reason. |
| `message_stop` | The end of the message. |
| `ping` | Keepalive. |
| `chat_status` | Progress: `connecting`, `waiting_first_token`, `streaming`, `running_tool` or `retrying`. Display only. |
| `error` | A failure after streaming began: `{"type":"error","error":{"type":"<kind>","message":"..."}}`. |

The error kinds and their messages are listed on [Chats](chats.md#failures-and-retries). A client that disconnects gets no error frame. Problems found before the stream opens, such as a missing message, an unknown model or a chat that is not found, come back as ordinary JSON errors with 400 or 404. Messages from a turn that never completed come back from the messages endpoint with `incomplete: true`.

## Errors

Every error body has the same shape:

```json
{
  "code": "VALIDATION_FAILED",
  "message": "temperature must be between 0 and 2",
  "details": {}
}
```

`code` is the stable value to switch on, `message` is for people, and `details` is optional.

| Status | Codes |
| --- | --- |
| 400 | `BAD_REQUEST`, `VALIDATION_FAILED` |
| 401 | `UNAUTHORIZED` |
| 403 | `FORBIDDEN` |
| 404 | `NOT_FOUND` |
| 409 | `CONFLICT` (setup already done, user name taken) |
| 413 | A request body over 1 MB. The `code` in this body is currently `INTERNAL_SERVER_ERROR`, so check the status instead. |
| 500 | `INTERNAL_SERVER_ERROR`, with the message `internal server error` and no internal detail |

## Request IDs

Every response carries an `X-Request-ID` header. chatz keeps an incoming value when it is a UUID or ULID of at most 64 characters and makes a new one otherwise. The same ID appears in the server's log lines for that request.

## Go client

[`pkg/http/api/client`](https://github.com/psyb0t/chatz/tree/main/pkg/http/api/client) is a Go client generated from the same contract with oapi-codegen. Point it at `http://<host>/api/v1` and give it an HTTP client with a cookie jar.
