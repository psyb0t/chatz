import {
  continueChatStream,
  StreamError,
  type StreamHandle,
} from "$lib/api/stream";
import {
  SSE_MESSAGE_START,
  SSE_CHAT_STATUS,
  SSE_CONTENT_BLOCK_START,
  SSE_CONTENT_BLOCK_DELTA,
  SSE_MESSAGE_DELTA,
  SSE_MESSAGE_STOP,
  SSE_ERROR,
  SSE_PING,
  BLOCK_TEXT,
  BLOCK_TEXT_DELTA,
  BLOCK_THINKING,
  BLOCK_THINKING_DELTA,
  BLOCK_TOOL_USE,
  BLOCK_TOOL_RESULT,
  BLOCK_INPUT_JSON_DELTA,
  BLOCK_JSON_PARTIAL,
  type SSEEvent,
} from "$lib/api/sse-events";
import type { ChatTurnStatus } from "$lib/common/turn-status";
import {
  getChat,
  listChatMessages,
  updateChatSettings,
  type Message as ApiMessage,
  type ChatSettings,
} from "$lib/api/client";
import { chats } from "$lib/stores/chats.svelte";
import { log } from "$lib/log";
import {
  EVENT_SSE_EVENT,
  EVENT_SSE_TEXT_DELTA,
  EVENT_SSE_DONE,
  EVENT_SSE_ERROR,
  EVENT_TOOL_USE,
  EVENT_TOOL_RESULT,
  EVENT_THINKING,
  EVENT_CHAT_SEND,
  EVENT_CHAT_LOAD,
  EVENT_CHAT_LOAD_ERROR,
} from "$lib/common/log-events";

export const ROLE_USER = "user";
export const ROLE_ASSISTANT = "assistant";
export const ROLE_TOOL = "tool";

export type MessageRole =
  typeof ROLE_USER | typeof ROLE_ASSISTANT | typeof ROLE_TOOL;

// Block-kind tags for the message's ordered content model. A turn is a list of
// blocks in stream-arrival order — text, thinking, and tool blocks interleave
// exactly as the model emitted them (text, tool, more text, another tool, ...),
// so the render is never "all tool cards on top, all text below".
export const BLOCK_KIND_TEXT = "text";
export const BLOCK_KIND_THINKING = "thinking";
export const BLOCK_KIND_TOOL = "tool";

// MAX_CHAT_TITLE_LEN caps the optimistic new-chat title to match the backend's
// per-chat title cap (runes), so the sidebar shows the same trimmed title
// before and after a reload.
export const MAX_CHAT_TITLE_LEN = 60;

// HISTORY_LOAD_LIMIT is a generous page size so a typical chat's full history
// still renders in one page, matching what the old embedded (unpaginated)
// `messages` array showed. Chats with more than this many visible messages
// will only show the oldest HISTORY_LOAD_LIMIT on load — a "load older
// messages" / infinite-scroll UI is a natural follow-up, not built here (see
// load()'s doc).
const HISTORY_LOAD_LIMIT = 200;
const STREAM_FAILURE_FALLBACK_MESSAGE = "The model request failed. Try again.";
const TURN_ELAPSED_INTERVAL_MS = 1000;

interface RetryTurn {
  text: string;
  model: string;
}

// ToolCall is one tool_use block: name + accumulating args JSON, and its result
// (attached when the matching tool_result arrives, keyed by toolUseId).
export interface ToolCall {
  toolUseId: string;
  name: string;
  args: string;
  result: string | null;
  isError: boolean;
  done: boolean;
}

// TextBlock is a prose run — rendered as markdown + inline ```spec json-render.
export interface TextBlock {
  kind: typeof BLOCK_KIND_TEXT;
  id: string;
  text: string;
}

// ThinkingBlock is a reasoning run — rendered collapsed/muted, apart from the
// answer text.
export interface ThinkingBlock {
  kind: typeof BLOCK_KIND_THINKING;
  id: string;
  text: string;
}

