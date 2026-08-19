// data-testid values. Kept in one place so component markup and any future e2e
// assertions reference the same strings (never spelled inline).
export const TESTID_MESSAGE_LIST = "message-list";
export const TESTID_MESSAGE = "message";
export const TESTID_ASSISTANT_TEXT = "assistant-text";
export const TESTID_SPEC_BLOCK = "spec-block";
export const TESTID_TOOL_CARD = "tool-card";
export const TESTID_THINKING_BLOCK = "thinking-block";
export const TESTID_COMPOSER = "composer";
export const TESTID_COMPOSER_SEND = "send";
export const TESTID_COMPOSER_STOP = "stop";
export const TESTID_CONTEXT_METER = "context-meter";

// Shared loading / empty / error state markers.
export const TESTID_CHAT_LOADING = "chat-loading";
export const TESTID_CHAT_ERROR = "chat-error";
export const TESTID_CHAT_RETRY_TURN = "chat-retry-turn";
export const TESTID_CHAT_TURN_STATUS = "chat-turn-status";

// Root route ("/"): the redirector that resolves the caller's empty chat and
// navigates to it (see chats.goToNewChat).
export const TESTID_NEW_CHAT_LOADING = "new-chat-loading";
export const TESTID_NEW_CHAT_ERROR = "new-chat-error";
export const TESTID_SIDEBAR_LOADING = "sidebar-loading";
export const TESTID_SIDEBAR_ERROR = "sidebar-error";
export const TESTID_ADMIN_LOADING = "admin-loading";
export const TESTID_ADMIN_EMPTY = "admin-empty";
export const TESTID_ADMIN_ERROR = "admin-error";

// Admin nav + screens.
export const TESTID_NAV_ADMIN_USERS = "admin-users";
export const TESTID_NAV_ADMIN_MCP = "admin-mcp";
export const TESTID_NAV_ADMIN_READINESS = "admin-readiness";
export const TESTID_ADMIN_FORBIDDEN = "admin-forbidden";
export const TESTID_ADMIN_READINESS = "admin-readiness-page";
export const TESTID_ADMIN_READINESS_CONTENT = "admin-readiness-content";

// Sidebar collapse/expand toggle.
export const TESTID_SIDEBAR_TOGGLE = "sidebar-toggle";

// Mobile drawer: the hamburger that opens the off-canvas sidebar and the
// backdrop that closes it on an outside tap.
export const TESTID_SIDEBAR_OPEN = "sidebar-open";
export const TESTID_SIDEBAR_BACKDROP = "sidebar-backdrop";

// Sidebar chat rename.
export const TESTID_CHAT_RENAME = "chat-rename";
export const TESTID_CHAT_RENAME_INPUT = "chat-rename-input";
export const TESTID_CHAT_SEARCH = "chat-search";
export const TESTID_CHAT_DELETE = "chat-delete";
// The per-row "⋮" trigger and the edit item inside its popup menu. Delete reuses
// TESTID_CHAT_DELETE; rename reuses TESTID_CHAT_RENAME.
export const TESTID_CHAT_MENU = "chat-menu";

// MCP admin: per-server actions.
export const TESTID_MCP_RECONNECT = "mcp-reconnect";
export const TESTID_MCP_EDIT = "mcp-edit";
export const TESTID_MCP_ADD_TOGGLE = "mcp-add-toggle";
export const TESTID_MCP_IMPORT_TOGGLE = "mcp-import-toggle";
export const TESTID_MCP_EDIT_SUBMIT = "mcp-edit-submit";
export const TESTID_MCP_MODAL = "mcp-modal";
export const TESTID_MCP_TOOLS_TOGGLE = "mcp-tools-toggle";
export const TESTID_MCP_TOOLS_PANEL = "mcp-tools-panel";
export const TESTID_MCP_HEALTH = "mcp-health";

// Per-chat model settings.
export const TESTID_CHAT_SETTINGS_TOGGLE = "chat-settings-toggle";
export const TESTID_CHAT_SETTINGS_PANEL = "chat-settings-panel";
export const TESTID_CHAT_SETTINGS_PRESET = "chat-settings-preset";
export const TESTID_CHAT_SETTINGS_PRESET_NAME = "chat-settings-preset-name";
export const TESTID_CHAT_SETTINGS_PRESET_SAVE = "chat-settings-preset-save";
export const TESTID_CHAT_SETTINGS_PRESET_DELETE = "chat-settings-preset-delete";

// Searchable model picker (composer).
export const TESTID_MODEL_PICKER = "model-picker";
export const TESTID_MODEL_PICKER_SEARCH = "model-picker-search";

// Per-chat MCP server enable/disable (composer trigger + popup).
export const TESTID_CHAT_MCP_TOGGLE = "chat-mcp-toggle";
export const TESTID_CHAT_MCP_PANEL = "chat-mcp-panel";
export const TESTID_CHAT_MCP_ITEM = "chat-mcp-item";
export const TESTID_CHAT_MCP_ITEM_TOGGLE = "chat-mcp-item-toggle";

export const TESTID_USER_CREATE_SUBMIT = "user-create-submit";
export const TESTID_USER_CREATE_USERNAME = "user-create-username";
export const TESTID_USER_CREATE_PASSWORD = "user-create-password";
export const TESTID_USER_CREATE_ADMIN = "user-create-admin";
export const TESTID_USER_DELETE = "user-delete";

export const TESTID_MCP_CREATE_SUBMIT = "mcp-create-submit";
export const TESTID_MCP_CREATE_NAME = "mcp-create-name";
export const TESTID_MCP_CREATE_TRANSPORT = "mcp-create-transport";
export const TESTID_MCP_CREATE_COMMAND = "mcp-create-command";
export const TESTID_MCP_CREATE_ARGS = "mcp-create-args";
export const TESTID_MCP_CREATE_URL = "mcp-create-url";
export const TESTID_MCP_CREATE_HEADERS = "mcp-create-headers";
export const TESTID_MCP_CREATE_ENV = "mcp-create-env";
export const TESTID_MCP_DELETE = "mcp-delete";
export const TESTID_MCP_IMPORT_SUBMIT = "mcp-import-submit";
export const TESTID_MCP_IMPORT_CONTENT = "mcp-import-content";

// Rendered in place of a generative-UI block whose spec failed validation.
export const TESTID_SPEC_ERROR = "spec-error";
