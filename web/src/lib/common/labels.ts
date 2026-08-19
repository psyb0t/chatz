// User-facing UI label strings, kept in one place so markup never spells them
// inline (rename-safe, grep-able). These are the fixed chrome labels — not
// runtime data.

// Topbar status chips.
export const CHIP_ADMIN = "[ADMIN]";

// mcpChip builds the connected-MCP-server status pill from the live enabled
// count (e.g. `[MCP:2]`). The count is real runtime data, not a fixed label.
export function mcpChip(count: number): string {
  return `[MCP:${count}]`;
}

// Sidebar brand + collapse toggle. The wordmark now lives in the left bar.
export const SIDEBAR_WORDMARK = "Chatz";
export const SIDEBAR_CREDIT_LABEL = "by psyb0t";
export const SIDEBAR_CREDIT_URL = "https://github.com/psyb0t/";
export const A11Y_SIDEBAR_COLLAPSE = "Collapse sidebar";
export const A11Y_SIDEBAR_EXPAND = "Expand sidebar";
// Mobile off-canvas drawer open/close (hamburger + in-drawer close button).
export const A11Y_SIDEBAR_OPEN = "Open sidebar";
export const A11Y_SIDEBAR_CLOSE = "Close sidebar";

// Sidebar admin nav labels (moved from the removed topbar).
export const NAV_ADMIN_USERS = "Users";
export const NAV_ADMIN_MCP = "MCP";
export const NAV_ADMIN_READINESS = "Readiness";

// Admin screen headings + action labels.
export const ADMIN_USERS_TITLE = "Users";
export const ADMIN_USERS_CREATE_TITLE = "Create User";
export const ADMIN_MCP_TITLE = "MCP Servers";
export const ADMIN_READINESS_TITLE = "Readiness";
export const ADMIN_READINESS_RUNTIME = "Runtime";
export const ADMIN_READINESS_BACKUP = "Backup";
export const ADMIN_READINESS_UPSTREAMS = "Upstreams";
export const ADMIN_READINESS_NOT_RECORDED = "No backup completion recorded.";
export const ADMIN_READINESS_NONE = "No upstreams configured.";
export const LABEL_REFRESH = "Refresh";
export const ADMIN_MCP_CREATE_TITLE = "Add Server";
export const ADMIN_MCP_EDIT_TITLE = "Edit Server";
export const ADMIN_MCP_IMPORT_TITLE = "Import .mcp.json";
export const ADMIN_FORBIDDEN_TITLE = "Forbidden";
export const ADMIN_FORBIDDEN_BODY = "Admin access required.";
export const LABEL_DELETE = "Delete";
export const LABEL_CREATE = "Create";
export const LABEL_ADD = "Add";
export const LABEL_IMPORT = "Import";
export const LABEL_DISMISS = "Dismiss";
export const LABEL_RETRY = "Retry";
export const LABEL_LOGOUT = "Logout";
export const LABEL_RECONNECT = "Reconnect";
export const LABEL_EDIT = "Edit";
export const LABEL_CANCEL = "Cancel";

// MCP per-server tools disclosure.
export const MCP_TOOLS_NONE = "No tools available.";
export const MCP_TOOLS_NO_PARAMS = "No parameters.";
export const MCP_TOOLS_REQUIRED = "required";
export const A11Y_CHAT_RENAME = "Rename chat";
export const A11Y_CHAT_SEARCH = "Search chats";
export const A11Y_CHAT_PIN = "Pin chat";
export const A11Y_CHAT_UNPIN = "Unpin chat";
export const A11Y_CHAT_DELETE = "Delete chat";
export const LABEL_SAVE = "Save";
export const LABEL_CLOSE = "Close";

// Per-chat model settings panel.
export const SETTINGS_TITLE = "Model settings";
export const A11Y_CHAT_SETTINGS = "Model settings";
export const SETTINGS_REASONING = "Reasoning effort";
export const SETTINGS_REASONING_AUTO = "Auto";
export const SETTINGS_TEMPERATURE = "Temperature";
export const SETTINGS_TOP_P = "Top-P";
export const SETTINGS_MAX_OUTPUT = "Max output tokens";
export const SETTINGS_MAX_HISTORY = "Max history tokens";
export const SETTINGS_UNSET_HINT =
  "leave empty for provider defaults; history defaults to 100,000 tokens";