// ToolBlock is one tool call rendered as an expandable tool card.
export interface ToolBlock {
  kind: typeof BLOCK_KIND_TOOL;
  id: string;
  call: ToolCall;
}

export type MsgBlock = TextBlock | ThinkingBlock | ToolBlock;

// ConvMessage is one rendered turn. Assistant turns accumulate blocks in arrival
// order as the stream arrives; `streaming` gates the live caret.
export interface ConvMessage {
  id: string;
  role: MessageRole;
  blocks: MsgBlock[];
  streaming: boolean;
  incomplete: boolean;
}

// blockRef maps a live content-block index to the target it builds within the
// current assistant message, so deltas + results route correctly. Text/thinking
// blocks are addressed by their block id; tool_use/tool_result deltas route by
// toolUseId (tool_result attaches to the tool block opened by its tool_use).
type BlockRef =
  | { kind: typeof BLOCK_TEXT; blockId: string }
  | { kind: typeof BLOCK_THINKING; blockId: string }
  | { kind: typeof BLOCK_TOOL_USE; toolUseId: string }
  | { kind: typeof BLOCK_TOOL_RESULT; toolUseId: string };

let localSeq = 0;

function nextLocalId(prefix: string): string {
  localSeq += 1;

  return `${prefix}-local-${localSeq}`;
}

// messageText returns a message's plain answer text (its text blocks joined) —
// used for the sidebar title and the create-chat adoption path. Thinking + tool
// blocks are excluded; they are not part of the durable message text.
export function messageText(message: ConvMessage): string {
  return message.blocks
    .filter((block): block is TextBlock => block.kind === BLOCK_KIND_TEXT)
    .map((block) => block.text)
    .join("");
}

// capChatTitle truncates to MAX_CHAT_TITLE_LEN code points (rune-safe via the
// spread), matching the backend's title cap so the optimistic + reloaded titles
// agree.
function capChatTitle(text: string): string {
  const chars = [...text];

  return chars.length > MAX_CHAT_TITLE_LEN
    ? chars.slice(0, MAX_CHAT_TITLE_LEN).join("")
    : text;
}

// ConversationStore owns the reactive timeline for the active chat. It is a
// singleton reused across route changes; load()/reset() re-point it at a chat.
class ConversationStore {
  chatId = $state<string | null>(null);
  // The model the active chat currently uses (its last-used model) — the
  // composer reflects this in the picker. Switching the picker on a continued
  // turn updates it (persisted server-side).
  model = $state<string | null>(null);
  // The active chat's per-chat generation settings (temperature, reasoning
  // effort, token caps). Null until a chat is loaded; the settings panel reads
  // + writes it through updateSettings.
  settings = $state<ChatSettings | null>(null);
  messages = $state<ConvMessage[]>([]);
  streaming = $state(false);
  // loading gates the history-fetch spinner in load(); it is NOT set during a
  // live turn (that is `streaming`). error holds the last load/stream failure
  // message for inline display in the chat pane, cleared on the next attempt.
  loading = $state(false);
  error = $state<string | null>(null);
  turnStatus = $state<ChatTurnStatus | null>(null);
  elapsedSeconds = $state(0);
  retryAvailable = $state(false);

  private handle: StreamHandle | null = null;
  private elapsedTimer: ReturnType<typeof setInterval> | null = null;
  private turnStartedAt: number | null = null;
  private currentTurn: RetryTurn | null = null;
  private retryTurn: RetryTurn | null = null;
  // The streaming assistant message is tracked by ID, never by object
  // reference: $state deep-proxies the objects inside `messages`, so mutating a
  // raw pre-proxy reference updates state the UI proxies never see (the render
  // freezes at the initial empty text). Every update goes through
  // updateAssistant, which immutably replaces the message in the array — new
  // references are the one reactivity contract that can't silently break.
  private assistantId: string | null = null;
  private blockRefs = new Map<number, BlockRef>();
  // A route can request an empty history immediately before the user starts a
  // turn. That stale snapshot must not replace the optimistic live timeline.
  private loadRevision = 0;
  // Set at the top of send() for the turn about to start; onMessageStart
  // reads it to decide whether to title the chat (only its first message —
  // mirrors the backend's Continue(), which only titles an untitled chat).
  private turnIsFirstMessage = false;

