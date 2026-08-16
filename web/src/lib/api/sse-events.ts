// Typed SSE event model mirroring the server's Anthropic-style wire format
// (internal/pkg/sse/types.go). The server frames each event as
// `event: <name>\ndata: <json>\n\n`; the `data` JSON's `type` field is the same
// name. We model the parsed events as a discriminated union keyed on `type`.
//
// Content-block events (`content_block_start` / `content_block_delta`) are
// polymorphic on the server: the same event name carries text, tool_use,
// tool_result, or input-json shapes. We narrow those by inspecting the nested
// block/delta `type` (see narrow* helpers) rather than trusting a bare `any`.

// --- wire event names (server EventType) -----------------------------------
import { isChatTurnStatus, type ChatTurnStatus } from "$lib/common/turn-status";

export const SSE_CHAT_STATUS = "chat_status";
export const SSE_MESSAGE_START = "message_start";
export const SSE_CONTENT_BLOCK_START = "content_block_start";
export const SSE_PING = "ping";
export const SSE_CONTENT_BLOCK_DELTA = "content_block_delta";
export const SSE_CONTENT_BLOCK_STOP = "content_block_stop";
export const SSE_MESSAGE_DELTA = "message_delta";
export const SSE_MESSAGE_STOP = "message_stop";
export const SSE_ERROR = "error";

// --- nested block / delta type tags (server ContentBlockType) --------------
export const BLOCK_TEXT = "text";
export const BLOCK_TEXT_DELTA = "text_delta";
export const BLOCK_THINKING = "thinking";
export const BLOCK_THINKING_DELTA = "thinking_delta";
export const BLOCK_TOOL_USE = "tool_use";
export const BLOCK_TOOL_RESULT = "tool_result";
export const BLOCK_INPUT_JSON_DELTA = "input_json_delta";
export const BLOCK_JSON_PARTIAL = "json_partial";

// --- parsed event union ----------------------------------------------------
// Each variant is a lossless-enough projection of the server struct for the UI:
// we keep the fields the conversation assembler reads and drop usage counters
// we don't render.

export interface MessageStartEvent {
  readonly kind: typeof SSE_MESSAGE_START;
  readonly messageId: string;
  readonly conversationId: string;
  readonly model: string;
}

export interface PingEvent {
  readonly kind: typeof SSE_PING;
}

export interface ChatStatusEvent {
  readonly kind: typeof SSE_CHAT_STATUS;
  readonly status: ChatTurnStatus;
}

export interface TextBlockStartEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_START;
  readonly block: typeof BLOCK_TEXT;
  readonly index: number;
}

export interface ThinkingBlockStartEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_START;
  readonly block: typeof BLOCK_THINKING;
  readonly index: number;
}

export interface ToolUseStartEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_START;
  readonly block: typeof BLOCK_TOOL_USE;
  readonly index: number;
  readonly toolUseId: string;
  readonly name: string;
}

export interface ToolResultStartEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_START;
  readonly block: typeof BLOCK_TOOL_RESULT;
  readonly index: number;
  readonly toolUseId: string;
  readonly isError: boolean;
}

export interface TextDeltaEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_DELTA;
  readonly block: typeof BLOCK_TEXT_DELTA;
  readonly index: number;
  readonly text: string;
}

export interface ThinkingDeltaEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_DELTA;
  readonly block: typeof BLOCK_THINKING_DELTA;
  readonly index: number;
  readonly text: string;
}

export interface ToolInputDeltaEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_DELTA;
  readonly block: typeof BLOCK_INPUT_JSON_DELTA;
  readonly index: number;
  readonly partialJson: string;
}

export interface ToolResultDeltaEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_DELTA;
  readonly block: typeof BLOCK_JSON_PARTIAL;
  readonly index: number;
  readonly text: string;
}

export interface ContentBlockStopEvent {
  readonly kind: typeof SSE_CONTENT_BLOCK_STOP;
  readonly index: number;
}

export interface MessageDeltaEvent {
  readonly kind: typeof SSE_MESSAGE_DELTA;
  readonly stopReason: string;
}

export interface MessageStopEvent {
  readonly kind: typeof SSE_MESSAGE_STOP;
}

export interface ErrorEvent {
  readonly kind: typeof SSE_ERROR;
  readonly code: string;
  readonly message: string;
}

export type SSEEvent =
  | ChatStatusEvent
  | MessageStartEvent
  | PingEvent
  | TextBlockStartEvent
  | ThinkingBlockStartEvent
  | ToolUseStartEvent
  | ToolResultStartEvent
  | TextDeltaEvent
  | ThinkingDeltaEvent
  | ToolInputDeltaEvent
  | ToolResultDeltaEvent
  | ContentBlockStopEvent
  | MessageDeltaEvent
  | MessageStopEvent
  | ErrorEvent;

// --- narrowing helpers on unknown JSON -------------------------------------
// The wire `data` is JSON.parse'd to unknown; these read fields defensively so
// a malformed frame yields null (dropped) rather than a crash — matching the
// server parser's "warn + drop" posture.

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null;
}

