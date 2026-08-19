// App route paths. Referenced by the layout guard and navigation so no route
// string is spelled inline.
export const ROUTE_HOME = "/";
export const ROUTE_SETUP = "/setup";
export const ROUTE_LOGIN = "/login";
export const ROUTE_ADMIN = "/admin";
export const ROUTE_ADMIN_USERS = "/admin/users";
export const ROUTE_ADMIN_MCP = "/admin/mcp";
export const ROUTE_ADMIN_READINESS = "/admin/readiness";

const ROUTE_CHAT_PREFIX = "/chat/";

// chatRoute builds the shareable URL for a specific chat.
export function chatRoute(chatId: string): string {
  return `${ROUTE_CHAT_PREFIX}${chatId}`;
}