  // reset clears the timeline for a fresh compose (new-chat) state.
  reset(): void {
    this.stop();
    this.loadRevision++;
    this.chatId = null;
    this.model = null;
    this.settings = null;
    this.messages = [];
    this.error = null;
    this.turnStatus = null;
    this.retryAvailable = false;
    this.retryTurn = null;
  }

  // updateSettings persists the chat's generation settings and reflects the
  // server's stored copy back into the store. No-op with no active chat.
  async updateSettings(next: ChatSettings): Promise<void> {
    const chatId = this.chatId;
    if (chatId === null) {
      return;
    }

    this.settings = await updateChatSettings(chatId, next);
  }

  // load reconstructs an existing chat: metadata from GET /chats/{id} and the
  // message timeline from GET /chats/{id}/messages (its own paginated
  // sub-resource — see HISTORY_LOAD_LIMIT). History is flat text per message
  // (user/assistant/tool) — no live tool/thinking blocks — so each message
  // renders as a single text block.
  async load(chatId: string): Promise<void> {
    this.stop();
    const loadRevision = ++this.loadRevision;
    this.chatId = chatId;
    this.error = null;
    this.turnStatus = null;
    this.retryAvailable = false;
    this.retryTurn = null;
    this.messages = [];
    this.loading = true;

    try {
      const [chat, page] = await Promise.all([
        getChat(chatId),
        listChatMessages(chatId, { limit: HISTORY_LOAD_LIMIT, offset: 0 }),
      ]);
      if (loadRevision !== this.loadRevision) {
        return;
      }

      this.messages = groupApiMessages(page.items);
      this.model = chat.model;
      this.settings = chat.settings ?? {};
      log.info(EVENT_CHAT_LOAD, {
        chat_id: chatId,
        count: page.items.length,
      });
    } catch (err) {
      if (loadRevision !== this.loadRevision) {
        return;
      }

      const message = err instanceof Error ? err.message : String(err);
      this.error = message;
      log.error(EVENT_CHAT_LOAD_ERROR, { chat_id: chatId, message });
    } finally {
      if (loadRevision === this.loadRevision) {
        this.loading = false;
      }
    }
  }

  // dismissError clears a surfaced load/stream error so the inline banner in the
  // chat pane can be dismissed without triggering a reload.
  dismissError(): void {
    this.error = null;
  }

  // retryLoad re-fetches the active chat's history after a load failure.
  async retryLoad(): Promise<void> {
    if (this.chatId === null) {
      return;
    }

    await this.load(this.chatId);
  }

  async retryLastTurn(): Promise<void> {
    const retryTurn = this.retryTurn;
    if (retryTurn === null || this.streaming) {
      return;
    }

    await this.send(retryTurn.text, retryTurn.model);
  }

  // send streams the user's text as the next turn of the active chat. A chat
  // always exists by the time the composer can call this — routing always
  // lands on a real chat id (a fresh "New chat" resolves via
  // chats.goToNewChat before the composer ever renders) — so this is a
  // continue-only call; the null-chatId branch below is a defensive guard
  // against that routing invariant, not an expected path.
  async send(text: string, model: string): Promise<void> {
    const trimmed = text.trim();
    const chatId = this.chatId;
    if (trimmed === "" || this.streaming) {
      return;
    }

    if (chatId === null) {
      log.error(EVENT_CHAT_SEND, {
        message: "send called with no active chat",
      });

      return;
    }

    this.loadRevision++;
    this.loading = false;
    this.error = null;
    this.retryAvailable = false;
    this.retryTurn = null;
    this.currentTurn = { text: trimmed, model };
    this.turnIsFirstMessage = this.messages.length === 0;
    this.pushUser(trimmed);

    // Remember the model used this turn so the picker — and a reopened chat —
    // come back to the last-used model (a switch is persisted server-side).
    this.model = model;

    log.info(EVENT_CHAT_SEND, { chat_id: chatId });
    const handle = continueChatStream(chatId, { message: trimmed, model });

    await this.consume(handle);
  }

