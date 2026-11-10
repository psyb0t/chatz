# Web app

- `web/` is a SvelteKit static SPA (Svelte 5, TypeScript). The production `Dockerfile` builds it and copies `web/build` into `internal/pkg/webassets/dist`, which `go:embed` serves. Only `dist/index.html`, a placeholder, is committed.
- pnpm only, never npm. Installs use `--frozen-lockfile --ignore-scripts`. Dependency changes go through `make web-pkg-add`, `web-pkg-update`, `web-pkg-upgrade` or `web-pkg-remove`, which first move `minimumReleaseAge` in `web/pnpm-workspace.yaml` to a 7-day floor.
- `make lint-web` runs `prettier --check .` and `svelte-check`; `make format-web` applies prettier. `make test-web` runs vitest.
- Node modules live in the `chatz-web-node-modules` Docker volume, not in a host `web/node_modules`.
- The generative UI catalog is closed. A component change follows the lockstep workflow in `web/src/lib/render/README.md` and ends with `make genui-prompt`. Props stay declarative: no arbitrary HTML, callbacks or side-effecting URLs.
- Shared UI primitives are in `web/src/lib/components/ui/` (see its README). Test IDs are in `web/src/lib/common/test-ids.ts`; the browser tests in `tests/api` select by them.
