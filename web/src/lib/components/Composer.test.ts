import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TESTID_CONTEXT_METER } from "$lib/common/test-ids";
import { conversation } from "$lib/stores/conversation.svelte";
import { models } from "$lib/stores/models.svelte";
import Composer from "./Composer.svelte";

const { listChatMCPServersMock, previewChatContextMock } = vi.hoisted(() => ({
  listChatMCPServersMock: vi.fn(),
  previewChatContextMock: vi.fn(),
}));

vi.mock("$lib/api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("$lib/api/client")>()),
  listChatMCPServers: listChatMCPServersMock,
  previewChatContext: previewChatContextMock,
}));

describe("Composer", () => {
  beforeEach(() => {
    conversation.reset();
    conversation.chatId = "chat-1";
    conversation.streaming = false;
    listChatMCPServersMock.mockResolvedValue([]);
    models.list = [
      {
        id: "default-model",
        alias: "Default model",
        upstream: "gateway",
        availability: "available",
        default: true,
      },
    ];
    previewChatContextMock.mockResolvedValue({
      budgetTokens: 100_000,
      systemTokens: 1_000,
      historyTokens: 40_000,
      currentMessageTokens: 25,
      totalTokens: 41_025,
      availableTokens: 58_975,
      retainedMessages: 20,
      retainedTurns: 10,
      omittedMessages: 4,
      omittedTurns: 2,
    });
  });

  afterEach(() => {
    conversation.reset();
    models.list = [];
    listChatMCPServersMock.mockReset();
    previewChatContextMock.mockReset();
  });

  it("renders the backend preview for the active draft", async () => {
    render(Composer);

    await fireEvent.input(screen.getByLabelText("Message"), {
      target: { value: "Summarize the incident." },
    });

    await waitFor(() => {
      expect(previewChatContextMock).toHaveBeenLastCalledWith(
        "chat-1",
        "Summarize the incident.",
      );
    });

    const meter = await screen.findByTestId(TESTID_CONTEXT_METER);
    expect(meter).toHaveTextContent("41,025 / 100,000 tokens · 58,975 free");
    expect(meter).toHaveTextContent("system 1,000 · history 40,000 · draft 25");
    expect(meter).toHaveTextContent("omitted 2 earlier turns (4 messages)");
  });
});
