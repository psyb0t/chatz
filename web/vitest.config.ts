import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";
import { defineConfig } from "vitest/config";
import path from "node:path";

// Unit-test config, separate from vite.config.ts (which wires the full SvelteKit
// plugin + dev proxy). Here we only need the Svelte compiler so runes `.svelte`
// and `.svelte.ts` files compile, a jsdom DOM, and a manual `$app` alias (the
// kit plugin normally provides it; we mock `$app/*` in tests). `#lib` resolves
// through the `imports` field in package.json.
export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  resolve: {
    alias: {
      $app: path.resolve("./src/test/app-mocks"),
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.{test,spec}.{js,ts}"],
    setupFiles: ["src/test/setup.ts"],
  },
});
