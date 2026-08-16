import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { Chat, MessageList } from "$lib/api/client";
import {
  conversation,
  messageText,
  ROLE_USER,
  ROLE_ASSISTANT,
  BLOCK_KIND_TEXT,
  BLOCK_KIND_THINKING,
  BLOCK_KIND_TOOL,
  type ConvMessage,
  type ToolCall,
} from "./conversation.svelte";
import { chats } from "./chats.svelte";
// The vitest config aliases $app/navigation to this same mock module, so the
// store's goto() calls land in this array. Imported by relative path (not the
// $app alias) so svelte-check resolves the real SvelteKit types elsewhere.
import { gotoCalls } from "../../test/app-mocks/navigation";
import {
  SSE_CHAT_STATUS,
  SSE_CONTENT_BLOCK_START,
  SSE_CONTENT_BLOCK_DELTA,
  SSE_MESSAGE_STOP,
  SSE_ERROR,
  BLOCK_TEXT,
  BLOCK_TEXT_DELTA,
} from "$lib/api/sse-events";
import { CHAT_TURN_STATUS_CONNECTING } from "$lib/common/turn-status";
import {
  frame,
  messageStart as sseMessageStart,
  textBlock,
  thinkingBlock,
  toolBlocks,
} from "../../test/sse";

// getChat/listChatMessages are load()'s dependencies — mocked so the new
// load() tests below don't hit the network. updateChatSettings/listChats/
// renameChat are mocked too since they live in the same module (vi.mock
// replaces the whole module); the SSE-path tests above never call them.
const {
  getChatMock,
  listChatMessagesMock,
  updateChatSettingsMock,
  listChatsMock,
  renameChatMock,
} = vi.hoisted(() => ({
  getChatMock: vi.fn(),
  listChatMessagesMock: vi.fn(),
  updateChatSettingsMock: vi.fn(),
  listChatsMock: vi.fn(),
  renameChatMock: vi.fn(),
}));

vi.mock("$lib/api/client", () => ({
  getChat: getChatMock,
  listChatMessages: listChatMessagesMock,
  updateChatSettings: updateChatSettingsMock,
  listChats: listChatsMock,
  renameChat: renameChatMock,
}));

const MODEL = "gpt-test";
// send() now always continues an already-active chat (routing resolves a
// real chat id before the composer can call it — see chats.goToNewChat), so
// every SSE-assembly test below presets conversation.chatId in beforeEach.
const TEST_CHAT_ID = "chat-1";

// Thin wrapper closing the shared sseMessageStart builder over this file's
// test model, so call sites below read the same as before extraction.
function messageStart(conversationId: string): string {
  return sseMessageStart(conversationId, MODEL);
}

// mockStreamResponse installs a fetch stub that returns an ok text/event-stream
// whose body emits the given frames. Feeds the whole create/continue → decode →
// assemble chain deterministically, no network.
function mockStreamResponse(frames: string[]): void {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const f of frames) {
        controller.enqueue(encoder.encode(f));
      }
      controller.close();
    },
  });

  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(new Response(body, { status: 200 }))),
  );
}

function deferred<T>(): {
  promise: Promise<T>;
  resolve: (value: T) => void;
} {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });

  return { promise, resolve };
}

function assistantMessage(): ConvMessage {
  const found = conversation.messages.find((m) => m.role === ROLE_ASSISTANT);
  if (found === undefined) {
    throw new Error("no assistant message");
  }

  return found;
}

function toolCalls(message: ConvMessage): ToolCall[] {
  return message.blocks
    .map((block) => (block.kind === BLOCK_KIND_TOOL ? block.call : null))
    .filter((call): call is ToolCall => call !== null);
}

