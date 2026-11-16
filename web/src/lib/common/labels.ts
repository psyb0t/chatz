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

// Sidebar brand, repository link, and collapse toggle.
export const SIDEBAR_WORDMARK = "Chatz";
export const SIDEBAR_REPOSITORY_LABEL = "github.com/psyb0t/chatz";
export const SIDEBAR_REPOSITORY_URL = "https://github.com/psyb0t/chatz";
export const A11Y_SIDEBAR_COLLAPSE = "Collapse sidebar";
export const A11Y_SIDEBAR_EXPAND = "Expand sidebar";
// Mobile off-canvas drawer open/close (hamburger + in-drawer close button).
export const A11Y_SIDEBAR_OPEN = "Open sidebar";
export const A11Y_SIDEBAR_CLOSE = "Close sidebar";

// Sidebar chat-list date groups. Older chats are grouped by month.
export const GROUP_TODAY = "Today";
export const GROUP_YESTERDAY = "Yesterday";
export const GROUP_WEEK = "Previous 7 days";
export const GROUP_MONTH = "Previous 30 days";

// Sidebar admin nav labels (moved from the removed topbar).
// Accessible name for the single sidebar admin entry (a gear icon). It opens
// the /admin page, where Users/MCP/Readiness are tabs.
export const NAV_ADMIN_SYSTEM = "System";
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
export const A11Y_CHAT_DELETE = "Delete chat";
// Accessible name for the per-row "⋮" button that opens the edit/delete menu.
export const A11Y_CHAT_MENU = "Chat actions";
// Context-menu item labels.
export const LABEL_CHAT_EDIT = "Edit";
export const LABEL_SAVE = "Save";
export const LABEL_CLOSE = "Close";

// Per-chat model settings panel.
export const SETTINGS_TITLE = "Model settings";
export const A11Y_CHAT_SETTINGS = "Model settings";
export const SETTINGS_SECTION_REASONING = "Reasoning";
export const SETTINGS_SECTION_INSTRUCTIONS = "Instructions";
export const SETTINGS_SECTION_SAMPLING = "Sampling";
export const SETTINGS_SECTION_LENGTH = "Length and context";
export const SETTINGS_SECTION_ADVANCED = "Advanced";
export const SETTINGS_SECTION_PRESETS = "Presets";
export const SETTINGS_REASONING = "Reasoning effort";
export const SETTINGS_REASONING_AUTO = "Auto";
export const SETTINGS_REASONING_HINT =
  "How long the model thinks before answering. Off turns thinking off; Auto leaves it to the provider.";
export const SETTINGS_SYSTEM_PROMPT = "System prompt";
export const SETTINGS_SYSTEM_PROMPT_PLACEHOLDER =
  "Instructions for this chat, added after chatz's own";
export const SETTINGS_TEMPERATURE = "Temperature";
export const SETTINGS_TEMPERATURE_HINT =
  "Randomness. Lower is focused and repeatable, higher is more varied.";
export const SETTINGS_TOP_P = "Top-P";
export const SETTINGS_TOP_P_HINT =
  "Samples only from the most likely tokens whose probabilities add up to this.";
export const SETTINGS_TOP_K = "Top-K";
export const SETTINGS_TOP_K_HINT =
  "Samples only from this many of the most likely tokens.";
export const SETTINGS_FREQUENCY_PENALTY = "Frequency penalty";
export const SETTINGS_FREQUENCY_PENALTY_HINT =
  "Positive values discourage repeating the same words.";
export const SETTINGS_PRESENCE_PENALTY = "Presence penalty";
export const SETTINGS_PRESENCE_PENALTY_HINT =
  "Positive values push the model toward new topics.";
export const SETTINGS_MAX_OUTPUT = "Max output tokens";
export const SETTINGS_MAX_OUTPUT_HINT =
  "Upper limit on the reply length, reasoning included.";
export const SETTINGS_MAX_HISTORY = "Max history tokens";
export const SETTINGS_MAX_HISTORY_HINT =
  "How much of the conversation is sent with each message. Default 100,000.";
export const SETTINGS_SEED = "Seed";
export const SETTINGS_SEED_HINT =
  "The same seed and settings make replies more repeatable, where the provider supports it.";
export const SETTINGS_STOP = "Stop sequences";
export const SETTINGS_STOP_HINT =
  "The reply ends when the model writes one of these. Up to 4.";
