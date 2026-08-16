// Hand-rolled SSE client. openapi-fetch does JSON, not text/event-stream, so
// the two streaming chat endpoints are fetched directly here: POST the request,
// read response.body as a byte stream, decode UTF-8, buffer on the blank-line
// event terminator, split each frame's `event:`/`data:` lines, JSON-parse the
// data, and yield typed SSEEvents (see sse-events.ts). The server framing is
// `event: <name>\ndata: <json>\n\n` (internal/pkg/sse/sink.go FrameLines).
import { parseEvent, type SSEEvent } from "./sse-events";
import { log } from "$lib/log";
import { EVENT_SSE_OPEN } from "$lib/common/log-events";
import { API_PATH_CHATS, chatPath } from "$lib/common/api-paths";

const METHOD_POST = "POST";
const HEADER_CONTENT_TYPE = "Content-Type";
const CONTENT_TYPE_JSON = "application/json";
const FRAME_SEPARATOR = "\n\n";
const LINE_SEPARATOR = "\n";
const PREFIX_EVENT = "event: ";
const PREFIX_DATA = "data: ";

// StreamHandle bundles the typed event stream with its abort control so a caller
// (the conversation store) can render events and stop the turn.
export interface StreamHandle {
  readonly events: AsyncGenerator<SSEEvent, void, void>;
  abort(): void;
}

export interface CreateChatArgs {
  model: string;
  message: string;
}

export interface ContinueChatArgs {
  message: string;
  // Model for this + subsequent turns; switching it updates the chat's model.
  model: string;
}

// createChatStream POSTs /api/v1/chats to create a chat and stream its first
// turn. The new chat id arrives in the first message_start event.
export function createChatStream(args: CreateChatArgs): StreamHandle {
  return openStream(API_PATH_CHATS, {
    model: args.model,
    message: args.message,
  });
}

// continueChatStream POSTs /api/v1/chats/{chatId} to stream the next turn of an
// existing chat.
export function continueChatStream(
  chatId: string,
  args: ContinueChatArgs,
): StreamHandle {
  return openStream(chatPath(chatId), {
    message: args.message,
    model: args.model,
  });
}

function openStream(path: string, body: unknown): StreamHandle {
  const controller = new AbortController();
  log.debug(EVENT_SSE_OPEN, { path });

  const events = streamEvents(path, body, controller.signal);

  return {
    events,
    abort: () => controller.abort(),
  };
}

async function* streamEvents(
  path: string,
  body: unknown,
  signal: AbortSignal,
): AsyncGenerator<SSEEvent, void, void> {
  const response = await fetch(path, {
    method: METHOD_POST,
    credentials: "include",
    headers: { [HEADER_CONTENT_TYPE]: CONTENT_TYPE_JSON },
    body: JSON.stringify(body),
    signal,
  });

  if (!response.ok) {
    throw new StreamError(response.status, `stream request failed (${path})`);
  }

  if (response.body === null) {
    throw new StreamError(response.status, `stream has no body (${path})`);
  }

  yield* decodeFrames(response.body, signal);
}

// decodeFrames turns a raw byte ReadableStream into typed SSE events. Split out
// from the fetch so it is unit-testable against a synthetic stream: it handles
// frames split across chunk boundaries (buffer until FRAME_SEPARATOR) and the
// trailing frame that arrives without a terminator at stream end.
export async function* decodeFrames(
  stream: ReadableStream<Uint8Array>,
  signal?: AbortSignal,
): AsyncGenerator<SSEEvent, void, void> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }

      buffer += decoder.decode(value, { stream: true });

      let sepIndex = buffer.indexOf(FRAME_SEPARATOR);
      while (sepIndex !== -1) {
        const frame = buffer.slice(0, sepIndex);
        buffer = buffer.slice(sepIndex + FRAME_SEPARATOR.length);

        const event = decodeFrame(frame);
        if (event !== null) {
          yield event;
        }

        sepIndex = buffer.indexOf(FRAME_SEPARATOR);
      }
    }

    // A well-formed stream ends with a blank line, so `buffer` is normally empty
    // here; flush any trailing frame that lacked its terminator defensively.
    const tail = decodeFrame(buffer);
    if (tail !== null) {
      yield tail;
    }
  } finally {
    if (signal?.aborted !== true) {
      reader.releaseLock();
    }
  }
}

// decodeFrame parses one `event:`/`data:` frame into a typed event, or null when
// the frame is blank / has no data / is malformed JSON (dropped, matching the
// server parser's warn-and-drop posture).
function decodeFrame(frame: string): SSEEvent | null {
  const trimmed = frame.trim();
  if (trimmed === "") {
    return null;
  }

  let eventName = "";
  let dataRaw = "";

  for (const line of trimmed.split(LINE_SEPARATOR)) {
    if (line.startsWith(PREFIX_EVENT)) {
      eventName = line.slice(PREFIX_EVENT.length);

      continue;
    }

    if (line.startsWith(PREFIX_DATA)) {
      dataRaw = line.slice(PREFIX_DATA.length);
    }
  }

  if (eventName === "" || dataRaw === "") {
    return null;
  }

  let data: unknown;
  try {
    data = JSON.parse(dataRaw);
  } catch {
    return null;
  }

  return parseEvent(eventName, data);
}

// StreamError carries the HTTP status of a failed stream open so the store can
// surface a meaningful error state.
export class StreamError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "StreamError";
    this.status = status;
  }
}
