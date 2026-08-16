import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig, loadEnv } from "vite";

const DEFAULT_API_TARGET = "http://localhost:8080";

export default defineConfig(({ mode }) => {
  // Forward API + health calls to a locally-running backend so `pnpm dev` works
  // against the real server. Override the target with CHATZ_API_TARGET.
  const env = loadEnv(mode, ".", "");
  const target = env.CHATZ_API_TARGET || DEFAULT_API_TARGET;

  return {
    plugins: [sveltekit()],
    // @json-render/svelte ships pre-bundled ESM with Svelte 5 runes; excluding it
    // from Vite's dep pre-bundling avoids double-compiling its .svelte files (per
    // the json-render svelte-chat example).
    optimizeDeps: {
      exclude: ["@json-render/svelte"],
    },
    server: {
      proxy: {
        "/api": { target, changeOrigin: true },
        "/healthz": { target, changeOrigin: true },
      },
    },
  };
});
