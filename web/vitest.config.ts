import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";
import { defineConfig } from "vitest/config";
import path from "node:path";

// Unit-test config, separate from vite.config.ts (which wires the full SvelteKit
// plugin + dev proxy). Here we only need the Svelte compiler so runes `.svelte`
// and `.svelte.ts` files compile, a jsdom DOM, and manual `$lib` / `$app`
// aliases (the kit plugin normally provides these; we mock `$app/*` in tests).
export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  resolve: {
    alias: {
      $lib: path.resolve("./src/lib"),
      $app: path.resolve("./src/test/app-mocks"),
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.{test,spec}.{js,ts}"],
    setupFiles: ["src/test/setup.ts"],
  },
});