export const SETTINGS_STOP_PLACEHOLDER = "Type and press Enter";
export const SETTINGS_UNSUPPORTED = "Not supported by this model";
export const SETTINGS_RESET = "Reset all";
export const SETTINGS_SAVING = "Saving…";
export const SETTINGS_SAVED = "Saved";
export const SETTINGS_NEEDS_CHAT = "Send a message first to save settings.";
export const SETTINGS_REASONING_UNSUPPORTED =
  "This model has no reasoning controls.";
export const SETTINGS_PRESET = "Preset";
export const SETTINGS_PRESET_NONE = "Choose a preset";
export const SETTINGS_PRESET_NAME = "Preset name";
export const SETTINGS_PRESET_SAVE = "Save preset";
export const SETTINGS_PRESET_DELETE = "Delete preset";
export const SETTINGS_PRESET_HINT =
  "Presets stay in this browser. Choosing one applies it to this chat.";
export const SETTINGS_PRESET_STORAGE_ERROR =
  "This browser could not save that preset.";

export function settingsPresetNameInvalid(maxLength: number): string {
  return `Enter a preset name of up to ${maxLength} characters.`;
}

// Reasoning effort names as the UI shows them.
const EFFORT_LABELS: Record<string, string> = {
  none: "Off",
  minimal: "Minimal",
  low: "Low",
  medium: "Medium",
  high: "High",
  xhigh: "Extra high",
  max: "Max",
};

export function effortLabel(effort: string): string {
  return EFFORT_LABELS[effort] ?? effort;
}

export const A11Y_REASONING = "Reasoning effort";

// Thinking block header.
export const THINKING_LIVE = "Thinking…";
export const THINKING_DONE = "Thought";
const SECONDS_PER_MINUTE = 60;

export function thinkingDuration(seconds: number): string {
  if (seconds < 1) {
    return "Thought for under a second";
  }

  if (seconds < SECONDS_PER_MINUTE) {
    return `Thought for ${Math.round(seconds)}s`;
  }

  const minutes = Math.floor(seconds / SECONDS_PER_MINUTE);
  const rest = Math.round(seconds % SECONDS_PER_MINUTE);

  return `Thought for ${minutes}m ${rest}s`;
}

// Tool call card.
export const TOOL_ARGS = "Arguments";
export const TOOL_RESULT = "Result";
export const TOOL_ERROR = "Error";
const MS_PER_SECOND = 1000;

export function toolTitle(
  tool: string,
  done: boolean,
  failed: boolean,
): string {
  if (!done) {
    return `Running ${tool}`;
  }

  return failed ? `${tool} failed` : `Ran ${tool}`;
}

export function toolElapsed(ms: number): string {
  if (ms < MS_PER_SECOND) {
    return `${Math.round(ms)}ms`;
  }

  return `${(ms / MS_PER_SECOND).toFixed(1)}s`;
}

// Composer.
export const COMPOSER_PLACEHOLDER = "Message chatz…";
export const COMPOSER_HINT = "Enter to send, Shift+Enter for a new line";
export const A11Y_SEND = "Send";
export const A11Y_STOP = "Stop";
export const COMPOSER_NO_MODELS = "No models";
export const COMPOSER_NO_UPSTREAM =
  "No upstream configured. Add one to CHATZ_UPSTREAMS to chat.";

// Assistant message actions.
export const A11Y_COPY = "Copy reply";
export const A11Y_COPY_CODE = "Copy code";
export const MD_IMAGE_LABEL = "image";
export const LABEL_COPIED = "Copied";

// Empty conversation.
export const EMPTY_TITLE = "What's on your mind?";
export const EMPTY_SUGGESTIONS: readonly string[] = [
  "Explain how DNS resolution works, step by step",
  "Write a bash script that backs up a directory with timestamps",
  "Compare REST and gRPC for internal services",
  "Summarize the trade-offs of SQLite versus Postgres",
];

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

// Loading / empty / error state copy. The strings are the display form; CSS
// does not change their case.
export const STATE_LOADING = "Loading…";
export const STATE_LOADING_CHATS = "Loading chats…";
export const STATE_LOADING_HISTORY = "Loading history…";
export const STATE_STARTING_CHAT = "Starting chat…";
export const EMPTY_USERS = "No users";
export const EMPTY_MCP = "No MCP servers";
export const EMPTY_CHATS = "No chats yet";
export const SIDEBAR_SEARCH_PLACEHOLDER = "Search chats…";