describe("conversation store — SSE assembly", () => {
  beforeEach(() => {
    conversation.reset();
    conversation.chatId = TEST_CHAT_ID;
    chats.list = [];
    gotoCalls.length = 0;
    getChatMock.mockReset();
    listChatMessagesMock.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("assembles a user + streamed assistant text message", async () => {
    mockStreamResponse([
      messageStart(TEST_CHAT_ID),
      ...textBlock(0, "There are ", "3 users."),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);

    await conversation.send("how many users?", MODEL);

    expect(conversation.chatId).toBe(TEST_CHAT_ID);
    expect(conversation.streaming).toBe(false);
    expect(conversation.error).toBeNull();

    const roles = conversation.messages.map((m) => m.role);
    expect(roles).toEqual([ROLE_USER, ROLE_ASSISTANT]);

    const [user, assistant] = conversation.messages;
    expect(messageText(user)).toBe("how many users?");
    expect(messageText(assistant)).toBe("There are 3 users.");
    expect(assistant.streaming).toBe(false);
    expect(toolCalls(assistant)).toEqual([]);
  });

  it("uses the latest status while streaming and clears it at the terminal event", async () => {
    const encoder = new TextEncoder();
    let controller!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({
      start(next) {
        controller = next;
      },
    });

    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.resolve(new Response(body, { status: 200 }))),
    );

    const sent = conversation.send("status", MODEL);
    await new Promise((resolve) => setTimeout(resolve, 0));
    controller.enqueue(
      encoder.encode(
        frame(SSE_CHAT_STATUS, {
          type: SSE_CHAT_STATUS,
          status: CHAT_TURN_STATUS_CONNECTING,
        }),
      ),
    );
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(conversation.turnStatus).toBe(CHAT_TURN_STATUS_CONNECTING);

    controller.enqueue(
      encoder.encode(frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP })),
    );
    controller.close();
    await sent;

    expect(conversation.turnStatus).toBeNull();
  });

  it("keeps the live turn when an initial empty-history request finishes late", async () => {
    const chatLoad = deferred<Chat>();
    const messagesLoad = deferred<MessageList>();
    getChatMock.mockReturnValue(chatLoad.promise);
    listChatMessagesMock.mockReturnValue(messagesLoad.promise);

    const load = conversation.load(TEST_CHAT_ID);
    mockStreamResponse([
      messageStart(TEST_CHAT_ID),
      ...textBlock(0, "streamed reply"),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);

    await conversation.send("first message", MODEL);
    chatLoad.resolve(fakeChat());
    messagesLoad.resolve(fakeMessageList());
    await load;

    expect(conversation.messages.map((m) => m.role)).toEqual([
      ROLE_USER,
      ROLE_ASSISTANT,
    ]);
    expect(messageText(conversation.messages[0])).toBe("first message");
    expect(messageText(conversation.messages[1])).toBe("streamed reply");
    expect(conversation.loading).toBe(false);
  });

  it("does nothing when called with no active chat (routing invariant guard)", async () => {
    conversation.chatId = null;
    mockStreamResponse([
      messageStart(TEST_CHAT_ID),
      ...textBlock(0, "unreachable"),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);

    await conversation.send("hey", MODEL);

    expect(conversation.messages).toEqual([]);
    expect(conversation.streaming).toBe(false);
  });

  it("bumps the active chat to the top of the sidebar, titling it only on its first message", async () => {
    chats.list = [
      { id: "other-chat", title: "older", createdAt: "t", updatedAt: "t" },
    ];

    mockStreamResponse([
      messageStart(TEST_CHAT_ID),
      ...textBlock(0, "hi"),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);
    await conversation.send("hey", MODEL);

    expect(gotoCalls).toEqual([]);
    expect(chats.list.map((c) => c.id)).toEqual([TEST_CHAT_ID, "other-chat"]);
    expect(chats.list[0].title).toBe("hey");

    mockStreamResponse([
      messageStart(TEST_CHAT_ID),
      ...textBlock(0, "second reply"),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);
    await conversation.send("second message", MODEL);

    expect(chats.list[0].id).toBe(TEST_CHAT_ID);
    expect(chats.list[0].title).toBe("hey");
  });

  it("attaches a tool result to its call by tool_use_id", async () => {
    mockStreamResponse([
      messageStart(TEST_CHAT_ID),
      ...toolBlocks(0, "tu-9", "list_users", '{"limit":5}', "3 users"),
      ...textBlock(2, "There are 3 users."),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);

    await conversation.send("count users", MODEL);

    const assistant = assistantMessage();
    expect(messageText(assistant)).toBe("There are 3 users.");

    const calls = toolCalls(assistant);
    expect(calls).toHaveLength(1);
    expect(calls[0].name).toBe("list_users");
    expect(calls[0].args).toBe('{"limit":5}');
    expect(calls[0].result).toBe("3 users");
    expect(calls[0].isError).toBe(false);
    expect(calls[0].done).toBe(true);
  });

  // The headline behavior: blocks render in stream-arrival order, NOT clumped
  // (all tool cards on top, all text below). A thinking → text → tool → text
  // stream must produce blocks in exactly that order.
  it("keeps blocks in arrival order (thinking, text, tool, text)", async () => {
    mockStreamResponse([
      messageStart("chat-interleave-1"),
      ...thinkingBlock(0, "Let me check ", "the users."),
      ...textBlock(1, "Looking that up.\n"),
      ...toolBlocks(2, "tu-1", "list_users", "{}", "3 users"),
      ...textBlock(4, "There are 3 users."),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);

    await conversation.send("count users", MODEL);

    const assistant = assistantMessage();
    expect(assistant.blocks.map((b) => b.kind)).toEqual([
      BLOCK_KIND_THINKING,
      BLOCK_KIND_TEXT,
      BLOCK_KIND_TOOL,
      BLOCK_KIND_TEXT,
    ]);

    const thinking = assistant.blocks[0];
    expect(thinking.kind === BLOCK_KIND_THINKING ? thinking.text : "").toBe(
      "Let me check the users.",
    );
    expect(toolCalls(assistant)[0].result).toBe("3 users");
  });

  it("marks a tool card as errored when tool_result.is_error is set", async () => {
    mockStreamResponse([
      messageStart("chat-tool-err"),
      ...toolBlocks(0, "tu-x", "flaky", "{}", "boom", true),
      ...textBlock(2, "that failed"),
      frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
    ]);

    await conversation.send("try it", MODEL);

    const call = toolCalls(assistantMessage())[0];
    expect(call.isError).toBe(true);
    expect(call.result).toBe("boom");
    expect(call.done).toBe(true);
  });

  // Regression guard for the frozen-render bug: the store once mutated a raw
  // (pre-$state-proxy) reference to the streaming assistant message, so the
  // template's proxy signals never saw the deltas — the UI froze at empty text
  // + a stuck caret while the store's own state advanced. The reactivity
  // contract is per-delta IMMUTABLE REPLACEMENT: after each delta, the
  // assistant entry in `messages` must be a NEW object reference carrying the
  // accumulated text.
  it("replaces the assistant message object on every delta (never mutates)", async () => {
    const encoder = new TextEncoder();
    let controller!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({
      start(c) {
        controller = c;
      },
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.resolve(new Response(body, { status: 200 }))),
    );

    const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
    const enqueue = (f: string) => controller.enqueue(encoder.encode(f));

    const done = conversation.send("stream me", MODEL);
    await settle();

    enqueue(messageStart("chat-ref-1"));
    enqueue(
      frame(SSE_CONTENT_BLOCK_START, {
        type: SSE_CONTENT_BLOCK_START,
        index: 0,
        content_block: { type: BLOCK_TEXT, text: "" },
      }),
    );
    enqueue(
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index: 0,
        delta: { type: BLOCK_TEXT_DELTA, text: "first " },
      }),
    );
    await settle();

    const afterFirst = conversation.messages.find(
      (m) => m.role === ROLE_ASSISTANT,
    );
    expect(afterFirst && messageText(afterFirst)).toBe("first ");

    enqueue(
      frame(SSE_CONTENT_BLOCK_DELTA, {
        type: SSE_CONTENT_BLOCK_DELTA,
        index: 0,
        delta: { type: BLOCK_TEXT_DELTA, text: "second" },
      }),
    );
    await settle();

    const afterSecond = conversation.messages.find(
      (m) => m.role === ROLE_ASSISTANT,
    );
    expect(afterSecond && messageText(afterSecond)).toBe("first second");
    expect(afterSecond).not.toBe(afterFirst);

    enqueue(frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }));
    controller.close();
    await done;

    const final = conversation.messages.find((m) => m.role === ROLE_ASSISTANT);
    expect(final?.streaming).toBe(false);
    expect(final).not.toBe(afterSecond);
  });

  it("surfaces a stream error and leaves streaming false", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.resolve(new Response(null, { status: 500 }))),
    );

    await conversation.send("boom", MODEL);

    expect(conversation.streaming).toBe(false);
    expect(conversation.error).not.toBeNull();
    expect(conversation.retryAvailable).toBe(true);
    // The optimistic user message stays so the input isn't lost.
    expect(conversation.messages.map((m) => m.role)).toContain(ROLE_USER);
  });

  it("surfaces a terminal stream error without dropping partial text", async () => {
    mockStreamResponse([
      frame(SSE_CHAT_STATUS, {
        type: SSE_CHAT_STATUS,
        status: CHAT_TURN_STATUS_CONNECTING,
      }),
      messageStart(TEST_CHAT_ID),
      ...textBlock(0, "I found the ", "first result."),
      frame(SSE_ERROR, {
        type: SSE_ERROR,
        error: {
          type: "upstream_timeout",
          message: "The model did not respond in time. Try again.",
        },
      }),
    ]);

    await conversation.send("check it", MODEL);

    expect(conversation.streaming).toBe(false);
    expect(conversation.error).toBe(
      "The model did not respond in time. Try again.",
    );
    expect(conversation.turnStatus).toBeNull();
    expect(conversation.retryAvailable).toBe(true);
    expect(messageText(assistantMessage())).toBe("I found the first result.");
  });

  it("retries the failed prompt with its original model", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(new Response(null, { status: 500 }))
        .mockResolvedValueOnce(
          new Response(
            new ReadableStream<Uint8Array>({
              start(controller) {
                const encoder = new TextEncoder();
                controller.enqueue(encoder.encode(messageStart(TEST_CHAT_ID)));
                controller.enqueue(
                  encoder.encode(
                    frame(SSE_MESSAGE_STOP, { type: SSE_MESSAGE_STOP }),
                  ),
                );
                controller.close();
              },
            }),
            { status: 200 },
          ),
        ),
    );

    await conversation.send("try again", MODEL);
    await conversation.retryLastTurn();

    expect(conversation.retryAvailable).toBe(false);
    expect(
      conversation.messages
        .filter((message) => message.role === ROLE_USER)
        .map(messageText),
    ).toEqual(["try again", "try again"]);
  });

  it("clears the live status immediately when the user stops a turn", async () => {
    const encoder = new TextEncoder();
    let controller!: ReadableStreamDefaultController<Uint8Array>;
    const body = new ReadableStream<Uint8Array>({
      start(next) {
        controller = next;
      },
    });

    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.resolve(new Response(body, { status: 200 }))),
    );

    const sent = conversation.send("stop status", MODEL);
    await new Promise((resolve) => setTimeout(resolve, 0));
    controller.enqueue(
      encoder.encode(
        frame(SSE_CHAT_STATUS, {
          type: SSE_CHAT_STATUS,
          status: CHAT_TURN_STATUS_CONNECTING,
        }),
      ),
    );
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(conversation.turnStatus).toBe(CHAT_TURN_STATUS_CONNECTING);

    conversation.stop();
    expect(conversation.turnStatus).toBeNull();

    controller.close();
    await sent;
  });
});

