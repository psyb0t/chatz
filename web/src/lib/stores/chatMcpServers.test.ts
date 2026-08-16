import { describe, it, expect, beforeEach, vi } from "vitest";
import type { ChatMCPServer } from "$lib/api/client";
import {
  MCP_STATUS_CONNECTED,
  MCP_STATUS_CONNECTING,
  MCP_STATUS_FAILED,
  MCP_STATUS_DISABLED,
} from "$lib/common/mcp";

// Mock the API client so the store exercises its load/refresh/setEnabled logic
// against controllable fakes — no network. vi.mock is hoisted, so the spies are
// declared with vi.hoisted to be in scope here.
const { listChatMCPServersMock, updateChatMCPServerMock } = vi.hoisted(() => ({
  listChatMCPServersMock: vi.fn(),
  updateChatMCPServerMock: vi.fn(),
}));

vi.mock("$lib/api/client", () => ({
  listChatMCPServers: listChatMCPServersMock,
  updateChatMCPServer: updateChatMCPServerMock,
}));

// Import AFTER the mock is registered so the store binds to the fakes.
const { chatMcpServers } = await import("./chatMcpServers.svelte");

function server(
  id: string,
  name: string,
  status: ChatMCPServer["status"],
  enabled: boolean,
): ChatMCPServer {
  return { id, name, status, enabled };
}

const CHAT_ID = "chat-1";

describe("ChatMcpServersStore", () => {
  beforeEach(() => {
    listChatMCPServersMock.mockReset();
    updateChatMCPServerMock.mockReset();
    chatMcpServers.reset();
  });

  it("load populates the list and records the chat it belongs to", async () => {
    listChatMCPServersMock.mockResolvedValue([
      server("1", "alpha", MCP_STATUS_CONNECTED, true),
      server("2", "beta", MCP_STATUS_DISABLED, false),
    ]);

    await chatMcpServers.load(CHAT_ID);

    expect(listChatMCPServersMock).toHaveBeenCalledWith(CHAT_ID);
    expect(chatMcpServers.loaded).toBe(true);
    expect(chatMcpServers.chatId).toBe(CHAT_ID);
    expect(chatMcpServers.list).toHaveLength(2);
    expect(chatMcpServers.enabledCount).toBe(1);
    expect(chatMcpServers.loading).toBe(false);
    expect(chatMcpServers.error).toBeNull();
  });

  it("load handles an empty list without erroring", async () => {
    listChatMCPServersMock.mockResolvedValue([]);

    await chatMcpServers.load(CHAT_ID);

    expect(chatMcpServers.loaded).toBe(true);
    expect(chatMcpServers.list).toEqual([]);
    expect(chatMcpServers.error).toBeNull();
  });

  it("load swallows errors, surfaces the message, leaves loaded false", async () => {
    listChatMCPServersMock.mockRejectedValue(new Error("boom"));

    await chatMcpServers.load(CHAT_ID);

    expect(chatMcpServers.loaded).toBe(false);
    expect(chatMcpServers.list).toEqual([]);
    expect(chatMcpServers.loading).toBe(false);
    expect(chatMcpServers.error).toBe("boom");
  });

  it("setEnabled sends {enabled} and replaces the matching row in place", async () => {
    listChatMCPServersMock.mockResolvedValue([
      server("1", "alpha", MCP_STATUS_CONNECTED, true),
      server("2", "beta", MCP_STATUS_CONNECTED, true),
    ]);
    await chatMcpServers.load(CHAT_ID);

    updateChatMCPServerMock.mockResolvedValue(
      server("2", "beta", MCP_STATUS_CONNECTED, false),
    );

    await chatMcpServers.setEnabled("2", false);

    expect(updateChatMCPServerMock).toHaveBeenCalledWith(CHAT_ID, "2", {
      enabled: false,
    });
    expect(chatMcpServers.list[0].enabled).toBe(true);
    expect(chatMcpServers.list[1].enabled).toBe(false);
    expect(chatMcpServers.enabledCount).toBe(1);
  });

  it("anySettling is true while a server is connecting or failed", async () => {
    listChatMCPServersMock.mockResolvedValue([
      server("1", "alpha", MCP_STATUS_CONNECTED, true),
      server("2", "beta", MCP_STATUS_CONNECTING, true),
    ]);
    await chatMcpServers.load(CHAT_ID);
    expect(chatMcpServers.anySettling).toBe(true);

    chatMcpServers.list = [server("1", "alpha", MCP_STATUS_FAILED, true)];
    expect(chatMcpServers.anySettling).toBe(true);

    chatMcpServers.list = [
      server("1", "alpha", MCP_STATUS_CONNECTED, true),
      server("2", "beta", MCP_STATUS_DISABLED, false),
    ];
    expect(chatMcpServers.anySettling).toBe(false);
  });
});
