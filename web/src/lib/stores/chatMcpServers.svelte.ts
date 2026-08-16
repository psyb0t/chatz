import {
  listChatMCPServers,
  updateChatMCPServer,
  type ChatMCPServer,
} from "$lib/api/client";
import { log } from "$lib/log";
import { MCP_STATUS_CONNECTING, MCP_STATUS_FAILED } from "$lib/common/mcp";
import {
  EVENT_CHAT_MCP_LOADED,
  EVENT_CHAT_MCP_ERROR,
  EVENT_CHAT_MCP_TOGGLE,
} from "$lib/common/log-events";

// ChatMcpServersStore holds the MCP servers as they apply to ONE chat: identity,
// live global status, and per-chat enablement. Enabling/disabling here is a
// per-chat preference, independent of a server's global enabled flag. load()
// runs when the composer's MCP popup mounts / the active chat changes; the popup
// polls via refresh() while any server's status is still settling. Secrets never
// enter store state or log lines — only ids, names, counts, and the enabled flag.
class ChatMcpServersStore {
  list = $state<ChatMCPServer[]>([]);
  loaded = $state(false);
  loading = $state(false);
  error = $state<string | null>(null);
  // chatId is the chat the current list belongs to; refresh() re-fetches for it.
  chatId = $state<string | null>(null);

  get enabledCount(): number {
    return this.list.filter((s) => s.enabled).length;
  }

  // anySettling drives the popup's status polling: keep refreshing while any
  // server could still change on its own — mid-connect, or failed (the manager
  // auto-retries a failed server, so failed is not terminal).
  get anySettling(): boolean {
    return this.list.some(
      (s) =>
        s.status === MCP_STATUS_CONNECTING || s.status === MCP_STATUS_FAILED,
    );
  }

  async load(chatId: string): Promise<void> {
    this.loading = true;
    this.error = null;
    this.chatId = chatId;

    try {
      this.list = await listChatMCPServers(chatId);
      this.loaded = true;
      log.info(EVENT_CHAT_MCP_LOADED, {
        chat_id: chatId,
        count: this.list.length,
        enabled: this.enabledCount,
      });
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_CHAT_MCP_ERROR, { chat_id: chatId, message: this.error });
    } finally {
      this.loading = false;
    }
  }

  // refresh re-fetches for the current chat WITHOUT toggling the loading spinner
  // (status polling must not flash the list back to a loading state).
  async refresh(): Promise<void> {
    const chatId = this.chatId;
    if (chatId === null) {
      return;
    }

    try {
      this.list = await listChatMCPServers(chatId);
      this.error = null;
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_CHAT_MCP_ERROR, { chat_id: chatId, message: this.error });
    }
  }

  // setEnabled toggles one server's tools for this chat and patches the matching
  // row in place with the server's returned view (no full refetch).
  async setEnabled(serverId: string, enabled: boolean): Promise<void> {
    const chatId = this.chatId;
    if (chatId === null) {
      return;
    }

    const updated = await updateChatMCPServer(chatId, serverId, { enabled });
    this.list = this.list.map((s) => (s.id === updated.id ? updated : s));
    log.info(EVENT_CHAT_MCP_TOGGLE, {
      chat_id: chatId,
      id: serverId,
      enabled,
    });
  }

  // reset clears the store when leaving a chat so a new chat starts fresh.
  reset(): void {
    this.list = [];
    this.loaded = false;
    this.loading = false;
    this.error = null;
    this.chatId = null;
  }
}

export const chatMcpServers = new ChatMcpServersStore();
