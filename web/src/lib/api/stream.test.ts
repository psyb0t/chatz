import { describe, it, expect } from "vitest";
import { decodeFrames } from "./stream";
import {
  SSE_CHAT_STATUS,
  SSE_MESSAGE_START,
  SSE_CONTENT_BLOCK_START,
  SSE_CONTENT_BLOCK_DELTA,
  SSE_CONTENT_BLOCK_STOP,
  SSE_MESSAGE_STOP,
  SSE_ERROR,
  BLOCK_TEXT,
  BLOCK_TEXT_DELTA,
  BLOCK_TOOL_USE,
  BLOCK_TOOL_RESULT,
  BLOCK_INPUT_JSON_DELTA,
  BLOCK_JSON_PARTIAL,
  type SSEEvent,
} from "./sse-events";
import { CHAT_TURN_STATUS_CONNECTING } from "$lib/common/turn-status";
import { frame } from "../../test/sse";

// streamFromChunks builds a ReadableStream emitting the given string chunks in
// order (UTF-8), so we can exercise frames split across chunk boundaries.
function streamFromChunks(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();

  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    },
  });
}

async function collect(
  stream: ReadableStream<Uint8Array>,
): Promise<SSEEvent[]> {
  const out: SSEEvent[] = [];
  for await (const event of decodeFrames(stream)) {
    out.push(event);
  }

  return out;
}

describe("decodeFrames", () => {
  it("parses a text-delta sequence into typed events", async () => {
    const chunks = [
      frame(SSE_MESSAGE_START, {
        type: SSE_MESSAGE_START,
        message: { id: "msg_1", stream_id: "chat-1", model: "gpt" },
      }),
      frame(SSE_CONTENT_BLOCK_START, {
        type: SSE_CONTENT_BLOCK_START,
        index: 0,
        content_block: { type: BLOCK_TEXT, text: "" },
      }),
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index: 0,
        delta: { type: BLOCK_TEXT_DELTA, text: "Hel" },
      }),
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index: 0,
        delta: { type: BLOCK_TEXT_DELTA, text: "lo" },
      }),
      frame(SSE_CONTENT_BLOCK_STOP, { type: SSE_CONTENT_BLOCK_STOP, index: 0 }),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ];

    const events = await collect(streamFromChunks(chunks));

    expect(events).toEqual([
      {
        kind: SSE_MESSAGE_START,
        messageId: "msg_1",
        conversationId: "chat-1",
        model: "gpt",
      },
      { kind: SSE_CONTENT_BLOCK_START, block: BLOCK_TEXT, index: 0 },
      {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_TEXT_DELTA,
        index: 0,
        text: "Hel",
      },
      {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_TEXT_DELTA,
        index: 0,
        text: "lo",
      },
      { kind: SSE_CONTENT_BLOCK_STOP, index: 0 },
      { kind: SSE_MESSAGE_STOP },
    ]);
  });

  it("reassembles a frame split across two chunks", async () => {
    const full = frame(SSE_CONTENT_BLOCK_DELTA, {
      type: SSE_CONTENT_BLOCK_DELTA,
      index: 0,
      delta: { type: BLOCK_TEXT_DELTA, text: "split" },
    });
    // Cut mid-JSON so neither half is a complete frame on its own.
    const cut = Math.floor(full.length / 2);

    const events = await collect(
      streamFromChunks([full.slice(0, cut), full.slice(cut)]),
    );

    expect(events).toEqual([
      {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_TEXT_DELTA,
        index: 0,
        text: "split",
      },
    ]);
  });

  it("parses a tool_use + tool_result sequence keyed by index", async () => {
    const chunks = [
      frame(SSE_CONTENT_BLOCK_START, {
        type: SSE_CONTENT_BLOCK_START,
        index: 0,
        content_block: { type: BLOCK_TOOL_USE, id: "tu-1", name: "list_users" },
      }),
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index: 0,
        delta: { type: BLOCK_INPUT_JSON_DELTA, partial_json: '{"limit":5}' },
      }),
      frame(SSE_CONTENT_BLOCK_STOP, { type: SSE_CONTENT_BLOCK_STOP, index: 0 }),
      frame(SSE_CONTENT_BLOCK_START, {
        type: SSE_CONTENT_BLOCK_START,
        index: 1,
        content_block: {
          type: BLOCK_TOOL_RESULT,
          tool_use_id: "tu-1",
          is_error: false,
        },
      }),
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index: 1,
        delta: { type: BLOCK_JSON_PARTIAL, text: "3 users" },
      }),
      frame(SSE_CONTENT_BLOCK_STOP, { type: SSE_CONTENT_BLOCK_STOP, index: 1 }),
    ];

    const events = await collect(streamFromChunks(chunks));

    expect(events).toEqual([
      {
        kind: SSE_CONTENT_BLOCK_START,
        block: BLOCK_TOOL_USE,
        index: 0,
        toolUseId: "tu-1",
        name: "list_users",
      },
      {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_INPUT_JSON_DELTA,
        index: 0,
        partialJson: '{"limit":5}',
      },
      { kind: SSE_CONTENT_BLOCK_STOP, index: 0 },
      {
        kind: SSE_CONTENT_BLOCK_START,
        block: BLOCK_TOOL_RESULT,
        index: 1,
        toolUseId: "tu-1",
        isError: false,
      },
      {
        kind: SSE_CONTENT_BLOCK_DELTA,
        block: BLOCK_JSON_PARTIAL,
        index: 1,
        text: "3 users",
      },
      { kind: SSE_CONTENT_BLOCK_STOP, index: 1 },
    ]);
  });

  it("flushes a trailing frame that lacks its terminator", async () => {
    const noTerminator = `event: ${SSE_MESSAGE_STOP}\ndata: ${JSON.stringify({
      type: SSE_MESSAGE_STOP,
    })}`;

    const events = await collect(streamFromChunks([noTerminator]));

    expect(events).toEqual([{ kind: SSE_MESSAGE_STOP }]);
  });

  it("drops malformed and unknown frames but keeps parsing", async () => {
    const chunks = [
      "event: content_block_delta\ndata: {not json}\n\n",
      frame("mystery_event", { type: "mystery_event" }),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ];

    const events = await collect(streamFromChunks(chunks));

    expect(events).toEqual([{ kind: SSE_MESSAGE_STOP }]);
  });

  it("parses a safe terminal error event", async () => {
    const events = await collect(
      streamFromChunks([
        frame(SSE_ERROR, {
          type: SSE_ERROR,
          error: {
            type: "upstream_timeout",
            message: "The model did not respond in time. Try again.",
          },
        }),
      ]),
    );

    expect(events).toEqual([
      {
        kind: SSE_ERROR,
        code: "upstream_timeout",
        message: "The model did not respond in time. Try again.",
      },
    ]);
  });

  it("parses only declared chat status values", async () => {
    const events = await collect(
      streamFromChunks([
        frame(SSE_CHAT_STATUS, {
          type: SSE_CHAT_STATUS,
          status: CHAT_TURN_STATUS_CONNECTING,
        }),
        frame(SSE_CHAT_STATUS, {
          type: SSE_CHAT_STATUS,
          status: "untrusted status",
        }),
      ]),
    );

    expect(events).toEqual([
      { kind: SSE_CHAT_STATUS, status: CHAT_TURN_STATUS_CONNECTING },
    ]);
  });
});
