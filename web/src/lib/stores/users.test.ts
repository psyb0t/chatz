import { describe, it, expect, beforeEach, vi } from "vitest";
import type { User } from "$lib/api/client";

// Mock the API client so the store exercises its load/create/remove logic
// against controllable fakes — no network, no real openapi-fetch. vi.mock is
// hoisted, so the spies are declared with vi.hoisted to be in scope here.
const { listUsersMock, createUserMock, deleteUserMock } = vi.hoisted(() => ({
  listUsersMock: vi.fn(),
  createUserMock: vi.fn(),
  deleteUserMock: vi.fn(),
}));

vi.mock("$lib/api/client", () => ({
  listUsers: listUsersMock,
  createUser: createUserMock,
  deleteUser: deleteUserMock,
}));

// Import AFTER the mock is registered so the store binds to the fakes.
const { users } = await import("./users.svelte");

function user(id: string, username: string, isAdmin = false): User {
  return {
    id,
    username,
    isAdmin,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

describe("UsersStore", () => {
  beforeEach(() => {
    listUsersMock.mockReset();
    createUserMock.mockReset();
    deleteUserMock.mockReset();
    users.list = [];
    users.loaded = false;
    users.loading = false;
    users.error = null;
  });

  it("load populates the list and sets loaded", async () => {
    listUsersMock.mockResolvedValue([user("1", "admin", true)]);

    await users.load();

    expect(users.loaded).toBe(true);
    expect(users.list).toHaveLength(1);
    expect(users.list[0].username).toBe("admin");
    expect(users.loading).toBe(false);
    expect(users.error).toBeNull();
  });

  it("load swallows errors, surfaces the message, and leaves loaded false", async () => {
    listUsersMock.mockRejectedValue(new Error("boom"));

    await users.load();

    expect(users.loaded).toBe(false);
    expect(users.list).toEqual([]);
    expect(users.loading).toBe(false);
    expect(users.error).toBe("boom");
  });

  it("load clears a prior error on a subsequent success", async () => {
    listUsersMock.mockRejectedValueOnce(new Error("boom"));
    await users.load();
    expect(users.error).toBe("boom");

    listUsersMock.mockResolvedValueOnce([user("1", "admin", true)]);
    await users.load();

    expect(users.error).toBeNull();
    expect(users.loaded).toBe(true);
  });

  it("create calls the API then re-loads the list", async () => {
    createUserMock.mockResolvedValue(user("2", "bob"));
    listUsersMock.mockResolvedValue([
      user("1", "admin", true),
      user("2", "bob"),
    ]);

    await users.create({ username: "bob", password: "pw", isAdmin: false });

    expect(createUserMock).toHaveBeenCalledWith({
      username: "bob",
      password: "pw",
      isAdmin: false,
    });
    expect(listUsersMock).toHaveBeenCalledTimes(1);
    expect(users.list).toHaveLength(2);
  });

  it("remove calls the API then re-loads the list", async () => {
    deleteUserMock.mockResolvedValue(undefined);
    listUsersMock.mockResolvedValue([user("1", "admin", true)]);

    await users.remove("2");

    expect(deleteUserMock).toHaveBeenCalledWith("2");
    expect(listUsersMock).toHaveBeenCalledTimes(1);
    expect(users.list).toHaveLength(1);
  });
});
