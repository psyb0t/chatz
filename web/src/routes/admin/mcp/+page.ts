// Admin MCP screen is a client-only SPA route (session-gated, admin-guarded in
// the page), so it can't be crawled/prerendered. Served via the static
// adapter's SPA fallback and hydrated client-side, like the chat route.
export const prerender = false;
