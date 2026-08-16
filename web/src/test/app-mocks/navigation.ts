// Test stub for SvelteKit's $app/navigation. The real goto triggers router
// navigation; under vitest we record calls so the conversation store's
// navigate-on-create path can be asserted without a router.
export const gotoCalls: string[] = [];

export function goto(url: string): Promise<void> {
  gotoCalls.push(url);

  return Promise.resolve();
}
