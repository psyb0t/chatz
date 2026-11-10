# API and generated code

- `api/api.yml` is the contract. A change to a path, parameter, schema or status starts there, then `make generate` regenerates `internal/pkg/http/api/api.gen.go` and `pkg/http/api/client/client.gen.go`. `make web-gen-api` regenerates `web/src/lib/api/schema.d.ts`. Then implement the handler against the strict interface.
- The spec's server URL is `/v1`, but `internal/pkg/http/server/server.go` mounts the generated routes at `apiGroupPrefix` (`/api/v1`). `/healthz` is registered only at the root.
- Handlers live in `internal/pkg/http/server/handler_*.go`, one file per resource. They validate transport input and delegate to `internal/pkg/core/...` or `internal/pkg/mcp`. Domain types convert to API types in each package's `to_api.go`.
- Errors use the aichteeteapee envelope `{code, message, details}` built in `responses.go`. `code` is an upper snake case string from aichteeteapee. Admin-only handlers call `requireAdmin` first.
- Chat streams are SSE with Anthropic-style frames plus the `chat_status` progress event. A new frame or status value changes `api/api.yml`, `internal/pkg/core/chats` and `web/src/lib/api/sse-events.ts` together.
- Every generator is declared in its output package's `gen.go` (`//go:generate`), so `go generate ./...` finds it. A new generated package adds a `gen.go`, never a line in `scripts/make/generate.sh`.
- The GenUI prompt chain stays in lockstep: `web/src/lib/render/catalog.ts`, `web/src/lib/common/render.ts` and the registry feed `web/scripts/gen-ui-instructions.mjs`, and `make genui-prompt` writes `internal/pkg/core/chats/prompts/genui_instructions.gen.txt`. `web/src/lib/render/README.md` has the change workflow.
