import {
  listMCPServers,
  createMCPServer as apiCreateMCPServer,
  updateMCPServer as apiUpdateMCPServer,
  reconnectMCPServer as apiReconnectMCPServer,
  deleteMCPServer as apiDeleteMCPServer,
  importMCP as apiImportMCP,
  listMCPServerTools as apiListMCPServerTools,
  type MCPServer,
  type MCPTool,
  type CreateMCPServerRequest,
  type UpdateMCPServerRequest,
} from "$lib/api/client";
import { log } from "$lib/log";
import { MCP_STATUS_CONNECTING, MCP_STATUS_FAILED } from "$lib/common/mcp";
import {
  EVENT_ADMIN_MCP_CREATE,
  EVENT_ADMIN_MCP_UPDATE,
  EVENT_ADMIN_MCP_RECONNECT,
  EVENT_ADMIN_MCP_DELETE,
  EVENT_ADMIN_MCP_IMPORT,
  EVENT_MCP_SERVERS_ERROR,
  EVENT_MCP_SERVERS_LOADED,
} from "$lib/common/log-events";

// ServerToolsState is a server's lazily-loaded tool cache for the expandable
// tools view (list is null until fetched).
interface ServerToolsState {
  loading: boolean;
  error: string | null;
  list: MCPTool[] | null;
}

// McpServersStore holds the configured MCP servers with their live connection
// status. load() runs from the admin MCP page and from the root layout once
// authed (so the sidebar can show the enabled-server count). Mutations connect
// in the BACKGROUND server-side, so they return immediately and status settles
// to connected/failed on its own — the admin page polls via refresh() while any
// server is still connecting. Secrets (env/header values, raw .mcp.json content)
// live only in request bodies — never in store state or log lines.
class McpServersStore {
  list = $state<MCPServer[]>([]);
  loaded = $state(false);
  loading = $state(false);
  // error holds the last load failure's message for inline display; cleared on
  // the next load attempt. Never carries secret payloads.
  error = $state<string | null>(null);
  // tools caches each server's tool list, lazily fetched when a card's tools
  // section is first expanded (keyed by server id). Cleared on reconnect / edit
  // / delete so a re-expand refetches. Never carries secrets.
  tools = $state<Record<string, ServerToolsState>>({});

  // enabledCount is the number of enabled servers — the sidebar [MCP:n] source.
  get enabledCount(): number {
    return this.list.filter((s) => s.enabled).length;
  }

  // anySettling drives status polling on the admin page: keep refreshing while
  // any server's status could still change on its own — either mid-connect, or
  // failed (the manager auto-retries a failed server in the background, so
  // "failed" is not terminal; polling must keep running to pick up a silent
  // recovery, not just stop the moment a connect attempt fails).
  get anySettling(): boolean {
    return this.list.some(
      (s) =>
        s.status === MCP_STATUS_CONNECTING || s.status === MCP_STATUS_FAILED,
    );
  }

  async load(): Promise<void> {
    this.loading = true;
    this.error = null;

    try {
      this.list = await listMCPServers();
      this.loaded = true;
      log.info(EVENT_MCP_SERVERS_LOADED, {
        count: this.list.length,
        enabled: this.enabledCount,
      });
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_MCP_SERVERS_ERROR, { message: this.error });
    } finally {
      this.loading = false;
    }
  }

  // refresh re-fetches the list WITHOUT toggling the loading spinner, so status
  // polling doesn't flash the table back to a loading state.
  async refresh(): Promise<void> {
    try {
      this.list = await listMCPServers();
      this.error = null;
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_MCP_SERVERS_ERROR, { message: this.error });
    }
  }

  // create adds a server then refreshes. Logs only name + transport — never the
  // command args, url, header values, or env values.
  async create(body: CreateMCPServerRequest): Promise<void> {
    const created = await apiCreateMCPServer(body);
    log.info(EVENT_ADMIN_MCP_CREATE, {
      id: created.id,
      name: created.name,
      transport: created.transport,
    });
    await this.refresh();
  }

  // update edits a server then refreshes. Secrets stay in the request body.
  async update(serverId: string, body: UpdateMCPServerRequest): Promise<void> {
    const updated = await apiUpdateMCPServer(serverId, body);
    this.invalidateTools(serverId);
    log.info(EVENT_ADMIN_MCP_UPDATE, {
      id: updated.id,
      name: updated.name,
      transport: updated.transport,
    });
    await this.refresh();
  }

  async reconnect(serverId: string): Promise<void> {
    await apiReconnectMCPServer(serverId);
    this.invalidateTools(serverId);
    log.info(EVENT_ADMIN_MCP_RECONNECT, { id: serverId });
    await this.refresh();
  }

  async remove(serverId: string): Promise<void> {
    await apiDeleteMCPServer(serverId);
    this.invalidateTools(serverId);
    log.info(EVENT_ADMIN_MCP_DELETE, { id: serverId });
    await this.refresh();
  }

  // importJSON imports servers from raw .mcp.json content then refreshes. The
  // raw content is passed straight to the request body and never logged; only
  // the count of imported servers is.
  async importJSON(content: string): Promise<void> {
    const result = await apiImportMCP(content);
    log.info(EVENT_ADMIN_MCP_IMPORT, { imported: result.imported.length });
    await this.refresh();
  }

  // loadTools lazily fetches a server's tools on first expand and caches them by
  // id; a cached (non-null) list short-circuits. Errors are held per server for
  // inline display, never thrown to the caller.
  async loadTools(serverId: string): Promise<void> {
    const cached = this.tools[serverId];
    if (cached !== undefined && cached.list !== null) {
      return;
    }

    this.tools = {
      ...this.tools,
      [serverId]: { loading: true, error: null, list: null },
    };

    try {
      const list = await apiListMCPServerTools(serverId);
      this.tools = {
        ...this.tools,
        [serverId]: { loading: false, error: null, list },
      };
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.tools = {
        ...this.tools,
        [serverId]: { loading: false, error: message, list: null },
      };
    }
  }

  // invalidateTools drops a server's cached tools so the next expand refetches
  // (its live tool set may have changed after a reconnect/edit).
  invalidateTools(serverId: string): void {
    if (this.tools[serverId] === undefined) {
      return;
    }

    const next = { ...this.tools };
    delete next[serverId];
    this.tools = next;
  }
}

export const mcpServers = new McpServersStore();
