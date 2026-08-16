// Dot-namespaced log event names. Every log.* call references one of these so no
// event string is spelled inline (and the e2e harness can grep for them).
export const EVENT_APP_BOOT = "app.boot";
export const EVENT_ROUTE_GUARD = "route.guard";

export const EVENT_API_REQUEST = "api.request";
export const EVENT_API_RESPONSE = "api.response";
export const EVENT_API_ERROR = "api.error";

export const EVENT_AUTH_PHASE = "auth.phase";
export const EVENT_AUTH_SETUP = "auth.setup";
export const EVENT_AUTH_LOGIN = "auth.login";
export const EVENT_AUTH_LOGOUT = "auth.logout";

export const EVENT_CHATS_LOADED = "chats.loaded";
export const EVENT_CHATS_ERROR = "chats.error";
export const EVENT_MODELS_LOADED = "models.loaded";
export const EVENT_MODELS_ERROR = "models.error";

export const EVENT_USERS_LOADED = "users.loaded";
export const EVENT_USERS_ERROR = "users.error";
export const EVENT_MCP_SERVERS_LOADED = "mcp_servers.loaded";
export const EVENT_MCP_SERVERS_ERROR = "mcp_servers.error";

// Per-chat MCP enable/disable (composer popup). Payloads carry only chat_id,
// server id, count, and the enabled flag — never secrets.
export const EVENT_CHAT_MCP_LOADED = "chat_mcp.loaded";
export const EVENT_CHAT_MCP_ERROR = "chat_mcp.error";
export const EVENT_CHAT_MCP_TOGGLE = "chat_mcp.toggle";

// Admin actions. NEVER carry passwords, env values, header values, or raw
// .mcp.json content in these payloads — only ids, names, counts, and transport.
export const EVENT_ADMIN_USER_CREATE = "admin.user.create";
export const EVENT_ADMIN_USER_DELETE = "admin.user.delete";
export const EVENT_ADMIN_MCP_CREATE = "admin.mcp.create";
export const EVENT_ADMIN_MCP_DELETE = "admin.mcp.delete";
export const EVENT_ADMIN_MCP_IMPORT = "admin.mcp.import";
export const EVENT_ADMIN_MCP_UPDATE = "admin.mcp.update";
export const EVENT_ADMIN_MCP_RECONNECT = "admin.mcp.reconnect";
export const EVENT_ADMIN_CHAT_RENAME = "admin.chat.rename";

export const EVENT_SSE_OPEN = "sse.open";
export const EVENT_SSE_EVENT = "sse.event";
export const EVENT_SSE_TEXT_DELTA = "sse.text_delta";
export const EVENT_SSE_DONE = "sse.done";
export const EVENT_SSE_ERROR = "sse.error";

export const EVENT_TOOL_USE = "tool.use";
export const EVENT_TOOL_RESULT = "tool.result";
export const EVENT_THINKING = "thinking";

export const EVENT_CHAT_CREATE = "chat.create";
export const EVENT_CHAT_SEND = "chat.send";
export const EVENT_CHAT_LOAD = "chat.load";
export const EVENT_CHAT_LOAD_ERROR = "chat.load_error";
export const EVENT_COMPOSER_PRESET_STORAGE_ERROR =
  "composer_preset.storage_error";

// ```spec fence detection + json-render pipeline (client-side GenUI).
export const EVENT_SPEC_OPEN = "spec.open";
export const EVENT_SPEC_PATCH = "spec.patch";
export const EVENT_SPEC_RENDER = "spec.render";
export const EVENT_SPEC_ERROR = "spec.error";