  // stop aborts an in-flight stream and finalizes the open assistant message.
  stop(): void {
    if (this.handle !== null) {
      this.handle.abort();
      this.handle = null;
    }

    this.finalizeAssistant();
    this.streaming = false;
    this.turnStatus = null;
    this.currentTurn = null;
    this.stopElapsed();
  }

  private async consume(handle: StreamHandle): Promise<void> {
    this.handle = handle;
    this.streaming = true;
    this.startElapsed();

    try {
      for await (const event of handle.events) {
        this.apply(event);
      }

      log.info(EVENT_SSE_DONE, { chat_id: this.chatId ?? "" });
    } catch (err) {
      this.onError(err);
    } finally {
      this.finalizeAssistant();
      this.streaming = false;
      this.turnStatus = null;
      this.handle = null;
      this.currentTurn = null;
      this.stopElapsed();
    }
  }

  private apply(event: SSEEvent): void {
    if (event.kind !== SSE_PING) {
      log.debug(EVENT_SSE_EVENT, { type: event.kind });
    }

    switch (event.kind) {
      case SSE_CHAT_STATUS:
        this.turnStatus = event.status;

        return;
      case SSE_MESSAGE_START:
        this.onMessageStart();

        return;
      case SSE_CONTENT_BLOCK_START:
        this.onBlockStart(event);

        return;
      case SSE_CONTENT_BLOCK_DELTA:
        this.onBlockDelta(event);

        return;
      case SSE_MESSAGE_DELTA:
      case SSE_MESSAGE_STOP:
        this.finalizeAssistant();

        return;
      case SSE_ERROR:
        this.onStreamFailure(event.message);

        return;
      case SSE_PING:
        return;
    }
  }

  // onMessageStart bumps the active chat to the top of the sidebar list (its
  // first message just made it non-empty server-side — see chats.touch) and
  // opens the turn's assistant message. Titling only happens on the chat's
  // first message, mirroring the backend's Continue().
  private onMessageStart(): void {
    if (this.chatId !== null) {
      const title = this.turnIsFirstMessage
        ? capChatTitle(this.firstUserText())
        : undefined;
      chats.touch(this.chatId, title);
      this.turnIsFirstMessage = false;
    }

    this.openAssistant();
  }

  private firstUserText(): string {
    for (const m of this.messages) {
      if (m.role === ROLE_USER) {
        return messageText(m);
      }
    }

    return "";
  }

  private onBlockStart(
    event: Extract<SSEEvent, { kind: typeof SSE_CONTENT_BLOCK_START }>,
  ): void {
    switch (event.block) {
      case BLOCK_TEXT:
        this.openTextBlock(event.index);

        return;
      case BLOCK_THINKING:
        this.openThinkingBlock(event.index);

        return;
      case BLOCK_TOOL_USE:
        this.openToolCall(event.index, event.toolUseId, event.name);

        return;
      case BLOCK_TOOL_RESULT:
        this.openToolResult(event.index, event.toolUseId, event.isError);

        return;
    }
  }

  private openTextBlock(index: number): void {
    this.openAssistant();
    const id = nextLocalId(BLOCK_KIND_TEXT);
    this.appendBlock({ kind: BLOCK_KIND_TEXT, id, text: "" });
    this.blockRefs.set(index, { kind: BLOCK_TEXT, blockId: id });
  }