function fakeChat(overrides: Partial<Chat> = {}): Chat {
  return {
    id: "chat-1",
    title: "hi there",
    model: MODEL,
    ...overrides,
  };
}

function fakeMessageList(overrides: Partial<MessageList> = {}): MessageList {
  return {
    items: [],
    limit: 200,
    offset: 0,
    total: 0,
    ...overrides,
  };
}

// The load() path fetches chat metadata and the message timeline as two
// separate calls (getChat + listChatMessages) since messages became their own
// paginated sub-resource — this replaces the old single getChat().messages
// read.
describe("conversation store — load() (metadata + messages split)", () => {
  beforeEach(() => {
    conversation.reset();
    getChatMock.mockReset();
    listChatMessagesMock.mockReset();
  });

  it("hydrates metadata from getChat and the timeline from listChatMessages", async () => {
    getChatMock.mockResolvedValue(fakeChat({ settings: { temperature: 0.5 } }));
    listChatMessagesMock.mockResolvedValue(
      fakeMessageList({
        items: [
          {
            id: "m1",
            role: "user",
            content: "hi there",
            createdAt: "2026-01-01T00:00:00Z",
          },
          {
            id: "m2",
            role: "assistant",
            content: "hello!",
            createdAt: "2026-01-01T00:00:01Z",
          },
        ],
        total: 2,
      }),
    );

    await conversation.load("chat-1");

    expect(getChatMock).toHaveBeenCalledWith("chat-1");
    expect(listChatMessagesMock).toHaveBeenCalledWith("chat-1", {
      limit: 200,
      offset: 0,
    });

    expect(conversation.model).toBe(MODEL);
    expect(conversation.settings).toEqual({ temperature: 0.5 });
    expect(conversation.messages.map((m) => m.role)).toEqual([
      ROLE_USER,
      ROLE_ASSISTANT,
    ]);
    expect(messageText(conversation.messages[0])).toBe("hi there");
    expect(messageText(conversation.messages[1])).toBe("hello!");
    expect(conversation.loading).toBe(false);
    expect(conversation.error).toBeNull();
  });

  it("defaults settings to {} when getChat omits it", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(fakeMessageList());

    await conversation.load("chat-1");

    expect(conversation.settings).toEqual({});
  });

  it("surfaces an error and clears loading when getChat rejects", async () => {
    getChatMock.mockRejectedValue(new Error("chat gone"));
    listChatMessagesMock.mockResolvedValue(fakeMessageList());

    await conversation.load("chat-1");

    expect(conversation.error).toBe("chat gone");
    expect(conversation.loading).toBe(false);
    expect(conversation.messages).toEqual([]);
  });

  it("surfaces an error and clears loading when listChatMessages rejects", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockRejectedValue(new Error("messages gone"));

    await conversation.load("chat-1");

    expect(conversation.error).toBe("messages gone");
    expect(conversation.loading).toBe(false);
  });

  // Regression: a reload used to flatten every stored row into its own
  // single-text-block ConvMessage, so a turn that streamed thinking + a tool
  // call + a final answer came back as 3 separate bubbles instead of the ONE
  // turn the user actually saw live. groupApiMessages folds consecutive
  // assistant/tool rows between user rows into a single ConvMessage whose
  // blocks match the live arrival order.
  it("folds a multi-round turn (thinking, tool call, tool result, final text) into one assistant message", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(
      fakeMessageList({
        items: [
          { id: "m1", role: "user", content: "search for X", createdAt: "" },
          {
            id: "m2",
            role: "assistant",
            content: "",
            thinking: "Let me search first",
            toolCalls: [
              { id: "call_1", name: "search", arguments: '{"q":"X"}' },
            ],
            createdAt: "",
          },
          {
            id: "m3",
            role: "tool",
            content: "search failed: timeout",
            toolCallId: "call_1",
            isError: true,
            createdAt: "",
          },
          {
            id: "m4",
            role: "assistant",
            content: "Here's what I found",
            createdAt: "",
          },
        ],
        total: 4,
      }),
    );

    await conversation.load("chat-1");

    expect(conversation.messages.map((m) => m.role)).toEqual([
      ROLE_USER,
      ROLE_ASSISTANT,
    ]);

    const turn = conversation.messages[1];
    expect(turn.blocks.map((b) => b.kind)).toEqual([
      BLOCK_KIND_THINKING,
      BLOCK_KIND_TOOL,
      BLOCK_KIND_TEXT,
    ]);

    const thinking = turn.blocks[0];
    if (thinking.kind === BLOCK_KIND_THINKING) {
      expect(thinking.text).toBe("Let me search first");
    }

    const tool = turn.blocks[1];
    if (tool.kind === BLOCK_KIND_TOOL) {
      expect(tool.call).toEqual({
        toolUseId: "call_1",
        name: "search",
        args: '{"q":"X"}',
        result: "search failed: timeout",
        isError: true,
        done: true,
      });
    }

    const text = turn.blocks[2];
    if (text.kind === BLOCK_KIND_TEXT) {
      expect(text.text).toBe("Here's what I found");
    }
  });

  it("keeps separate turns as separate assistant messages, not one merged blob", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(
      fakeMessageList({
        items: [
          { id: "m1", role: "user", content: "first", createdAt: "" },
          {
            id: "m2",
            role: "assistant",
            content: "first answer",
            createdAt: "",
          },
          { id: "m3", role: "user", content: "second", createdAt: "" },
          {
            id: "m4",
            role: "assistant",
            content: "second answer",
            createdAt: "",
          },
        ],
        total: 4,
      }),
    );

    await conversation.load("chat-1");

    expect(conversation.messages.map((m) => m.role)).toEqual([
      ROLE_USER,
      ROLE_ASSISTANT,
      ROLE_USER,
      ROLE_ASSISTANT,
    ]);
    expect(messageText(conversation.messages[1])).toBe("first answer");
    expect(messageText(conversation.messages[3])).toBe("second answer");
  });

  it("keeps a recovered assistant checkpoint and marks it incomplete", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(
      fakeMessageList({
        items: [
          { id: "m1", role: "user", content: "first", createdAt: "" },
          {
            id: "m2",
            role: "assistant",
            content: "partial answer",
            incomplete: true,
            createdAt: "",
          },
        ],
        total: 2,
      }),
    );

    await conversation.load("chat-1");

    const recovered = conversation.messages[1];
    expect(messageText(recovered)).toBe("partial answer");
    expect(recovered.incomplete).toBe(true);
    expect(recovered.streaming).toBe(false);
  });

  // Edge case the happy-path fold test above doesn't cover: executeTools runs
  // requested calls in parallel (runToolsParallel), so a single assistant row
  // routinely carries MORE THAN ONE tool call, each answered by its own
  // later tool-role row. Each result must attach to the matching ToolBlock by
  // toolCallId, not just "the next tool block in the list".
  it("attaches each of several parallel tool calls to its own result by toolCallId", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(
      fakeMessageList({
        items: [
          { id: "m1", role: "user", content: "check both", createdAt: "" },
          {
            id: "m2",
            role: "assistant",
            content: "",
            toolCalls: [
              { id: "call_a", name: "weather", arguments: '{"city":"a"}' },
              { id: "call_b", name: "weather", arguments: '{"city":"b"}' },
            ],
            createdAt: "",
          },
          // Results arrive out of call order — attachment must key on
          // toolCallId, not array position.
          {
            id: "m3",
            role: "tool",
            content: "b: sunny",
            toolCallId: "call_b",
            createdAt: "",
          },
          {
            id: "m4",
            role: "tool",
            content: "a: rainy",
            toolCallId: "call_a",
            createdAt: "",
          },
        ],
        total: 4,
      }),
    );

    await conversation.load("chat-1");

    const turn = conversation.messages[1];
    const calls = turn.blocks
      .map((b) => (b.kind === BLOCK_KIND_TOOL ? b.call : null))
      .filter((c): c is ToolCall => c !== null);

    expect(calls).toHaveLength(2);
    const byId = Object.fromEntries(calls.map((c) => [c.toolUseId, c]));
    expect(byId.call_a).toMatchObject({ result: "a: rainy", done: true });
    expect(byId.call_b).toMatchObject({ result: "b: sunny", done: true });
  });

  it("returns no messages for an empty chat", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(fakeMessageList());

    await conversation.load("chat-1");

    expect(conversation.messages).toEqual([]);
  });

  // Edge case: a tool-role row whose toolCallId doesn't match any tool call
  // in the turn being folded (e.g. a page boundary cut a turn in half). Must
  // not throw and must not misattach the result to an unrelated tool block —
  // it's dropped, same as a live tool_result with no matching blockRef.
  it("drops a tool result that answers no call in the current turn, without crashing or misattaching it", async () => {
    getChatMock.mockResolvedValue(fakeChat());
    listChatMessagesMock.mockResolvedValue(
      fakeMessageList({
        items: [
          { id: "m1", role: "user", content: "check it", createdAt: "" },
          {
            id: "m2",
            role: "assistant",
            content: "",
            toolCalls: [{ id: "call_a", name: "check", arguments: "{}" }],
            createdAt: "",
          },
          {
            id: "m3",
            role: "tool",
            content: "orphaned",
            toolCallId: "call_nonexistent",
            createdAt: "",
          },
        ],
        total: 3,
      }),
    );

    await conversation.load("chat-1");

    const turn = conversation.messages[1];
    const calls = turn.blocks
      .map((b) => (b.kind === BLOCK_KIND_TOOL ? b.call : null))
      .filter((c): c is ToolCall => c !== null);

    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatchObject({
      toolUseId: "call_a",
      result: null,
      done: false,
    });
  });
});
