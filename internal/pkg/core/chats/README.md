# Chat orchestration

`internal/pkg/core/chats` owns durable turns: model/fixed-response selection,
prompt construction, MCP tool execution, SSE streaming, and persistence. HTTP
handlers validate transport input and delegate here.

## Contents

- [Turn contract](#turn-contract)
- [History and settings](#history-and-settings)
- [MCP, demos, and showcase](#mcp-demos-and-showcase)
- [Debugging and verification](#debugging-and-verification)

## Turn contract

For a real turn, the order is intentional:

1. Resolve the model and acquire a per-chat lock.
2. Build the system message, bounded history, and current user message, the
   exact list sent to the upstream.
3. Save the user message as an incomplete durable row before opening SSE.
4. Checkpoint useful assistant text/reasoning at most once per interval under
   the same turn ID. Checkpoints contain no partial tool call/result data.
5. Stream ephemeral `chat_status` progress frames only after the matching
   provider/tool transition, then stream reasoning, text, tool-use, and
   tool-result blocks in arrival order.
6. On success, atomically remove the checkpoint, save assistant/tool rows as
   one completed turn, and record usage.
   Provider-native reasoning is stored as opaque JSON and restored byte-for-byte
   for providers that require signed thinking blocks to be replayed.

Visible history includes completed `user`, `assistant`, and `tool` rows plus an
incomplete user row and a useful incomplete assistant checkpoint. The browser
labels that checkpoint as an interrupted response. Incomplete tool rows stay
hidden. Incomplete rows are excluded from `loadHistory`, so neither a submitted
prompt nor a partial assistant answer can be replayed into a later model turn.
Do not reorder these writes.

The frontend store at `web/src/lib/stores/conversation.svelte.ts` adopts a new
chat id from `message_start` without clobbering a live stream with stale history.
It treats the closed `chat_status` enum as UI-only metadata: `connecting`,
`waiting_first_token`, `streaming`, `running_tool`, and `retrying` are never
persisted as assistant content and are cleared when the turn ends, fails, or is
stopped. The browser measures elapsed time locally while a status is active.
On a terminal provider failure it can retry the retained prompt and model as a
new durable turn; cancellation deliberately offers no retry.

## History and settings

The chat service supplies the system message, validated stored history, and
current user prompt to `elelem`. The engine applies the outbound shape:

```text
sticky system message → newest fitting history suffix → current user message
```

The system message includes the base instruction and generated GenUI guide. It
is invisible in history but always counted; the current user message is pinned
too. The default cap is 100,000 tokens. Elelem's default `o200k_base` counter
counts portable message/tool content and framing. It drops the oldest complete
unit until the transcript fits. An assistant tool call and its contiguous
results are one unit, so the upstream never gets an orphaned result. A live
unresolved exchange stays pinned, while a completed older exchange is
droppable.

The composer controls `temperature`, `topP`, `reasoningEffort`,
`maxOutputTokens`, and `maxHistoryTokens`. Updating settings fully replaces the
object; omitted generation settings use provider defaults. `maxHistoryTokens`
is a prompt budget, not an upstream generation parameter. See `api/api.yml` and
`web/src/lib/components/ChatSettings.svelte`.

Before a user sends, `POST /chats/{chatId}/context-preview` runs this same
loading, tokenization, and complete-unit selection path against the unsent
draft. The composer renders that response rather than estimating locally, so
its system/history/draft counts and omitted-turn indicator describe the prompt
the next stream will actually receive. The endpoint is ownership-checked and
does not persist the draft.

## MCP, demos, and showcase

MCP tools are namespaced as `<server>__<tool>`. A chat can disable a globally
enabled server in its composer, hiding that server's tools from the model. This
is a preference, not a security boundary. See
[`../../mcp/README.md`](../../mcp/README.md) for transport and credential rules.

| Trigger | Live model/MCP | Durable | Purpose |
| --- | --- | --- | --- |
| Exact showcase prompt | No for the match | Yes | Paced dashboard turn with deterministic thinking + synthetic tool cards. |

`CHATZ_SHOWCASE_MODE=true` (`make run-showcase`) only intercepts exact catalog
prompts. It never hides models, changes MCP setup, or captures near matches.

## Debugging and verification

`history_logging.go` receives each round's exact post-limiting transcript from
elelem immediately before the driver call. At `LOG_LEVEL=debug` it records
model, budget, token count, ordered messages, reasoning, and tool arguments.
`RedactText` masks secret-shaped values, but normal chat content can remain;
keep debug local.

| Path | Responsibility |
| --- | --- |
| `chats.go` / `stream.go` | Turn entry points, SSE production, persistence. |
| `history.go` / `history_logging.go` | Stored-history validation and per-round debug trace. |
| `turn_locks.go` / `mcp_tools.go` | Serialization and tool filtering. |
| `prompts/` / `fixedresponses/` | GenUI guide and demo/showcase fixtures. |

Run `make test`, `make web-test`, and `make test-api` from the repository root.
The showcase render + streamed thinking/tool-cards/GenUI is covered by the Go
browser drivers in the api tier `tests/api/` (`showcase_test.go`, `smoke_test.go`).