  private openThinkingBlock(index: number): void {
    this.openAssistant();
    const id = nextLocalId(BLOCK_KIND_THINKING);
    this.appendBlock({ kind: BLOCK_KIND_THINKING, id, text: "" });
    this.blockRefs.set(index, { kind: BLOCK_THINKING, blockId: id });
    log.info(EVENT_THINKING, {});
  }

  private openToolCall(index: number, toolUseId: string, name: string): void {
    this.openAssistant();
    const id = nextLocalId(BLOCK_KIND_TOOL);
    this.appendBlock({
      kind: BLOCK_KIND_TOOL,
      id,
      call: {
        toolUseId,
        name,
        args: "",
        result: null,
        isError: false,
        done: false,
      },
    });
    this.blockRefs.set(index, { kind: BLOCK_TOOL_USE, toolUseId });
    log.info(EVENT_TOOL_USE, { name });
  }

  private openToolResult(
    index: number,
    toolUseId: string,
    isError: boolean,
  ): void {
    this.blockRefs.set(index, { kind: BLOCK_TOOL_RESULT, toolUseId });

    // The error flag rides the tool_result block start, before its payload; mark
    // the card so it renders the error state as the result streams in.
    if (isError) {
      this.patchTool(toolUseId, (call) => {
        call.isError = true;
      });
    }
  }

  private onBlockDelta(
    event: Extract<SSEEvent, { kind: typeof SSE_CONTENT_BLOCK_DELTA }>,
  ): void {
    switch (event.block) {
      case BLOCK_TEXT_DELTA:
        this.appendText(event.index, event.text);

        return;
      case BLOCK_THINKING_DELTA:
        this.appendThinking(event.index, event.text);

        return;
      case BLOCK_INPUT_JSON_DELTA:
        this.appendToolArgs(event.index, event.partialJson);

        return;
      case BLOCK_JSON_PARTIAL:
        this.appendToolResult(event.index, event.text);

        return;
    }
  }

  private appendText(index: number, text: string): void {
    if (text === "") {
      return;
    }

    const ref = this.blockRefs.get(index);
    if (ref === undefined || ref.kind !== BLOCK_TEXT) {
      return;
    }

    this.patchTextBlock(ref.blockId, BLOCK_KIND_TEXT, text);
    log.debug(EVENT_SSE_TEXT_DELTA, { len: text.length });
  }

  private appendThinking(index: number, text: string): void {
    if (text === "") {
      return;
    }

    const ref = this.blockRefs.get(index);
    if (ref === undefined || ref.kind !== BLOCK_THINKING) {
      return;
    }

    this.patchTextBlock(ref.blockId, BLOCK_KIND_THINKING, text);
  }

  private appendToolArgs(index: number, partialJson: string): void {
    const toolUseId = this.toolUseIdFor(index);
    if (toolUseId === null) {
      return;
    }

    this.patchTool(toolUseId, (call) => {
      call.args += partialJson;
    });
  }

  private appendToolResult(index: number, text: string): void {
    const toolUseId = this.toolUseIdFor(index);
    if (toolUseId === null) {
      return;
    }

    this.patchTool(toolUseId, (call) => {
      call.result = (call.result ?? "") + text;
    });
  }

  private toolUseIdFor(index: number): string | null {
    const ref = this.blockRefs.get(index);
    if (ref === undefined) {
      return null;
    }

    if (ref.kind === BLOCK_TOOL_USE || ref.kind === BLOCK_TOOL_RESULT) {
      return ref.toolUseId;
    }

    return null;
  }

  // appendBlock adds a new block to the streaming assistant message in order.
  private appendBlock(block: MsgBlock): void {
    this.updateAssistant((message) => ({
      ...message,
      blocks: [...message.blocks, block],
    }));
  }

