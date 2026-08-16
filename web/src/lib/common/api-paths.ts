// Server API paths used by the hand-rolled SSE stream client (openapi-fetch
// handles JSON ops; SSE is fetched directly, so these paths are spelled here
// rather than inferred from the openapi client).
export const API_PATH_CHATS = "/api/v1/chats";

// chatPath builds the per-chat path for continue (POST) and get (GET).
export function chatPath(chatId: string): string {
  return `${API_PATH_CHATS}/${chatId}`;
}
