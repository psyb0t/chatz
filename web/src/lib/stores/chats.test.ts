import { describe, it, expect, beforeEach, vi } from "vitest";
import type { Chat, ChatSummary, ChatList } from "$lib/api/client";
// The vitest config aliases $app/navigation to this same mock module, so the
// store's goto() calls land in this array. Imported by relative path (not the
// $app alias) so svelte-check resolves the real SvelteKit types elsewhere.
import { gotoCalls } from "../../test/app-mocks/navigation";
// Same aliasing story as navigation: the store reads page.params.chatId to know
// which chat is on screen, so these tests drive it through the mock. Aliased
// because this file already has a local page() helper building a ChatList.
import { page as routePage } from "../../test/app-mocks/state";

// Mock the API client so the store exercises its load/touch/rename/delete/
// goToNewChat logic against controllable fakes — no network, no real
// openapi-fetch. vi.mock is hoisted, so the spies are declared with
// vi.hoisted to be in scope here.
const {
  deleteChatMock,
  getOrCreateEmptyChatMock,
  listChatsMock,
  renameChatMock,
} = vi.hoisted(() => ({
  deleteChatMock: vi.fn(),
  getOrCreateEmptyChatMock: vi.fn(),
  listChatsMock: vi.fn(),
  renameChatMock: vi.fn(),
}));

vi.mock("$lib/api/client", () => ({
  deleteChat: deleteChatMock,
  getOrCreateEmptyChat: getOrCreateEmptyChatMock,
  listChats: listChatsMock,
  renameChat: renameChatMock,
}));

// Import AFTER the mock is registered so the store binds to the fakes.
const { chats } = await import("./chats.svelte");

