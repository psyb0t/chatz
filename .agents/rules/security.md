# Security

- Never commit secrets. `.env` and `.env.*` (except `.env.example`) are gitignored. Upstream keys are referenced by name through `apiKeyEnv` in `CHATZ_UPSTREAMS`, never written into the JSON.
- MCP HTTP headers and stdio environment maps are sealed with AES-256-GCM using `CHATZ_SECRETS_KEY` (`internal/pkg/secrets`). Without the key, storing one fails with `secrets.ErrNotConfigured`. Never add a plaintext fallback.
- The `Authorization` MCP header comes back masked as `<scheme> [SECRET]` in admin responses; the mask and restore logic is in `internal/pkg/http/server/handler_mcp.go`. Keep any new credential field masked the same way.
- Sessions are opaque tokens in the `chatz_session` cookie (HttpOnly, SameSite Lax). Only the SHA-256 hash is stored. The cookie is not marked Secure, so TLS belongs on a reverse proxy in front of chatz.
- Admin endpoints (users, upstreams, readiness, MCP servers and import) must call `requireAdmin`. A chat's MCP server toggle is a preference, not a security boundary.
- The admin can configure stdio MCP servers, which run commands inside the container. The single admin is trusted; do not widen who can create them.
- The metrics (`:9091`) and pprof (`:6060`) listeners are internal. Do not publish them in `docker-compose.yml` or the README examples.
- The image runs as a non-root user and works with a read-only root filesystem. Keep `docker-compose.yml` hardened: dropped capabilities, `no-new-privileges`, resource limits and loopback-bound ports.
- Debug logs can contain chat text. Redaction in `internal/pkg/logging/redact.go` covers secret-shaped keys and values only.
