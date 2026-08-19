import createClient from "openapi-fetch";
import type { components, paths } from "./schema";
import { ApiError, toApiError } from "./errors";
import { log } from "$lib/log";
import {
  EVENT_API_ERROR,
  EVENT_API_REQUEST,
  EVENT_API_RESPONSE,
} from "$lib/common/log-events";

type Credentials = components["schemas"]["Credentials"];
type User = components["schemas"]["User"];
type AuthStatus = components["schemas"]["AuthStatus"];
type Model = components["schemas"]["Model"];
type ChatSummary = components["schemas"]["ChatSummary"];
type ChatList = components["schemas"]["ChatList"];
type Chat = components["schemas"]["Chat"];
type ChatSettings = components["schemas"]["ChatSettings"];
type PromptContextPreview = components["schemas"]["PromptContextPreview"];
type Message = components["schemas"]["Message"];
type MessageList = components["schemas"]["MessageList"];
type CreateUserRequest = components["schemas"]["CreateUserRequest"];
type MCPServer = components["schemas"]["MCPServer"];
type CreateMCPServerRequest = components["schemas"]["CreateMCPServerRequest"];
type UpdateMCPServerRequest = components["schemas"]["UpdateMCPServerRequest"];
type MCPImportResult = components["schemas"]["MCPImportResult"];
type MCPTool = components["schemas"]["MCPTool"];
type ChatMCPServer = components["schemas"]["ChatMCPServer"];
type ChatMCPServerUpdate = components["schemas"]["ChatMCPServerUpdate"];
type AdminReadiness = components["schemas"]["AdminReadiness"];

const METHOD_GET = "GET";
const METHOD_POST = "POST";
const METHOD_PATCH = "PATCH";
const METHOD_PUT = "PUT";
const METHOD_DELETE = "DELETE";

const PATH_AUTH_STATUS = "/api/v1/auth/status";
const PATH_AUTH_SETUP = "/api/v1/auth/setup";
const PATH_AUTH_LOGIN = "/api/v1/auth/login";
const PATH_AUTH_LOGOUT = "/api/v1/auth/logout";
const PATH_MODELS = "/api/v1/models";
const PATH_CHATS = "/api/v1/chats";
const PATH_USERS = "/api/v1/users";
const PATH_MCP_SERVERS = "/api/v1/mcp/servers";
const PATH_MCP_IMPORT = "/api/v1/mcp/import";
const PATH_ADMIN_READINESS = "/api/v1/admin/readiness";

// Default page sizes for the two paginated list endpoints, matching the
// OpenAPI spec's declared defaults (server-enforced; these are the client's
// own defaults for callers that don't pass explicit pagination args).
const DEFAULT_CHATS_LIMIT = 100;
const DEFAULT_MESSAGES_LIMIT = 50;
const DEFAULT_OFFSET = 0;

// Same-origin client, mounted at /api/v1 (the server-code mount point;
// api/api.yml's own servers.url only knows about /v1 — see server.go). Every
// typed call below therefore uses the schema's bare path keys (e.g. "/chats",
// not "/api/v1/chats"); PATH_* constants below carry the real wire path for
// logging/error messages only. credentials:"include" ships the HttpOnly
// session cookie the server sets on setup/login.
const client = createClient<paths>({
  baseUrl: "/api/v1",
  credentials: "include",
});

export type {
  Credentials,
  User,
  AuthStatus,
  Model,
  ChatSummary,
  ChatList,
  Chat,
  ChatSettings,
  PromptContextPreview,
  Message,
  MessageList,
  CreateUserRequest,
  MCPServer,
  CreateMCPServerRequest,
  UpdateMCPServerRequest,
  MCPImportResult,
  MCPTool,
  ChatMCPServer,
  ChatMCPServerUpdate,
  AdminReadiness,
};

