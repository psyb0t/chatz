import { goto } from "$app/navigation";
import { page } from "$app/state";
import {
  deleteChat as apiDeleteChat,
  listChats,
  pinChat as apiPinChat,
  renameChat as apiRenameChat,
  unpinChat as apiUnpinChat,
  getOrCreateEmptyChat,
  type ChatSummary,
} from "$lib/api/client";
import { chatRoute } from "$lib/common/routes";
import { log } from "$lib/log";
import {
  EVENT_CHATS_ERROR,
  EVENT_CHATS_LOADED,
  EVENT_ADMIN_CHAT_RENAME,
} from "$lib/common/log-events";

// SIDEBAR_CHATS_LIMIT is a generous page size so the sidebar shows the same
// chats today's bare-array listing did for any realistic user, without
// building pagination UI. Loading beyond this page is a natural follow-up
// (not built here — see the store's load() doc).
const SIDEBAR_CHATS_LIMIT = 100;
const EMPTY_SEARCH = "";

function compareChats(left: ChatSummary, right: ChatSummary): number {
  const leftPinned = left.pinnedAt !== undefined;
  const rightPinned = right.pinnedAt !== undefined;
  if (leftPinned !== rightPinned) {
    return leftPinned ? -1 : 1;
  }

  return right.updatedAt.localeCompare(left.updatedAt);
}

function sortChats(items: ChatSummary[]): ChatSummary[] {
  return [...items].sort(compareChats);
}

// ChatsStore holds the caller's chat list. load() is called once the auth phase
// flips to authed (see the root layout).
class ChatsStore {
  list = $state<ChatSummary[]>([]);
  loaded = $state(false);
  loading = $state(false);
  search = $state(EMPTY_SEARCH);
  // error holds the last load failure's message so the sidebar can surface it
  // inline instead of only logging it; cleared on the next load attempt.
  error = $state<string | null>(null);
  #loadGeneration = 0;

  // load fetches the first SIDEBAR_CHATS_LIMIT chats (newest activity first)
  // and unwraps the paginated envelope's items into the reactive list. A
  // caller with more than SIDEBAR_CHATS_LIMIT chats won't see the rest here —
  // no "load more" UI is built (out of scope for this fix); the envelope's
  // `total` is available on the response if that becomes a real need.
  async load(): Promise<void> {
    const loadGeneration = ++this.#loadGeneration;
    this.loading = true;
    this.error = null;

    try {
      const page = await listChats({
        limit: SIDEBAR_CHATS_LIMIT,
        offset: 0,
        search: this.search || undefined,
      });
      if (loadGeneration === this.#loadGeneration) {
        this.list = sortChats(page.items);
        this.loaded = true;
        log.info(EVENT_CHATS_LOADED, { count: this.list.length });
      }
    } catch (err) {
      if (loadGeneration === this.#loadGeneration) {
        this.captureError(err);
      }
    } finally {
      if (loadGeneration === this.#loadGeneration) {
        this.loading = false;
      }
    }
  }

  // setSearch applies a literal title query. The server owns matching
  // semantics and validates the query length.
  async setSearch(search: string): Promise<void> {
    if (this.search === search) {
      return;
    }

    this.search = search;
    await this.load();
  }

  // touch bumps a chat to the head of the sidebar list (adding it if it
  // wasn't there yet — its first message just made it non-empty, see
  // conversation.send) and refreshes its updatedAt, mirroring the server-side
  // activity-order bump — so the list re-sorts immediately instead of waiting
  // for the next full load(). title is passed only for a chat's first message
  // (conversation.send); omitted, an already-listed chat keeps its title.
  touch(id: string, title?: string): void {
    const now = new Date().toISOString();
    const existing = this.list.find((c) => c.id === id);
    const entry = existing
      ? { ...existing, title: title ?? existing.title, updatedAt: now }
      : { id, title: title ?? "", createdAt: now, updatedAt: now };

    const remaining = this.list.filter((chat) => chat.id !== id);
    if (!this.matchesFilters(entry)) {
      this.list = remaining;

      return;
    }

    if (
      entry.pinnedAt !== undefined ||
      remaining.some((chat) => chat.pinnedAt !== undefined)
    ) {
      this.list = sortChats([...remaining, entry]);

      return;
    }

    this.list = [entry, ...remaining];
  }

  // goToNewChat resolves the caller's reusable "empty chat" (creating one if
  // none exists — the backend enforces at most one per user) and navigates
  // there. Used by the sidebar's "+ New chat" button AND the root route (the
  // auth guard's default landing page), so repeated "New chat" clicks with
  // nothing typed always land back on the SAME chat instead of piling up
  // unused rows.
  async goToNewChat(): Promise<void> {
    this.error = null;

    try {
      const chat = await getOrCreateEmptyChat();
      void goto(chatRoute(chat.id));
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_CHATS_ERROR, { message: this.error });
    }
  }

  // rename persists a new title for a chat and updates the list in place with
  // the server-echoed (trimmed + capped) title. A failure is logged and the old
  // title kept — the client layer already logged the transport error.
  async rename(id: string, title: string): Promise<void> {
    try {
      const updated = await apiRenameChat(id, title);
      log.info(EVENT_ADMIN_CHAT_RENAME, { id });
      this.replaceVisible(updated);
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_CHATS_ERROR, { message: this.error });
    }
  }

  async pin(id: string): Promise<void> {
    await this.applyChatMutation(() => apiPinChat(id));
  }

  async unpin(id: string): Promise<void> {
    await this.applyChatMutation(() => apiUnpinChat(id));
  }

  // delete soft-deletes the chat and drops it from the sidebar. When it is the
  // chat currently on screen the router is moved off it too: the route param
  // would otherwise still name a row the API no longer resolves, leaving a dead
  // view in place that only fails on reload.
  async delete(id: string): Promise<void> {
    try {
      await apiDeleteChat(id);
      this.list = this.list.filter((chat) => chat.id !== id);

      if (page.params.chatId !== id) {
        return;
      }

      await this.goToNewChat();
    } catch (err) {
      this.captureError(err);
    }
  }

  private matchesFilters(chat: ChatSummary): boolean {
    return chat.title
      .toLocaleLowerCase()
      .includes(this.search.toLocaleLowerCase());
  }

  private replaceVisible(updated: ChatSummary): void {
    const withoutUpdated = this.list.filter((chat) => chat.id !== updated.id);
    this.list = this.matchesFilters(updated)
      ? sortChats([...withoutUpdated, updated])
      : withoutUpdated;
  }

  private async applyChatMutation(
    mutation: () => Promise<ChatSummary>,
  ): Promise<void> {
    try {
      this.replaceVisible(await mutation());
    } catch (err) {
      this.captureError(err);
    }
  }

  private captureError(err: unknown): void {
    this.error = err instanceof Error ? err.message : String(err);
    log.error(EVENT_CHATS_ERROR, { message: this.error });
  }
}

export const chats = new ChatsStore();