  // patchTextBlock appends text to the text/thinking block with blockId,
  // replacing it immutably (see the assistantId comment for why replacement).
  private patchTextBlock(
    blockId: string,
    kind: typeof BLOCK_KIND_TEXT | typeof BLOCK_KIND_THINKING,
    text: string,
  ): void {
    this.updateAssistant((message) => ({
      ...message,
      blocks: message.blocks.map((block) => {
        if (block.id !== blockId || block.kind !== kind) {
          return block;
        }

        return { ...block, text: block.text + text };
      }),
    }));
  }

  // patchTool applies a mutation to the tool call matching toolUseId and, on a
  // result arriving, marks it done + logs.
  private patchTool(toolUseId: string, mutate: (call: ToolCall) => void): void {
    this.updateAssistant((message) => ({
      ...message,
      blocks: message.blocks.map((block) => {
        if (
          block.kind !== BLOCK_KIND_TOOL ||
          block.call.toolUseId !== toolUseId
        ) {
          return block;
        }

        const next = { ...block.call };
        mutate(next);
        if (next.result !== null && !block.call.done) {
          next.done = true;
          log.info(EVENT_TOOL_RESULT, {
            name: next.name,
            is_error: next.isError,
          });
        }

        return { ...block, call: next };
      }),
    }));
  }

  // openAssistant ensures the turn's streaming assistant message exists,
  // creating an empty one on the first content of a turn.
  private openAssistant(): void {
    if (this.assistantId !== null) {
      return;
    }

    const message: ConvMessage = {
      id: nextLocalId(ROLE_ASSISTANT),
      role: ROLE_ASSISTANT,
      blocks: [],
      streaming: true,
      incomplete: false,
    };
    this.assistantId = message.id;
    this.messages = [...this.messages, message];
  }

  // updateAssistant immutably replaces the streaming assistant message with an
  // updated copy (see the assistantId comment for why replacement, never
  // in-place mutation, is required).
  private updateAssistant(update: (message: ConvMessage) => ConvMessage): void {
    const id = this.assistantId;
    if (id === null) {
      return;
    }

    this.messages = this.messages.map((message) =>
      message.id === id ? update(message) : message,
    );
  }

  private finalizeAssistant(): void {
    if (this.assistantId === null) {
      return;
    }

    this.updateAssistant((message) => ({ ...message, streaming: false }));
    this.assistantId = null;
    this.blockRefs.clear();
  }

  private pushUser(text: string): void {
    const message: ConvMessage = {
      id: nextLocalId(ROLE_USER),
      role: ROLE_USER,
      blocks: [
        { kind: BLOCK_KIND_TEXT, id: nextLocalId(BLOCK_KIND_TEXT), text },
      ],
      streaming: false,
      incomplete: false,
    };
    this.messages = [...this.messages, message];
  }

  private onError(err: unknown): void {
    const message =
      err instanceof StreamError
        ? `${err.message} (status ${err.status})`
        : err instanceof Error
          ? err.message
          : String(err);
    this.error = message;
    this.captureRetryTurn();
    log.error(EVENT_SSE_ERROR, { message });
  }

  private onStreamFailure(message: string): void {
    this.error = message === "" ? STREAM_FAILURE_FALLBACK_MESSAGE : message;
    this.captureRetryTurn();
    log.error(EVENT_SSE_ERROR, { message: this.error });
  }

  private captureRetryTurn(): void {
    if (this.currentTurn === null) {
      return;
    }

    this.retryTurn = this.currentTurn;
    this.retryAvailable = true;
  }

  private startElapsed(): void {
    this.stopElapsed();
    this.turnStartedAt = Date.now();
    this.elapsedSeconds = 0;
    this.elapsedTimer = setInterval(() => {
      this.refreshElapsed();
    }, TURN_ELAPSED_INTERVAL_MS);
  }

  private stopElapsed(): void {
    if (this.elapsedTimer !== null) {
      clearInterval(this.elapsedTimer);
      this.elapsedTimer = null;
    }

    this.turnStartedAt = null;
    this.elapsedSeconds = 0;
  }