function asString(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function asNumber(v: unknown): number {
  return typeof v === "number" ? v : 0;
}

function asBool(v: unknown): boolean {
  return v === true;
}

function blockType(data: Record<string, unknown>): string {
  const block = data.content_block;

  return isObject(block) ? asString(block.type) : "";
}

function deltaType(data: Record<string, unknown>): string {
  const delta = data.delta;

  return isObject(delta) ? asString(delta.type) : "";
}

// parseEvent maps a raw event name + parsed JSON data to a typed SSEEvent, or
// null when the frame is unrecognized/malformed (caller drops it).
export function parseEvent(eventName: string, data: unknown): SSEEvent | null {
  if (!isObject(data)) {
    return null;
  }

  switch (eventName) {
    case SSE_CHAT_STATUS:
      return parseChatStatus(data);
    case SSE_MESSAGE_START:
      return parseMessageStart(data);
    case SSE_PING:
      return { kind: SSE_PING };
    case SSE_CONTENT_BLOCK_START:
      return parseBlockStart(data);
    case SSE_CONTENT_BLOCK_DELTA:
      return parseBlockDelta(data);
    case SSE_CONTENT_BLOCK_STOP:
      return { kind: SSE_CONTENT_BLOCK_STOP, index: asNumber(data.index) };
    case SSE_MESSAGE_DELTA:
      return parseMessageDelta(data);
    case SSE_MESSAGE_STOP:
      return { kind: SSE_MESSAGE_STOP };
    case SSE_ERROR:
      return parseError(data);
    default:
      return null;
  }
}

function parseChatStatus(data: Record<string, unknown>): SSEEvent | null {
  const status = asString(data.status);
  if (asString(data.type) !== SSE_CHAT_STATUS || !isChatTurnStatus(status)) {
    return null;
  }

  return { kind: SSE_CHAT_STATUS, status };
}

function parseError(data: Record<string, unknown>): SSEEvent | null {
  const detail = data.error;
  if (!isObject(detail)) {
    return null;
  }

  return {
    kind: SSE_ERROR,
    code: asString(detail.type),
    message: asString(detail.message),
  };
}

function parseMessageStart(data: Record<string, unknown>): SSEEvent | null {
  const message = data.message;
  if (!isObject(message)) {
    return null;
  }

  return {
    kind: SSE_MESSAGE_START,
    messageId: asString(message.id),
    // The wire key is stream_id: essessey streams content blocks for anything,
    // not only conversations. chatz keeps its own "conversation" vocabulary, so
    // the rename stops here, at the parse boundary.
    conversationId: asString(message.stream_id),
    model: asString(message.model),
  };
}

function parseBlockStart(data: Record<string, unknown>): SSEEvent | null {
  const index = asNumber(data.index);
  const block = data.content_block;
  const t = blockType(data);

  if (t === BLOCK_TOOL_USE && isObject(block)) {
    return {
      kind: SSE_CONTENT_BLOCK_START,
      block: BLOCK_TOOL_USE,
      index,
      toolUseId: asString(block.id),
      name: asString(block.name),
    };
  }

  if (t === BLOCK_TOOL_RESULT && isObject(block)) {
    return {
      kind: SSE_CONTENT_BLOCK_START,
      block: BLOCK_TOOL_RESULT,
      index,
      toolUseId: asString(block.tool_use_id),
      isError: asBool(block.is_error),
    };
  }

  if (t === BLOCK_THINKING) {
    return { kind: SSE_CONTENT_BLOCK_START, block: BLOCK_THINKING, index };
  }

  // Text block start (server sends type "text"; treat empty/unknown as text to
  // mirror the Go parser's text fallthrough).
  return { kind: SSE_CONTENT_BLOCK_START, block: BLOCK_TEXT, index };
}

function parseBlockDelta(data: Record<string, unknown>): SSEEvent | null {
  const index = asNumber(data.index);
  const delta = data.delta;
  if (!isObject(delta)) {
    return null;
  }

  switch (deltaType(data)) {
    case BLOCK_TEXT_DELTA:
      return {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_TEXT_DELTA,
        index,
        text: asString(delta.text),
      };
    case BLOCK_THINKING_DELTA:
      return {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_THINKING_DELTA,
        index,
        text: asString(delta.text),
      };
    case BLOCK_INPUT_JSON_DELTA:
      return {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_INPUT_JSON_DELTA,
        index,
        partialJson: asString(delta.partial_json),
      };
    case BLOCK_JSON_PARTIAL:
      return {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_JSON_PARTIAL,
        index,
        text: asString(delta.text),
      };
    default:
      return null;
  }
}

function parseMessageDelta(data: Record<string, unknown>): SSEEvent | null {
  const delta = data.delta;

  return {
    kind: SSE_MESSAGE_DELTA,
    stopReason: isObject(delta) ? asString(delta.stop_reason) : "",
  };
}
