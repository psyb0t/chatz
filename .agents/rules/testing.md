# Testing

- Every test target runs in the dev container. Do not run host `go test` in project automation.
- Tiers and their build tags:
  - `make test`: Go unit tests with `-race` and no build tag, plus `make test-web` (vitest). No Docker socket, no external services.
  - `make test-integration`: `-tags integration` over `./tests/...` (`tests/integration`, `tests/boot`) with a testcontainers Postgres.
  - `make test-coverage`: the integration-tagged coverage gate over every package, then the web tests. This is the CI test command.
  - `make test-api`: `-tags api`, builds the production image and drives it with a headless browser against Postgres and SQLite, using `tests/fakeupstream` and `tests/mcpserver`. `API_RUN`, `API_DB_DRIVERS`, `API_PARALLEL`, `API_TIMEOUT` and `API_REAL` narrow or extend it.
  - `make test-real`: `-tags real` over `tests/real`, against the providers in `.env`. Opt-in and never in CI.
- `MIN_TEST_COVERAGE` in `Makefile` is a ratchet. Raise it when the measured number clears the next step; never lower it to make a build pass.
- Under `go test` every upstream resolves to elelem's scripted test double, so no test reaches a real provider by accident. Only `CHATZ_FORCE_REAL_LLM=true`, set in your own `.env` for `make test-real`, turns that off.
- Tests that touch the database, HTTP server, MCP manager or turn persistence belong in `tests/integration` with the shared `tests/testinfra` harness. A change to an end-to-end user flow gets a driver in `tests/api`.
- Run the narrowest target first, then `make lint` and `make test-coverage` before handing off. Run `make test-api` after changing a user flow, the SSE stream or the generative UI renderer.