export const SETTINGS_NEEDS_CHAT = "Send a message first to save settings.";
export const SETTINGS_REASONING_UNSUPPORTED =
  "This model does not advertise reasoning controls.";
export const SETTINGS_PRESET = "Preset";
export const SETTINGS_PRESET_NONE = "Choose a preset";
export const SETTINGS_PRESET_NAME = "Preset name";
export const SETTINGS_PRESET_SAVE = "Save preset";
export const SETTINGS_PRESET_DELETE = "Delete preset";
export const SETTINGS_PRESET_HINT =
  "Presets stay in this browser and apply when you save model settings.";
export const SETTINGS_PRESET_STORAGE_ERROR =
  "This browser could not save that preset.";

export function settingsPresetNameInvalid(maxLength: number): string {
  return `Enter a preset name of up to ${maxLength} characters.`;
}

// Accessible name for the composer message textarea (label-associated via
// aria-label since the control has only a placeholder).
export const A11Y_COMPOSER_INPUT = "Message";

// Searchable model picker (composer).
export const A11Y_MODEL = "Model";
export const A11Y_MODEL_SEARCH = "Search models";
export const MODEL_PICKER_SEARCH_PLACEHOLDER = "Search models…";
export const MODEL_PICKER_EMPTY = "No matching models";
export const MODEL_PICKER_DEFAULT = "default";

// Backend-authoritative composer context preview.
export function contextMeter(
  total: number,
  budget: number,
  available: number,
): string {
  return `${total.toLocaleString()} / ${budget.toLocaleString()} tokens · ${available.toLocaleString()} free`;
}

export function contextBreakdown(
  system: number,
  history: number,
  draft: number,
): string {
  return `system ${system.toLocaleString()} · history ${history.toLocaleString()} · draft ${draft.toLocaleString()}`;
}

export function contextOmitted(turns: number, messages: number): string {
  return `omitted ${turns} earlier turns (${messages} messages)`;
}

export function turnElapsed(seconds: number): string {
  return `${seconds}s elapsed`;
}

// Shown in place of a generative-UI block whose spec cannot be resolved. Such a
// spec renders as nothing at all, so without this the failure is invisible and
// reads as "the assistant chose not to draw anything".
export const SPEC_INVALID_LABEL = "UI SPEC COULD NOT BE RENDERED";

// specIssueLine renders one json-render validation issue for the reader. The
// code is the library's machine-readable identifier (root_not_found,
// missing_child, ...); elementKey is present only on the issues that name one.
export function specIssueLine(code: string, elementKey?: string): string {
  if (elementKey === undefined || elementKey === "") {
    return code;
  }

  return `${code}: ${elementKey}`;
}

// Per-chat MCP server picker (composer).
export const A11Y_CHAT_MCP = "MCP servers for this chat";
export const CHAT_MCP_TITLE = "MCP Servers";
export const CHAT_MCP_TRIGGER = "MCP";
export const CHAT_MCP_EMPTY = "No MCP servers configured.";

// Accessible name for the scrolling message transcript (role="log").
export const A11Y_CONVERSATION = "Conversation";

// Loading / empty / error state copy (brutalist all-caps, uppercased in CSS is
// avoided — the strings themselves are the display form).
export const STATE_LOADING = "LOADING…";
export const STATE_LOADING_CHATS = "LOADING CHATS…";
export const STATE_LOADING_HISTORY = "LOADING HISTORY…";
export const STATE_STARTING_CHAT = "STARTING CHAT…";
export const EMPTY_USERS = "NO USERS";
export const EMPTY_MCP = "NO MCP SERVERS";
export const EMPTY_CHATS = "NO CHATS YET";
export const SIDEBAR_SEARCH_PLACEHOLDER = "Search chats…";
export const LABEL_PIN = "Pin";
export const LABEL_UNPIN = "Unpin";
export const EMPTY_CONVERSATION = "Start a conversation.";