function chatSummary(id: string, title: string): ChatSummary {
  return {
    id,
    title,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
}

function page(items: ChatSummary[], total: number): ChatList {
  return { items, limit: 100, offset: 0, total };
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

describe("ChatsStore", () => {
  beforeEach(() => {
    deleteChatMock.mockReset();
    listChatsMock.mockReset();
    renameChatMock.mockReset();
    getOrCreateEmptyChatMock.mockReset();
    gotoCalls.length = 0;
    routePage.params = {};
    chats.list = [];
    chats.loaded = false;
    chats.loading = false;
    chats.search = "";
    chats.error = null;
  });

  it("load unwraps the paginated envelope's items into the list", async () => {
    listChatsMock.mockResolvedValue(
      page([chatSummary("1", "hello"), chatSummary("2", "world")], 2),
    );

    await chats.load();

    expect(listChatsMock).toHaveBeenCalledWith({
      limit: 100,
      offset: 0,
      search: undefined,
    });
    expect(chats.loaded).toBe(true);
    expect(chats.list).toHaveLength(2);
    expect(chats.list[0].title).toBe("hello");
    expect(chats.loading).toBe(false);
    expect(chats.error).toBeNull();
  });

  it("load handles an empty page (items: [], total: 0) without erroring", async () => {
    listChatsMock.mockResolvedValue(page([], 0));

    await chats.load();

    expect(chats.loaded).toBe(true);
    expect(chats.list).toEqual([]);
    expect(chats.error).toBeNull();
  });

  it("load swallows errors, surfaces the message, and leaves loaded false", async () => {
    listChatsMock.mockRejectedValue(new Error("boom"));

    await chats.load();

    expect(chats.loaded).toBe(false);
    expect(chats.list).toEqual([]);
    expect(chats.loading).toBe(false);
    expect(chats.error).toBe("boom");
  });

  it("setSearch passes the query through to the list request", async () => {
    listChatsMock.mockResolvedValue(page([], 0));

    await chats.setSearch("needle");

    expect(listChatsMock).toHaveBeenLastCalledWith({
      limit: 100,
      offset: 0,
      search: "needle",
    });
  });

  it("rename updates the matching entry in place", async () => {
    listChatsMock.mockResolvedValue(page([chatSummary("1", "old")], 1));
    await chats.load();

    renameChatMock.mockResolvedValue({
      id: "1",
      title: "new",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z",
    });

    await chats.rename("1", "new");

    expect(renameChatMock).toHaveBeenCalledWith("1", "new");
    expect(chats.list[0].title).toBe("new");
    expect(chats.list[0].updatedAt).toBe("2026-01-02T00:00:00Z");
  });

  it("keeps the list in newest-activity order after a rename re-inserts a chat", async () => {
    chats.list = [
      { ...chatSummary("newer", "newer"), updatedAt: "2026-01-03T00:00:00Z" },
      { ...chatSummary("older", "older"), updatedAt: "2026-01-02T00:00:00Z" },
    ];
    renameChatMock.mockResolvedValue({
      ...chatSummary("older", "renamed"),
      updatedAt: "2026-01-04T00:00:00Z",
    });

    await chats.rename("older", "renamed");

    // The renamed chat carries the newest updatedAt, so it sorts to the head.
    expect(chats.list.map((item) => item.id)).toEqual(["older", "newer"]);
  });

  it("removes a deleted chat from the visible list", async () => {
    chats.list = [chatSummary("1", "delete"), chatSummary("2", "keep")];
    deleteChatMock.mockResolvedValue(undefined);

    await chats.delete("1");

    expect(deleteChatMock).toHaveBeenCalledWith("1");
    expect(chats.list.map((item) => item.id)).toEqual(["2"]);
  });

  it("keeps the latest response when a prior list returns late", async () => {
    const active = deferred<ChatList>();
    const searched = deferred<ChatList>();
    listChatsMock.mockReturnValueOnce(active.promise);
    listChatsMock.mockReturnValueOnce(searched.promise);

    const activeLoad = chats.load();
    const searchLoad = chats.setSearch("needle");
    searched.resolve(page([chatSummary("needle", "needle")], 1));
    await searchLoad;
    active.resolve(page([chatSummary("stale", "stale")], 1));
    await activeLoad;

    expect(chats.list.map((item) => item.id)).toEqual(["needle"]);
  });

  it("touch adds a new chat at the head of the list", () => {
    chats.list = [chatSummary("existing", "already there")];

    chats.touch("new-chat", "fresh title");

    expect(chats.list.map((c) => c.id)).toEqual(["new-chat", "existing"]);
    expect(chats.list[0].title).toBe("fresh title");
  });

  it("touch moves an already-listed chat to the head without changing its title", () => {
    chats.list = [chatSummary("a", "a title"), chatSummary("b", "b title")];

    chats.touch("b");

    expect(chats.list.map((c) => c.id)).toEqual(["b", "a"]);
    expect(chats.list[0].title).toBe("b title");
  });

  it("touch overwrites the title of an already-listed chat when one is passed", () => {
    chats.list = [chatSummary("a", "old title")];

    chats.touch("a", "new title");

    expect(chats.list[0].title).toBe("new title");
  });

  it("touch drops a chat that no longer matches the active search", () => {
    chats.search = "keep";
    chats.list = [chatSummary("keep-me", "keep this one")];

    chats.touch("other", "unrelated title");

    expect(chats.list.map((c) => c.id)).toEqual(["keep-me"]);
  });

  it("goToNewChat resolves the empty chat and navigates to it", async () => {
    getOrCreateEmptyChatMock.mockResolvedValue({
      id: "empty-1",
      title: "",
      model: "",
    } satisfies Chat);

    await chats.goToNewChat();

    expect(getOrCreateEmptyChatMock).toHaveBeenCalled();
    expect(gotoCalls).toEqual(["/chat/empty-1"]);
    expect(chats.error).toBeNull();
  });

  it("goToNewChat surfaces the error and does not navigate on failure", async () => {
    getOrCreateEmptyChatMock.mockRejectedValue(new Error("boom"));

    await chats.goToNewChat();

    expect(gotoCalls).toEqual([]);
    expect(chats.error).toBe("boom");
  });

  it("delete navigates away when the deleted chat is the one on screen", async () => {
    routePage.params = { chatId: "doomed" };
    chats.list = [chatSummary("doomed", "bye"), chatSummary("other", "stay")];
    deleteChatMock.mockResolvedValue(undefined);
    getOrCreateEmptyChatMock.mockResolvedValue({
      id: "empty-1",
      title: "",
      model: "",
    } satisfies Chat);

    await chats.delete("doomed");

    expect(chats.list.map((chat) => chat.id)).toEqual(["other"]);
    expect(gotoCalls).toEqual(["/chat/empty-1"]);
  });

  it("delete stays put when a different chat is on screen", async () => {
    routePage.params = { chatId: "other" };
    chats.list = [chatSummary("doomed", "bye"), chatSummary("other", "stay")];
    deleteChatMock.mockResolvedValue(undefined);

    await chats.delete("doomed");

    expect(chats.list.map((chat) => chat.id)).toEqual(["other"]);
    expect(getOrCreateEmptyChatMock).not.toHaveBeenCalled();
    expect(gotoCalls).toEqual([]);
  });
});
