import { describe, it, expect, beforeEach, vi } from "vitest";
import type { Chat, ChatSummary, ChatList, Project } from "$lib/api/client";
// The vitest config aliases $app/navigation to this same mock module, so the
// store's goto() calls land in this array. Imported by relative path (not the
// $app alias) so svelte-check resolves the real SvelteKit types elsewhere.
import { gotoCalls } from "../../test/app-mocks/navigation";

// Mock the API client so the store exercises its load/touch/rename/
// goToNewChat logic against controllable fakes — no network, no real
// openapi-fetch. vi.mock is hoisted, so the spies are declared with
// vi.hoisted to be in scope here.
const {
  archiveChatMock,
  assignChatProjectMock,
  clearChatProjectMock,
  createProjectMock,
  deleteChatMock,
  getOrCreateEmptyChatMock,
  listChatsMock,
  listProjectsMock,
  pinChatMock,
  renameChatMock,
  unarchiveChatMock,
  unpinChatMock,
} = vi.hoisted(() => ({
  archiveChatMock: vi.fn(),
  assignChatProjectMock: vi.fn(),
  clearChatProjectMock: vi.fn(),
  createProjectMock: vi.fn(),
  deleteChatMock: vi.fn(),
  getOrCreateEmptyChatMock: vi.fn(),
  listChatsMock: vi.fn(),
  listProjectsMock: vi.fn(),
  pinChatMock: vi.fn(),
  renameChatMock: vi.fn(),
  unarchiveChatMock: vi.fn(),
  unpinChatMock: vi.fn(),
}));

vi.mock("$lib/api/client", () => ({
  archiveChat: archiveChatMock,
  assignChatProject: assignChatProjectMock,
  clearChatProject: clearChatProjectMock,
  createProject: createProjectMock,
  deleteChat: deleteChatMock,
  getOrCreateEmptyChat: getOrCreateEmptyChatMock,
  listChats: listChatsMock,
  listProjects: listProjectsMock,
  pinChat: pinChatMock,
  renameChat: renameChatMock,
  unarchiveChat: unarchiveChatMock,
  unpinChat: unpinChatMock,
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

function project(id: string, name: string): Project {
  return {
    id,
    name,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };
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
    archiveChatMock.mockReset();
    assignChatProjectMock.mockReset();
    clearChatProjectMock.mockReset();
    createProjectMock.mockReset();
    deleteChatMock.mockReset();
    listChatsMock.mockReset();
    listProjectsMock.mockReset();
    pinChatMock.mockReset();
    renameChatMock.mockReset();
    unarchiveChatMock.mockReset();
    unpinChatMock.mockReset();
    getOrCreateEmptyChatMock.mockReset();
    gotoCalls.length = 0;
    chats.list = [];
    chats.loaded = false;
    chats.loading = false;
    chats.projects = [];
    chats.projectsLoaded = false;
    chats.archived = false;
    chats.search = "";
    chats.projectId = null;
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
      archived: false,
      search: undefined,
      projectId: undefined,
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

  it("loads projects separately so project controls have caller-owned choices", async () => {
    listProjectsMock.mockResolvedValue([
      project("project-2", "Sales"),
      project("project-1", "Operations"),
    ]);

    await chats.loadProjects();

    expect(chats.projects.map((item) => item.id)).toEqual([
      "project-2",
      "project-1",
    ]);
    expect(chats.projectsLoaded).toBe(true);
  });

  it("changes the archive and project filters in the list request", async () => {
    listChatsMock.mockResolvedValue(page([], 0));

    await chats.showArchived(true);
    await chats.setProject("project-1");

    expect(listChatsMock).toHaveBeenLastCalledWith({
      limit: 100,
      offset: 0,
      archived: true,
      search: undefined,
      projectId: "project-1",
    });
  });

  it("removes an archived chat from the active view using the server summary", async () => {
    chats.list = [chatSummary("1", "keep")];
    archiveChatMock.mockResolvedValue({
      ...chatSummary("1", "keep"),
      archivedAt: "2026-01-02T00:00:00Z",
    });

    await chats.archive("1");

    expect(archiveChatMock).toHaveBeenCalledWith("1");
    expect(chats.list).toEqual([]);
  });

  it("pins a chat above newer unpinned chats", async () => {
    chats.list = [
      { ...chatSummary("newer", "newer"), updatedAt: "2026-01-03T00:00:00Z" },
      { ...chatSummary("older", "older"), updatedAt: "2026-01-02T00:00:00Z" },
    ];
    pinChatMock.mockResolvedValue({
      ...chatSummary("older", "older"),
      updatedAt: "2026-01-02T00:00:00Z",
      pinnedAt: "2026-01-04T00:00:00Z",
    });

    await chats.pin("older");

    expect(chats.list.map((item) => item.id)).toEqual(["older", "newer"]);
  });

  it("updates a chat assignment and removes it when it no longer matches the project filter", async () => {
    chats.projectId = "project-1";
    chats.list = [{ ...chatSummary("1", "keep"), projectId: "project-1" }];
    clearChatProjectMock.mockResolvedValue(chatSummary("1", "keep"));

    await chats.setChatProject("1", null);

    expect(clearChatProjectMock).toHaveBeenCalledWith("1");
    expect(chats.list).toEqual([]);
  });

  it("creates a project and keeps the selector alphabetized", async () => {
    chats.projects = [project("project-2", "Sales")];
    createProjectMock.mockResolvedValue(project("project-1", "Operations"));

    await chats.createProject("Operations");

    expect(createProjectMock).toHaveBeenCalledWith("Operations");
    expect(chats.projects.map((item) => item.name)).toEqual([
      "Operations",
      "Sales",
    ]);
  });

  it("removes a deleted chat from the visible list", async () => {
    chats.list = [chatSummary("1", "delete"), chatSummary("2", "keep")];
    deleteChatMock.mockResolvedValue(undefined);

    await chats.delete("1");

    expect(deleteChatMock).toHaveBeenCalledWith("1");
    expect(chats.list.map((item) => item.id)).toEqual(["2"]);
  });

  it("keeps the latest response when a prior filtered list returns late", async () => {
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
});