  private refreshElapsed(): void {
    if (this.turnStartedAt === null) {
      return;
    }

    this.elapsedSeconds = Math.floor(
      (Date.now() - this.turnStartedAt) / TURN_ELAPSED_INTERVAL_MS,
    );
  }
}

// groupApiMessages folds a flat, oldest-first page of stored rows into the
// SAME ConvMessage[] shape a live turn produces: one DB row per round/tool
// exchange, but ONE turn on screen. Consecutive assistant/tool rows between
// two user rows belong to one turn and fold into a single ConvMessage whose
// blocks interleave in arrival order (thinking, text, tool — matching what
// openThinkingBlock/openTextBlock/openToolCall build live); a user row is
// always its own ConvMessage. A tool-role row carries no block of its own —
// it attaches its result to the ToolBlock its toolCallId points at, exactly
// like a live tool_result SSE event attaches to the tool_use block that
// opened it.
function groupApiMessages(rows: ApiMessage[]): ConvMessage[] {
  const out: ConvMessage[] = [];
  let turn: ConvMessage | null = null;

  for (const m of rows) {
    if (m.role === ROLE_USER) {
      turn = null;
      out.push({
        id: m.id,
        role: ROLE_USER,
        blocks:
          m.content === ""
            ? []
            : [
                {
                  kind: BLOCK_KIND_TEXT,
                  id: nextLocalId(BLOCK_KIND_TEXT),
                  text: m.content,
                },
              ],
        streaming: false,
        incomplete: m.incomplete ?? false,
      });
      continue;
    }

    if (turn === null) {
      turn = {
        id: m.id,
        role: ROLE_ASSISTANT,
        blocks: [],
        streaming: false,
        incomplete: m.incomplete ?? false,
      };
      out.push(turn);
    }

    if (m.role === ROLE_TOOL) {
      attachToolResult(turn, m);
      continue;
    }

    turn.incomplete ||= m.incomplete ?? false;
    appendAssistantRowBlocks(turn, m);
  }

  return out;
}

// appendAssistantRowBlocks turns one assistant DB row into the blocks it
// represents, in the same order they streamed live: thinking, then text,
// then one open (unresolved) ToolBlock per requested tool call.
function appendAssistantRowBlocks(turn: ConvMessage, m: ApiMessage): void {
  if (m.thinking !== undefined && m.thinking !== "") {
    turn.blocks.push({
      kind: BLOCK_KIND_THINKING,
      id: nextLocalId(BLOCK_KIND_THINKING),
      text: m.thinking,
    });
  }

  if (m.content !== "") {
    turn.blocks.push({
      kind: BLOCK_KIND_TEXT,
      id: nextLocalId(BLOCK_KIND_TEXT),
      text: m.content,
    });
  }

  for (const call of m.toolCalls ?? []) {
    turn.blocks.push({
      kind: BLOCK_KIND_TOOL,
      id: nextLocalId(BLOCK_KIND_TOOL),
      call: {
        toolUseId: call.id,
        name: call.name,
        args: call.arguments,
        result: null,
        isError: false,
        done: false,
      },
    });
  }
}

// attachToolResult finds the ToolBlock a tool-role row answers (by
// toolCallId) within the turn still being folded and attaches the result —
// a row whose call isn't in this turn (e.g. a page boundary mid-turn) is
// dropped silently, same as a live tool_result with no matching blockRef.
function attachToolResult(turn: ConvMessage, m: ApiMessage): void {
  const block = turn.blocks.find(
    (b): b is ToolBlock =>
      b.kind === BLOCK_KIND_TOOL && b.call.toolUseId === m.toolCallId,
  );
  if (block === undefined) {
    return;
  }

  block.call.result = m.content;
  block.call.isError = m.isError ?? false;
  block.call.done = true;
}

export const conversation = new ConversationStore();
