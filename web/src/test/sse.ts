import {
  SSE_MESSAGE_START,
  SSE_CONTENT_BLOCK_START,
  SSE_CONTENT_BLOCK_DELTA,
  SSE_CONTENT_BLOCK_STOP,
  BLOCK_TEXT,
  BLOCK_TEXT_DELTA,
  BLOCK_THINKING,
  BLOCK_THINKING_DELTA,
  BLOCK_TOOL_USE,
  BLOCK_TOOL_RESULT,
  BLOCK_INPUT_JSON_DELTA,
  BLOCK_JSON_PARTIAL,
} from "$lib/api/sse-events";

// frame renders the canonical wire bytes for one event, matching the server's
// FrameLines (internal/pkg/sse/sink.go): `event: <name>\ndata: <json>\n\n`.
export function frame(event: string, data: unknown): string {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
}

// messageStart frames the message_start event that opens a stream.
export function messageStart(conversationId: string, model: string): string {
  return frame(SSE_MESSAGE_START, {
    type: SSE_MESSAGE_START,
    message: { id: "msg_1", stream_id: conversationId, model },
  });
}

// contentBlock frames a text-shaped block (text or thinking): a start, one
// delta per part, and a stop.
function contentBlock(
  index: number,
  blockType: string,
  startExtra: Record<string, unknown>,
  deltaType: string,
  deltaField: string,
  parts: string[],
): string[] {
  const out = [
    frame(SSE_CONTENT_BLOCK_START, {
      type: SSE_CONTENT_BLOCK_START,
      index,
      content_block: { type: blockType, text: "", ...startExtra },
    }),
  ];
  for (const p of parts) {
    out.push(
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index,
        delta: { type: deltaType, [deltaField]: p },
      }),
    );
  }
  out.push(
    frame(SSE_CONTENT_BLOCK_STOP, { type: SSE_CONTENT_BLOCK_STOP, index }),
  );

  return out;
}

export function textBlock(index: number, ...parts: string[]): string[] {
  return contentBlock(index, BLOCK_TEXT, {}, BLOCK_TEXT_DELTA, "text", parts);
}

export function thinkingBlock(index: number, ...parts: string[]): string[] {
  return contentBlock(
    index,
    BLOCK_THINKING,
    {},
    BLOCK_THINKING_DELTA,
    "text",
    parts,
  );
}

// toolBlocks emits a tool_use block at `index` followed by its tool_result at
// `index + 1`, matched by tool_use_id.
export function toolBlocks(
  index: number,
  id: string,
  name: string,
  args: string,
  result: string,
  isError = false,
): string[] {
  return [
    frame(SSE_CONTENT_BLOCK_START, {
      type: SSE_CONTENT_BLOCK_START,
      index,
      content_block: { type: BLOCK_TOOL_USE, id, name },
    }),
    frame(SSE_CONTENT_BLOCK_DELTA, {
      type: SSE_CONTENT_BLOCK_DELTA,
      index,
      delta: { type: BLOCK_INPUT_JSON_DELTA, partial_json: args },
    }),
    frame(SSE_CONTENT_BLOCK_STOP, { type: SSE_CONTENT_BLOCK_STOP, index }),
    frame(SSE_CONTENT_BLOCK_START, {
      type: SSE_CONTENT_BLOCK_START,
      index: index + 1,
      content_block: {
        type: BLOCK_TOOL_RESULT,
        tool_use_id: id,
        is_error: isError,
      },
    }),
    frame(SSE_CONTENT_BLOCK_DELTA, {
      type: SSE_CONTENT_BLOCK_DELTA,
      index: index + 1,
      delta: { type: BLOCK_JSON_PARTIAL, text: result },
    }),
    frame(SSE_CONTENT_BLOCK_STOP, {
      type: SSE_CONTENT_BLOCK_STOP,
      index: index + 1,
    }),
  ];
}
