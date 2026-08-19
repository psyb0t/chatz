# MCP integration

`internal/pkg/mcp` owns admin-managed Model Context Protocol connections:
stdio and streamable HTTP transports, live status, tool routing, credentials,
imports, and automatic recovery. Per-chat selection lives in
`internal/pkg/core/chats`.

## Contents

- [Lifecycle and recovery](#lifecycle-and-recovery)
- [Transports and tools](#transports-and-tools)
- [Credentials and selection](#credentials-and-selection)
- [Files and verification](#files-and-verification)

## Lifecycle and recovery

Creating/importing a server persists its configuration then calls
`Manager.ConnectAsync`, so an unavailable endpoint cannot block the admin
request. Status is `connecting`, `connected`, `failed`, or `disabled`; a
connected server includes its discovered tool count. The admin response also
retains the latest attempt, success, failure, connection latency, and last
connection error, so an operator can distinguish a current failure from an
in-progress reconnect. This diagnostic history excludes credentials and tool
payloads.

Each attempt has a 10-second deadline plus a five-second outer grace that
force-settles a hung attempt as `failed/not_responding`. Initial failures and
unexpected session deaths retry after 15 seconds. Generation guards prevent a
late old attempt from overwriting an edit or reconnect. Disabling, removing, or
replacing a server cancels its retry.

## Transports and tools

Stdio stores command, args, and an optional environment map. The MCP session
starts/owns that subprocess. HTTP stores a streamable-MCP URL and optional
static headers; chatz decrypts them at connect time and applies them to every
MCP request, including `Authorization`.

Tools are exposed as `<server>__<tool>` and routed back to the owning client. A
tool-level failure becomes an `IsError=true` tool result and remains visible;
transport/RPC failure remains an error.

## Credentials and selection

`CHATZ_SECRETS_KEY` is a base64-encoded 32-byte AES-256-GCM key. It seals every
stored HTTP header and stdio environment map. If unset, storage fails instead
of falling back to plaintext.

All headers are sealed. `Authorization` is additionally masked in admin
responses: it returns as `<scheme> [SECRET]`; submitting that unchanged mask
retains the stored value, omission removes it, and any other value replaces it.
Other headers are shown as configured. Stdio environment values are decrypted
for the trusted admin edit form. Keep keys and bearer values only in gitignored
`.env`. The mask/restore logic is in `internal/pkg/http/server/handler_mcp.go`.

Globally enabled connected servers are offered to all chats by default. A
composer can disable one server for one chat, hiding its tools from that model.
That toggle is a preference, not a security boundary.

Claude-style `.mcp.json` import accepts `mcpServers` entries with either
`command`/`args`/`env` (stdio) or `url`/`headers` (HTTP). If both command and
URL exist, stdio wins. Names are sorted for deterministic output.

## Files and verification

| Path | Responsibility |
| --- | --- |
| `manager.go` | Connections, statuses, retries, aggregated tools. |
| `client.go` | SDK session, transports, and tool calls. |
| `import.go` | `.mcp.json` parsing and sealed models. |
| `to_api.go` | Admin API projection. |

Run `make test` for transport/import/retry coverage. The real-browser MCP flows
(per-chat picker + admin connect/tools/edit/reconnect/fail) live in the Go e2e
suite: `tests/e2e/chat_mcp_test.go` and `tests/e2e/mcp_admin_test.go`, run via
`make test-e2e`.
