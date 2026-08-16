export const CHAT_TURN_STATUS_CONNECTING = "connecting";
export const CHAT_TURN_STATUS_WAITING_FOR_FIRST_TOKEN = "waiting_first_token";
export const CHAT_TURN_STATUS_STREAMING = "streaming";
export const CHAT_TURN_STATUS_RUNNING_TOOL = "running_tool";
export const CHAT_TURN_STATUS_RETRYING = "retrying";

export const CHAT_TURN_STATUSES = [
  CHAT_TURN_STATUS_CONNECTING,
  CHAT_TURN_STATUS_WAITING_FOR_FIRST_TOKEN,
  CHAT_TURN_STATUS_STREAMING,
  CHAT_TURN_STATUS_RUNNING_TOOL,
  CHAT_TURN_STATUS_RETRYING,
] as const;

export type ChatTurnStatus = (typeof CHAT_TURN_STATUSES)[number];

const CHAT_TURN_STATUS_LABELS: Record<ChatTurnStatus, string> = {
  [CHAT_TURN_STATUS_CONNECTING]: "Connecting to model…",
  [CHAT_TURN_STATUS_WAITING_FOR_FIRST_TOKEN]: "Waiting for first token…",
  [CHAT_TURN_STATUS_STREAMING]: "Streaming response…",
  [CHAT_TURN_STATUS_RUNNING_TOOL]: "Running tool…",
  [CHAT_TURN_STATUS_RETRYING]: "Retrying with fallback model…",
};

export function isChatTurnStatus(value: string): value is ChatTurnStatus {
  return CHAT_TURN_STATUSES.includes(value as ChatTurnStatus);
}

export function chatTurnStatusLabel(status: ChatTurnStatus): string {
  return CHAT_TURN_STATUS_LABELS[status];
}
