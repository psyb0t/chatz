---
title: "Chats"
description: "How chatz runs a conversation: per-chat model settings, the history token budget, context preview, streaming status, tool rounds, retries and demo mode."
nav_order: 5
---

# Chats

Each chat belongs to one user and keeps its own model, settings and MCP server choices. This page explains what happens to a message after you send it. The developer view of the same flow is the [chat orchestration README](https://github.com/psyb0t/chatz/blob/main/internal/pkg/core/chats/README.md).

## Creating, finding and deleting chats

- A new chat takes its title from the first message, cut to 60 characters. You can rename it later; a blank title is refused and a longer one is cut to 60 characters.
- The sidebar search matches chat titles, case-insensitively, up to 256 characters of query.
- Deleting a chat removes it from your list and from every API response. It is a soft delete: the rows stay in the database.
- Deleting a user deletes that user's sessions and chats.

## Settings

The composer's settings panel controls how the model generates. Every setting is optional; one you leave empty uses the provider's default.

| Setting | Allowed values | What it does |
| --- | --- | --- |
| `temperature` | 0 to 2 | Sampling temperature. |
| `topP` | 0 to 1 | Nucleus sampling. |
| `reasoningEffort` | `minimal`, `low`, `medium`, `high` | Reasoning depth for models that support it. |
| `maxOutputTokens` | positive integer | Cap on the reply length. |
| `maxHistoryTokens` | positive integer | Token budget for the prompt chatz sends. Default 100,000. Not passed to the provider. |

Saving settings replaces the whole set, so a field you clear goes back to the provider default. A value out of range is refused with `VALIDATION_FAILED`.

## What the model receives

Every model request is built in the same order:

```text
system message -> newest history that fits -> current message
```

- The system message holds the base instructions and the generative UI guide. It is not shown in the chat but always counts against the budget.
- The current message is always included.
- History fills the rest of `maxHistoryTokens`, newest first. When it does not fit, chatz drops the oldest complete turns. An assistant tool call and its results are dropped together, so the model never sees a tool result without its call.
- Tokens are counted with the `o200k_base` tokenizer built into the binary.

Before you send, the composer shows how much of the budget the system message, history and draft take and how many turns would be left out. It gets those numbers from `POST /api/v1/chats/{chatId}/context-preview`, which runs the same selection on your unsent draft and saves nothing.

## Streaming and status

Replies stream over Server-Sent Events. Reasoning, text, tool calls and tool results appear in the order they arrive. While a turn runs, the composer shows a status with a timer:

| Status | Meaning |
| --- | --- |
| `connecting` | chatz is starting the request to the model. |
| `waiting_first_token` | Connected, waiting for the first token. |
| `streaming` | Output is arriving. |
| `running_tool` | An MCP tool call is running. |
| `retrying` | The request failed before streaming and chatz is trying a fallback model. |

The status is display only. It is never saved as part of the reply.

## Tools in a turn

When MCP servers are enabled for the chat, the model can call their tools. One turn allows up to 12 rounds of tool calls, runs up to 8 tool calls at the same time, and then asks the model for a final answer. A tool that reports an error returns that error to the model as a visible tool result instead of ending the turn. [MCP servers](mcp.md) explains which tools a chat sees.

## Saving and interrupted turns

Your message is saved before the stream opens, so a refresh, a stop or a crash never loses it. While the reply streams, chatz saves the useful text and reasoning so far as a checkpoint. When the turn completes, the checkpoint is replaced by the finished reply and its tool calls in one step, and token usage is recorded.

If a turn never completes, the chat shows your message and the partial reply labeled as an interrupted response. Interrupted messages are never sent back to the model in later turns. Partial tool calls and results are not shown.

Only one turn runs per chat at a time.

## Failures and retries

A failure before the stream starts comes back as a normal JSON error, such as `unknown model` or `chat not found`. A failure during the stream ends it with an `error` event and one of these messages:

| Kind | Message shown |
| --- | --- |
| `upstream_timeout` | The model did not respond in time. Try again. |
| `rate_limited` | The model provider is busy. Try again shortly. |
| `model_unavailable` | The selected model is unavailable. Contact an administrator. |
| `context_limit` | This conversation is too large for the selected model. |
| `request_failed` | The model request failed. Try again. |

After a provider failure the web app offers a retry that sends the same prompt to the same model as a new turn. Stopping a turn yourself sends no error and offers no retry.

## Demo mode

With `CHATZ_SHOWCASE_MODE=true`, a message that matches one of the seven demo prompts exactly gets a fixed reply instead of a model call. The reply streams at token speed with thinking, synthetic tool cards and a generative UI dashboard, and is saved like any other turn. The prompts are listed on [Getting started](getting-started.md#try-demo-mode).

Demo mode does not hide models, change MCP servers or catch near matches: a message that differs by one character goes to the selected model. It is off by default.
