import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  BLOCK_KIND_TEXT,
  ROLE_USER,
  type ConvMessage,
  conversation,
} from "$lib/stores/conversation.svelte";
import { CHAT_TURN_STATUS_WAITING_FOR_FIRST_TOKEN } from "$lib/common/turn-status";
import { models } from "$lib/stores/models.svelte";
import ChatPane from "./ChatPane.svelte";

const { listChatMCPServersMock, previewChatContextMock } = vi.hoisted(() => ({
  listChatMCPServersMock: vi.fn(),
  previewChatContextMock: vi.fn(),
}));

vi.mock("$lib/api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("$lib/api/client")>()),
  listChatMCPServers: listChatMCPServersMock,
  previewChatContext: previewChatContextMock,
}));

const TEST_MESSAGE: ConvMessage = {
  id: "message-1",
  role: ROLE_USER,
  blocks: [{ id: "block-1", kind: BLOCK_KIND_TEXT, text: "Hello." }],
  streaming: false,
  incomplete: false,
};

describe("ChatPane", () => {
  beforeEach(() => {
    conversation.reset();
    conversation.chatId = "chat-1";
    conversation.messages = [TEST_MESSAGE];
    listChatMCPServersMock.mockResolvedValue([]);
    previewChatContextMock.mockResolvedValue({
      budgetTokens: 100_000,
      systemTokens: 1_000,
      historyTokens: 1,
      currentMessageTokens: 0,
      totalTokens: 1_001,
      availableTokens: 98_999,
      retainedMessages: 1,
      retainedTurns: 1,
      omittedMessages: 0,
      omittedTurns: 0,
    });
    models.list = [
      {
        id: "default-model",
        alias: "Default model",
        upstream: "gateway",
        availability: "available",
        default: true,
      },
    ];
  });

  afterEach(() => {
    conversation.reset();
    models.list = [];
    listChatMCPServersMock.mockReset();
    previewChatContextMock.mockReset();
  });

  it("keeps a newly reported turn status in view", async () => {
    render(ChatPane);

    const list = screen.getByRole("log");
    Object.defineProperties(list, {
      clientHeight: { configurable: true, value: 100 },
      scrollHeight: { configurable: true, value: 500 },
    });
    list.scrollTop = 0;
    conversation.turnStatus = CHAT_TURN_STATUS_WAITING_FOR_FIRST_TOKEN;

    await waitFor(() => {
      expect(list.scrollTop).toBe(500);
    });
  });

  it("returns focus to the composer after a turn when chat remains active", async () => {
    render(ChatPane);

    const list = screen.getByRole("log");
    const input = screen.getByLabelText("Message");
    conversation.streaming = true;
    await fireEvent.pointerDown(list);
    conversation.streaming = false;

    await waitFor(() => {
      expect(document.activeElement).toBe(input);
    });
  });

  it("does not steal focus after the user moves to another panel", async () => {
    render(ChatPane);

    const outside = document.createElement("button");
    document.body.append(outside);
    conversation.streaming = true;
    await fireEvent.pointerDown(outside);
    outside.focus();
    conversation.streaming = false;

    await Promise.resolve();

    expect(document.activeElement).toBe(outside);
    outside.remove();
  });
});