// instrument logs request/response/error around one openapi-fetch call. It logs
// only method, path, status, and timing — NEVER request bodies, since setup and
// login carry passwords. The result envelope is passed straight through to
// unwrap; on a thrown transport error (network failure, no server) we log then
// re-throw so the phase logic upstream still sees it.
async function instrument<T>(
  method: string,
  path: string,
  call: () => Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<{ data?: T; error?: unknown; response: Response }> {
  log.debug(EVENT_API_REQUEST, { method, path });
  const startedAt = performance.now();

  let result: { data?: T; error?: unknown; response: Response };
  try {
    result = await call();
  } catch (err) {
    log.error(EVENT_API_ERROR, {
      method,
      path,
      code: "TRANSPORT",
      message: err instanceof Error ? err.message : String(err),
    });

    throw err;
  }

  const durationMs = Math.round(performance.now() - startedAt);
  const status = result.response.status;
  log.info(EVENT_API_RESPONSE, {
    method,
    path,
    status,
    duration_ms: durationMs,
  });

  return result;
}

// unwrap turns an openapi-fetch result into typed data or throws an ApiError.
// It keys off data presence: openapi-fetch returns exactly one of {data} or
// {error}, and for operations the spec declares no error body for, error is
// never — so testing data is the discriminant that works for every op. On the
// error path it logs api.error with the machine-readable code (no body).
function unwrap<T>(
  method: string,
  path: string,
  result: {
    data?: T;
    error?: unknown;
    response: Response;
  },
): T {
  if (result.data !== undefined) {
    return result.data;
  }

  const apiError = toApiError(result.response.status, result.error);
  log.error(EVENT_API_ERROR, {
    method,
    path,
    code: apiError.code,
    message: apiError.message,
  });

  throw apiError;
}

// getAuthStatus reports whether setup is needed and whether the caller already
// holds a valid session cookie.
export async function getAuthStatus(): Promise<AuthStatus> {
  const result = await instrument(METHOD_GET, PATH_AUTH_STATUS, () =>
    client.GET("/auth/status"),
  );

  return unwrap(METHOD_GET, PATH_AUTH_STATUS, result);
}

// setup bootstraps the first admin user and logs them in (server sets cookie).
export async function setup(credentials: Credentials): Promise<User> {
  const result = await instrument(METHOD_POST, PATH_AUTH_SETUP, () =>
    client.POST("/auth/setup", { body: credentials }),
  );

  return unwrap(METHOD_POST, PATH_AUTH_SETUP, result);
}

// login authenticates a user (server sets the session cookie on success).
export async function login(credentials: Credentials): Promise<User> {
  const result = await instrument(METHOD_POST, PATH_AUTH_LOGIN, () =>
    client.POST("/auth/login", { body: credentials }),
  );

  return unwrap(METHOD_POST, PATH_AUTH_LOGIN, result);
}

// logout revokes the current session. The spec declares only a 204 (no body),
// so we key off the HTTP status rather than an error envelope.
export async function logout(): Promise<void> {
  const { response } = await instrument(METHOD_POST, PATH_AUTH_LOGOUT, () =>
    client.POST("/auth/logout"),
  );

  if (!response.ok) {
    const apiError = new ApiError(response.status, {
      code: "UNKNOWN",
      message: `logout failed with status ${response.status}`,
    });
    log.error(EVENT_API_ERROR, {
      method: METHOD_POST,
      path: PATH_AUTH_LOGOUT,
      code: apiError.code,
      message: apiError.message,
    });

    throw apiError;
  }
}

// listModels returns the merged model list across configured upstreams.
export async function listModels(): Promise<Model[]> {
  const result = await instrument(METHOD_GET, PATH_MODELS, () =>
    client.GET("/models"),
  );

  return unwrap(METHOD_GET, PATH_MODELS, result);
}

// getAdminReadiness returns only redacted operational state for an admin.
export async function getAdminReadiness(): Promise<AdminReadiness> {
  const result = await instrument(METHOD_GET, PATH_ADMIN_READINESS, () =>
    client.GET("/admin/readiness"),
  );

  return unwrap(METHOD_GET, PATH_ADMIN_READINESS, result);
}

// listChats returns a page of the caller's chats as a paginated envelope
// ({items, limit, offset, total}). Defaults to DEFAULT_CHATS_LIMIT/0 when
// omitted.
export async function listChats(
  params: {
    limit?: number;
    offset?: number;
    search?: string;
  } = {},
): Promise<ChatList> {
  const limit = params.limit ?? DEFAULT_CHATS_LIMIT;
  const offset = params.offset ?? DEFAULT_OFFSET;
  const result = await instrument(METHOD_GET, PATH_CHATS, () =>
    client.GET("/chats", {
      params: {
        query: {
          limit,
          offset,
          search: params.search,
        },
      },
    }),
  );

  return unwrap(METHOD_GET, PATH_CHATS, result);
}

// getOrCreateEmptyChat returns the caller's reusable "empty chat" (no
// messages yet), creating one if none exists. At most one is kept per user —
// repeated calls before any message is sent resolve to the SAME chat.
export async function getOrCreateEmptyChat(): Promise<Chat> {
  const path = `${PATH_CHATS}/empty`;
  const result = await instrument(METHOD_POST, path, () =>
    client.POST("/chats/empty"),
  );

  return unwrap(METHOD_POST, path, result);
}

// getChat returns a chat's metadata (title, model, settings) — no messages.
// Fetch the message timeline via listChatMessages. The streaming turns are
// POSTed via the hand-rolled SSE client (stream.ts); this JSON GET is for
// chat metadata only.
export async function getChat(chatId: string): Promise<Chat> {
  const path = `${PATH_CHATS}/${chatId}`;
  const result = await instrument(METHOD_GET, path, () =>
    client.GET("/chats/{chatId}", {
      params: { path: { chatId } },
    }),
  );

  return unwrap(METHOD_GET, path, result);
}

// listChatMessages returns a page of a chat's messages, oldest-first, as a
// paginated envelope ({items, limit, offset, total}). Defaults to
// DEFAULT_MESSAGES_LIMIT/0 when omitted.
export async function listChatMessages(
  chatId: string,
  params: { limit?: number; offset?: number } = {},
): Promise<MessageList> {
  const limit = params.limit ?? DEFAULT_MESSAGES_LIMIT;
  const offset = params.offset ?? DEFAULT_OFFSET;
  const path = `${PATH_CHATS}/${chatId}/messages`;
  const result = await instrument(METHOD_GET, path, () =>
    client.GET("/chats/{chatId}/messages", {
      params: { path: { chatId }, query: { limit, offset } },
    }),
  );

  return unwrap(METHOD_GET, path, result);
}

// updateChatSettings replaces a chat's model-generation settings (full
// replacement — an omitted field clears that setting) and returns the stored
// settings echoed back by the server.
export async function updateChatSettings(
  chatId: string,
  settings: ChatSettings,
): Promise<ChatSettings> {
  const path = `${PATH_CHATS}/${chatId}/settings`;
  const result = await instrument(METHOD_PATCH, path, () =>
    client.PATCH("/chats/{chatId}/settings", {
      params: { path: { chatId } },
      body: settings,
    }),
  );

  return unwrap(METHOD_PATCH, path, result);
}

// previewChatContext returns the backend-tokenized context selection for an
// unsent draft. The server uses the same history-selection path as streaming.
export async function previewChatContext(
  chatId: string,
  message: string,
): Promise<PromptContextPreview> {
  const path = `${PATH_CHATS}/${chatId}/context-preview`;
  const result = await instrument(METHOD_POST, path, () =>
    client.POST("/chats/{chatId}/context-preview", {
      params: { path: { chatId } },
      body: { message },
    }),
  );

  return unwrap(METHOD_POST, path, result);
}

// renameChat sets a chat's title and returns the updated summary.
export async function renameChat(
  chatId: string,
  title: string,
): Promise<ChatSummary> {
  const path = `${PATH_CHATS}/${chatId}`;
  const result = await instrument(METHOD_PATCH, path, () =>
    client.PATCH("/chats/{chatId}", {
      params: { path: { chatId } },
      body: { title },
    }),
  );

  return unwrap(METHOD_PATCH, path, result);
}

// pinChat pins one chat above ordinary recent conversations.
export async function pinChat(chatId: string): Promise<ChatSummary> {
  const path = `${PATH_CHATS}/${chatId}/pin`;
  const result = await instrument(METHOD_PUT, path, () =>
    client.PUT("/chats/{chatId}/pin", {
      params: { path: { chatId } },
    }),
  );

  return unwrap(METHOD_PUT, path, result);
}

// unpinChat removes a chat pin.
export async function unpinChat(chatId: string): Promise<ChatSummary> {
  const path = `${PATH_CHATS}/${chatId}/pin`;
  const result = await instrument(METHOD_DELETE, path, () =>
    client.DELETE("/chats/{chatId}/pin", {
      params: { path: { chatId } },
    }),
  );

  return unwrap(METHOD_DELETE, path, result);
}

// deleteChat soft-deletes one chat. The server returns no response body.
export async function deleteChat(chatId: string): Promise<void> {
  const path = `${PATH_CHATS}/${chatId}`;
  const { response } = await instrument(METHOD_DELETE, path, () =>
    client.DELETE("/chats/{chatId}", {
      params: { path: { chatId } },
    }),
  );

  throwIfNotOk(METHOD_DELETE, path, response);
}

// throwIfNotOk raises + logs an ApiError for a no-body (204) op whose status is
// not 2xx. Used by the delete helpers, which the spec declares with no success
// body so there is no {data} envelope to key off of.
function throwIfNotOk(method: string, path: string, response: Response): void {
  if (response.ok) {
    return;
  }

  const apiError = new ApiError(response.status, {
    code: "UNKNOWN",
    message: `${method} ${path} failed with status ${response.status}`,
  });
  log.error(EVENT_API_ERROR, {
    method,
    path,
    code: apiError.code,
    message: apiError.message,
  });

  throw apiError;
}

// listUsers returns all users (admin-only server-side).
export async function listUsers(): Promise<User[]> {
  const result = await instrument(METHOD_GET, PATH_USERS, () =>
    client.GET("/users"),
  );

  return unwrap(METHOD_GET, PATH_USERS, result);
}

// createUser provisions a user (admin-only server-side). The password lives only
// in the request body openapi-fetch sends; it never reaches the logger.
export async function createUser(body: CreateUserRequest): Promise<User> {
  const result = await instrument(METHOD_POST, PATH_USERS, () =>
    client.POST("/users", { body }),
  );

  return unwrap(METHOD_POST, PATH_USERS, result);
}

// deleteUser removes a user by id (admin-only server-side). 204, no body.
export async function deleteUser(userId: string): Promise<void> {
  const path = `${PATH_USERS}/${userId}`;
  const { response } = await instrument(METHOD_DELETE, path, () =>
    client.DELETE("/users/{userId}", {
      params: { path: { userId } },
    }),
  );

  throwIfNotOk(METHOD_DELETE, path, response);
}

// listMCPServers returns the configured MCP servers (admin-only server-side).
export async function listMCPServers(): Promise<MCPServer[]> {
  const result = await instrument(METHOD_GET, PATH_MCP_SERVERS, () =>
    client.GET("/mcp/servers"),
  );

  return unwrap(METHOD_GET, PATH_MCP_SERVERS, result);
}

// createMCPServer adds an MCP server (admin-only server-side). Any secret in the
// env/headers maps lives only in the request body; it never reaches the logger.
export async function createMCPServer(
  body: CreateMCPServerRequest,
): Promise<MCPServer> {
  const result = await instrument(METHOD_POST, PATH_MCP_SERVERS, () =>
    client.POST("/mcp/servers", { body }),
  );

  return unwrap(METHOD_POST, PATH_MCP_SERVERS, result);
}

// deleteMCPServer removes an MCP server by id (admin-only server-side). 204.
export async function deleteMCPServer(serverId: string): Promise<void> {
  const path = `${PATH_MCP_SERVERS}/${serverId}`;
  const { response } = await instrument(METHOD_DELETE, path, () =>
    client.DELETE("/mcp/servers/{serverId}", {
      params: { path: { serverId } },
    }),
  );

  throwIfNotOk(METHOD_DELETE, path, response);
}

// importMCP imports servers from a Claude-style .mcp.json (admin-only). The raw
// file content can carry secrets, so it lives only in the request body.
export async function importMCP(content: string): Promise<MCPImportResult> {
  const result = await instrument(METHOD_POST, PATH_MCP_IMPORT, () =>
    client.POST("/mcp/import", { body: { content } }),
  );

  return unwrap(METHOD_POST, PATH_MCP_IMPORT, result);
}

// updateMCPServer edits a server and returns it with its (connecting) status.
// Any secret in the env/headers maps lives only in the request body.
export async function updateMCPServer(
  serverId: string,
  body: UpdateMCPServerRequest,
): Promise<MCPServer> {
  const path = `${PATH_MCP_SERVERS}/${serverId}`;
  const result = await instrument(METHOD_PATCH, path, () =>
    client.PATCH("/mcp/servers/{serverId}", {
      params: { path: { serverId } },
      body,
    }),
  );

  return unwrap(METHOD_PATCH, path, result);
}

// reconnectMCPServer kicks a background reconnect and returns the server.
export async function reconnectMCPServer(serverId: string): Promise<MCPServer> {
  const path = `${PATH_MCP_SERVERS}/${serverId}/reconnect`;
  const result = await instrument(METHOD_POST, path, () =>
    client.POST("/mcp/servers/{serverId}/reconnect", {
      params: { path: { serverId } },
    }),
  );

  return unwrap(METHOD_POST, path, result);
}

// listMCPServerTools lists a connected server's tools (admin-only). A server
// that isn't currently connected returns an empty array.
export async function listMCPServerTools(serverId: string): Promise<MCPTool[]> {
  const path = `${PATH_MCP_SERVERS}/${serverId}/tools`;
  const result = await instrument(METHOD_GET, path, () =>
    client.GET("/mcp/servers/{serverId}/tools", {
      params: { path: { serverId } },
    }),
  );

  return unwrap(METHOD_GET, path, result);
}

// listChatMCPServers returns every MCP server as it applies to this chat: its
// live global status plus whether its tools are enabled for THIS chat.
export async function listChatMCPServers(
  chatId: string,
): Promise<ChatMCPServer[]> {
  const path = `${PATH_CHATS}/${chatId}/mcp-servers`;
  const result = await instrument(METHOD_GET, path, () =>
    client.GET("/chats/{chatId}/mcp-servers", {
      params: { path: { chatId } },
    }),
  );

  return unwrap(METHOD_GET, path, result);
}

// updateChatMCPServer enables or disables one MCP server's tools for this chat
// and returns the server's updated per-chat view.
export async function updateChatMCPServer(
  chatId: string,
  serverId: string,
  body: ChatMCPServerUpdate,
): Promise<ChatMCPServer> {
  const path = `${PATH_CHATS}/${chatId}/mcp-servers/${serverId}`;
  const result = await instrument(METHOD_PATCH, path, () =>
    client.PATCH("/chats/{chatId}/mcp-servers/{serverId}", {
      params: { path: { chatId, serverId } },
      body,
    }),
  );

  return unwrap(METHOD_PATCH, path, result);
}
