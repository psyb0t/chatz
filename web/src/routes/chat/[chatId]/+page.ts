// The chat id is only known at runtime (client-side SPA navigation), so this
// parameterized route can't be crawled/prerendered. It is served by the static
// adapter's SPA fallback (index.html) and hydrated client-side.
export const prerender = false;
