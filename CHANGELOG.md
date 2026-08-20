# Changelog

All notable changes to chatz are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-08-20

### Changed

- Restructured the test suite into three tiers, one per build tag. `make test`
  runs the Go unit tests only, with no build tag and no external infrastructure.
  `make test-integration` runs the `integration`-tagged tests that call the
  generated handlers in process over a real Postgres via testcontainers; the
  former `tests/auth`, `tests/db`, `tests/httpserver`, and `tests/mcp` packages
  are merged into one `tests/integration` package that boots a single container.
  `make test-api`, renamed from `make test-e2e`, builds the full app image and
  drives it end to end. Its build tag moved from `e2e` to `api`, its runner
  variables from `E2E_*` to `API_*`, and its fixture variables from `CHATZ_E2E_*`
  to `CHATZ_API_*`. Update any local scripts or CI that call `make test-e2e` or
  set those variables.
- Updated the servicepack framework from v1.3.1 to v1.6.4. The coverage gate now
  runs the framework's own measurement, which excludes generated code, command
  mains, and mocks from the denominator.
- Raised the coverage floor (`MIN_TEST_COVERAGE`) from 60 to 85.
- Bumped the Go directive to 1.26.6.

### Added

- In-process boot test tier (`tests/boot`) that runs the http-server service's
  real startup path in process: config parse, database connect, upstream
  discovery, MCP connect, server assembly, serve, and shutdown, all against a
  testcontainer Postgres. This credits the boot wiring that only the api tier
  exercised before. Statement coverage rose from 78.9% to 87.5%.

## [0.4.0] - 2026-08-19

### Changed

- The sidebar's three admin links (Users, MCP, Readiness) collapse into one gear
  icon labelled "System". It opens a new `/admin` page where those three are
  tabs, so the sidebar stays compact and the admin screens share one chrome. The
  existing routes (`/admin/users`, `/admin/mcp`, `/admin/readiness`) are
  unchanged, so deep links still work; bare `/admin` lands on the Users tab.

## [0.3.0] - 2026-08-19

### Removed

- **Breaking.** Chat pinning is gone. A row's edit and delete now live behind a
  hover-revealed three-dot menu; editing renames the chat inline, so the list
  stays one line per chat. Chats sort by newest activity only.
  - **API.** Removed `PUT/DELETE /chats/{chatId}/pin` and the `pinnedAt` field
    from `ChatSummary`. The only consumer is the SPA generated from this spec.
  - **Schema.** Migration `0000020` drops `chats.pinned_at` and rebuilds the
    sidebar ordering index without it (`idx_chats_user_active_updated`). The down
    migration restores the column and the pinned-order index, not the data.

### Fixed

- A generative-UI reply whose `/root` named an element the model never defined
  (it set `/root` to `main` while keying its container `stackMain`) rendered as a
  blank block. The renderer now recovers the real root when it is unambiguous
  (`web/src/lib/render/recover-root.ts`); genuinely ambiguous specs still surface
  a visible error instead of drawing nothing.
- A generative-UI reply where the model dropped the closing brace on a long
  element line lost that element and every one after it, leaving an empty spec.
  The fence parser now repairs a single-brace truncation and applies the element
  (`web/src/lib/render/fence.ts`).

### Changed

- Prompt rule guiding the model to set `/root` to the exact key of its top-level
  element, cutting the dangling-root failure at the source.

## [0.2.0] - 2026-08-19

### Added

- `chatz migrate` applies pending database migrations and exits without starting
  the HTTP server, for deploys that move the schema before the new binary takes
  traffic. `make migrate` already invoked this subcommand, but nothing registered
  it, so the target failed.

### Removed

- **Breaking.** Chat projects and chat archiving are gone. A chat is now kept or
  deleted — nothing else. The sidebar exposes a single delete control revealed on
  hover; pin and rename remain.
  - **API.** Removed `GET/POST /projects`, `PATCH/DELETE /projects/{projectId}`,
    `PUT/DELETE /chats/{chatId}/archive`, and `PUT/DELETE /chats/{chatId}/project`;
    dropped the `archived` and `projectId` query parameters from `GET /chats` and
    the `projectId` and `archivedAt` fields from `ChatSummary`. The only consumer
    is the single-page app generated from this same spec, updated in this commit.
  - **Schema.** Migration `0000018` drops `chats.project_id` and `chats.archived_at`
    (and their indexes, rebuilding the pinned-order index without the
    `archived_at IS NULL` predicate); migration `0000019` drops the `projects`
    table. Both are reversible — the down migrations restore the columns, indexes,
    and table structure, not the data.

### Changed

- Deleting the chat currently on screen navigates away from it. Previously the
  route still named the deleted chat, leaving a dead view that failed on reload.
- Container builds stamp the binary's own name and revision (`-X main.appName`,
  `-X main.buildCommit`) alongside the existing `buildinfo` version and commit.
  Log lines previously identified the process as `servicepack`.

### Fixed

- The embedded single-page app was a stale build carrying only 12 of the 26
  generative-UI components, so an assistant reply containing a chart, gauge,
  heatmap, treemap, sparkline, log viewer or network graph rendered as nothing at
  all. The embedded bundle now matches the current frontend.
- The context-usage meter under the composer no longer disappears while typing.
  Its value is kept while a refresh is in flight and cleared only when the chat
  changes.
- Two CSS custom properties were referenced but never declared, so the rules
  using them silently did nothing: the "Interrupted response" label renders muted
  and small as intended, and the funnel chart's detail list gets its indent.
- `make test-coverage` writes `coverage-percent.txt`, which the release pipeline
  reads to render the coverage badge. Only the superseded framework script
  produced it, so the badge had no source.

## [0.1.0] - 2026-08-16

### Changed

- LLM upstreams now explicitly isolate their Elelem SDK clients from ambient
  provider environment configuration. A keyless local endpoint cannot inherit
  another upstream's process-level credential or custom header.
- The repository generator is now a Go tool: `cmd/repogen` is registered in
  `go.mod`'s `tool` block and invoked via `go tool repogen` from the
  repositories `gen.go`, instead of `go run ../../../../cmd/repogen`. It builds
  from the pinned, vendored toolchain like every other generator, with no
  fragile relative path.
- Updated the bundled servicepack framework from v1.2.20 to v1.3.1 (framework
  make scripts, Dockerfiles, service-manager, runner, and app plumbing).

## [1.15.0] - 2026-08-13

### Added

- SQLite persistence is available through `CHATZ_DB_DRIVER=sqlite`. It uses a
  pure-Go driver, an embedded SQLite baseline, foreign keys, WAL mode, a bounded
  busy timeout, and a single persistent `/data` volume while the app filesystem
  remains read-only. Postgres remains the default and its migration history is
  unchanged.
- Chat organization now supports projects, pinned chats, archive/restore,
  soft deletion, literal title search, and project filtering. The API adds
  `GET/POST /projects`, `PATCH/DELETE /projects/{projectId}`, and the
  archive/pin/project chat actions.
- A streamed assistant response is checkpointed as an explicitly incomplete
  message. Reloading after an interrupted stream preserves the visible output
  without replaying it into later model context, and the UI can retry the
  original prompt as a fresh durable turn.
- `POST /chats/{chatId}/context-preview` reports the exact history selection
  and token budget that the next unsent message would use.
- Upstream configuration can define per-model aliases, context and output
  limits, capabilities, expected first-token latency, and integer currency
  prices. `CHATZ_DEFAULT_MODEL` selects an advertised default model.
- Admins can inspect redacted upstream health at `GET /upstreams` and the
  application readiness snapshot at `GET /admin/readiness`, including database
  migration state and backup freshness.
- The public `pkg/rebound` package provides context-aware bounded exponential
  retry with non-retryable error registration and pre/post-attempt hooks.

### Changed

- The build, development, and embedded Servicepack toolchains now pin Go
  1.26.6, which includes the available standard-library security fixes.

- The embedded Servicepack framework now uses v1.3.1. Its context-aware
  runner, application lifecycle, service manager, developer tooling, and
  documentation are synchronized while Chatz's Docker-first build and test
  overrides remain intact.
- All direct Psyb0t modules now use their current releases. In particular,
  common-go v0.7.1 supplies the shared SQLite migrator, so Chatz no longer
  keeps a divergent local migration implementation or registers SQLite twice.
- Outbound provider requests now have configured connection, first-token, total
  turn, and per-upstream concurrency bounds. The chat UI reports actual stream
  and tool lifecycle states while a turn is running.
- MCP server records expose redacted connection timing, failure, and tool-count
  status to administrators; credentials remain masked.
- Production image builds are offline from the checked-in Go vendor tree,
  embed build version/revision metadata, and release tags publish multi-arch
  Docker images with provenance and SBOM attestations.
- Browser E2E runs each selected flow against both Postgres and SQLite, with
  isolated ports and teardown of partially started fixture containers.

## [1.14.4] - 2026-08-08

### Changed

- `docs/PLAN.md` now marks where the plan diverged from what shipped, instead of
  reading as a description of the system. It called for `common-go/http`'s
  `CreateDefaultAPIMiddleware` + `BearerAuth` and a vendored SSE flush template;
  neither shipped. Auth is chatz's own cookie-based `sessionAuth()` in
  `internal/pkg/http/server/server.go`, and streaming uses oapi-codegen's stock
  `text/event-stream` codegen — `internal/pkg/http/api/config.yaml` has no
  `output-options.user-templates` entry.
- Logging references updated: `slog-configurator` → `slogging`, `AddHandler` →
  `AddSink` (which is the real function), and the ctx logger is
  `ctxscope.GetLogger`.
- The plan's `GetCtxWithLogger` is documented as **moved, not imaginary** — it
  was real in `common-go/slogging`, which is now only the Loki sink. Nothing
  installs a logger onto a context any more; `ctxscope` carries attributes and
  derives the logger on request.
- `internal/pkg/mcp/README.md` and `web/src/lib/components/ui/README.md` synced
  to the same names.

## [1.14.3] - 2026-08-08

### Changed

- Log scope now comes from `github.com/psyb0t/ctxscope` instead of
  `github.com/psyb0t/common-go/scope`. That package was extracted into its own
  module so it can ship on its own schedule rather than one shared with a module
  that also carries gorm, echo, NATS and the Temporal SDK. The API is unchanged
  apart from the package name — every call site moved from `scope.X` to
  `ctxscope.X`.
- `common-go` remains a dependency for `ai/claudecode`, `cache`, `constants`,
  `db`, `errors`, `http`, `llm`, `mq`, `slogging` and `types`.
- No exported signature mentions a scope type, so the public API is untouched.

## [1.14.2] - 2026-08-08

Dependency bump. No behaviour changed.

### Changed

- `slogging` v1.6.1 → v1.7.0, which rebuilt that module's handler API:
  `MultiWriterHandler` is gone, the fan-out moved to a `handlers` package, and
  `AddHandler` split into **`AddSink`** (add a destination) and **`SetOutput`**
  (change where the process prints, keeping the destinations).
- The `chatz.log` file sink now calls `slogconf.AddSink` — a rename at this call
  site, and the correct one of the two: the file is an additional destination
  alongside the console, not a replacement for it. `SetOutput` would have
  silenced stdout.
- The redaction wrapper is unaffected. It goes through `slog.SetDefault`
  directly rather than through this package, and the file handler carries its
  own `RedactingHandler`, so both paths stay masked independently — which is
  what the comment in `internal/pkg/logging/redact.go` describes, now naming
  `AddSink`.

## [1.14.1] - 2026-08-08

Dependency rename. No behaviour changed.

### Changed

- `github.com/psyb0t/slog-configurator` was renamed upstream to
  `github.com/psyb0t/slogging`, with the configurator now at
  `slogging/slogconf`. The blank imports in `cmd/main.go` and the four test
  suites follow it, as does the `AddHandler` call in
  `internal/pkg/logging/file_sink.go` and the vendor tree. Every exported name
  is unchanged, so the stdout/stderr split, the redaction wrapper and the
  `chatz.log` file sink behave exactly as before.
- The comments in `internal/pkg/logging/` and `cmd/init.go` that explain this
  package's init ordering — redaction and the file sink must be stacked *after*
  the configurator's blank-import init — now name `slogconf` too. They describe
  a load-bearing constraint, so a stale name there is worse than a stale import:
  the compiler checks the import, nothing checks the explanation.

## [1.14.0] - 2026-08-06

Streaming library rename that reaches the wire, plus a browser-container leak
that had been quietly poisoning the e2e suite.

### Changed

- **Breaking (SSE clients).** The `message_start` event's `conversation_id`
  field is now `stream_id`, following essessey v0.7.0. That library is not
  chat-specific — it carries content blocks for whatever the caller is doing —
  so naming the identifier after one caller's domain was wrong. chatz keeps its
  own "conversation" vocabulary internally and maps it onto the stream id at
  the boundary; only the wire key moved. The web client
  (`web/src/lib/api/sse-events.ts`), the OpenAPI description in `api/api.yml`
  and the generated clients are updated in this same release, so the shipped UI
  is unaffected. Anything reading the SSE stream directly must read the new
  key. No Go type in `pkg/http/api/client` changed.

- **The e2e browser gets five minutes to become ready, not two.** It boots in
  about 20 seconds on an idle host, but it starts Xvfb, Chrome and a Python API
  server, and the suite brings up several full stacks at once. Under load that
  startup was measured past the old deadline, and overshooting surfaced as
  `wait until ready: context deadline exceeded`, which reads like a broken
  image rather than a slow one. `BrowserOptions.BootTimeout` overrides it.

### Fixed

- **A browser container that never became ready leaked, and took the whole
  suite's teardown down with it.** `SetupBrowser` discarded the container that
  testcontainers returns *alongside* its start error — upstream returns a
  non-nil container there specifically so the caller can destroy it. The orphan
  stayed attached to the per-test Docker network, so `E2EStack.Teardown` then
  failed with `network ... has active endpoints`, and the leak survived into
  the next run, leaving less headroom and causing more leaks.

  The failing assertion was in teardown, nowhere near the cause, so it read as
  flake. Two full e2e batteries died to it.
  `tests/testinfra/browser_test.go` now reproduces it in about a second by
  forcing the failure path with a very short boot timeout and asserting the
  network can still be removed afterwards.

- `abandonBrowser` no longer dereferences a nil container. `GenericContainer`
  returns `(nil, err)` when it fails before creating anything, which would have
  turned a failed start into a panic.

## [1.13.6] - 2026-08-06

Servicepack framework update carrying two concurrency fixes to the service
runner chatz runs on.

### Fixed

- **A shutdown racing a service error could kill the process.** The app's
  deferred channel close, goroutine wait and stop ran in the wrong order
  because Go runs defers LIFO, so the error channel was closed while the
  goroutine sending on it was still live. A stop signal arriving while the
  service manager was unwinding toward an error panicked the process with
  `send on closed channel` — during what should be a graceful shutdown, with
  nothing to recover it.

- **Three or more services failing simultaneously hung the process.** The
  service manager's error channel has capacity 1 and is read exactly once, but
  the send was blocking, so every failure after the first parked forever and
  the manager's wait never returned: no error, no exit. Two simultaneous
  failures did *not* hang, which is why this went unnoticed. The send is now
  non-blocking; later errors are logged, matching the contract that the first
  non-allowed failure is what stops everything.

Both fixes arrive with regression tests that were verified by reintroducing the
original code — the old ordering panics and the old send hangs.

### Changed

- Updated servicepack from v1.2.18 to v1.2.20. Beyond the two fixes above:
  framework Docker images pin their base images by digest and inject the
  project's own name into the binary, framework script diagnostics moved to
  stderr, and the framework updater no longer builds its rsync exclude list
  through `eval` — a metacharacter in `.servicepackupdateignore` used to run as
  a command during an update.

## [1.13.5] - 2026-08-06

Servicepack framework update plus two local fixes.

### Changed

- Updated the servicepack framework from v1.2.17 to v1.2.18. Framework files
  only; no behaviour change to chatz. The update scopes its module-path rewrite
  to the files rsync actually delivered instead of walking the whole working
  tree — which previously meant it edited Go files in ignored scratch
  directories that the update had never touched.

### Fixed

- **A heartbeat test reported a bug that did not exist.** It called `Touch()` in
  a loop with a fixed 30ms sleep against a 30ms idle interval, then asserted no
  heartbeat had fired. On a loaded machine a sleep overruns its interval, the
  idle clock legitimately elapses, and the heartbeat fires correctly — so the
  test failed while the code under test was right. It now measures the gaps the
  loop actually achieved and asserts the invariant only on a run that kept up,
  retrying when the scheduler stalls and failing loudly if it never does.

- `.dockerignore` now excludes the whole root `docs/` tree and `.agents`, and
  the build backup directory. Note the markdown rule that matters here: only
  root-level and `docs/` markdown is excluded. `*.md` does not cross a `/`, so
  nested markdown is deliberately kept — packages in this repo embed resource
  files, and a blanket recursive markdown exclude would break those builds
  silently.

## [1.13.4] - 2026-08-06

servicepack framework updated v1.2.6 → v1.2.17. No behaviour change.

### Changed

- **Framework files are eleven versions newer.** The update touched five
  files, every one of them framework-owned; nothing chatz owns was modified and
  no dependency moved:
  - `internal/pkg/service-manager/` — the whole reason for the update. chatz
    was carrying two generations of flaky tests: the integration suite fixed
    upstream in v1.2.9, and the unit-test races fixed in v1.2.16
    (`TestServiceManager_Stop` and `TestServiceManager_Run` each used a
    `time.Sleep` in place of the condition they depended on). One of those is
    what failed a chatz release gate earlier, deep in a package chatz doesn't
    own.
  - `scripts/make/servicepack/` — `do_update.sh` and `test_coverage.sh`.
  - `servicepack.version`.

  Verified after merging: build, `go vet`, `make lint` (0 issues), the
  service-manager package 5× under `-race`, and the full battery including the
  browser e2e suite — all green. The upstream zero-sleep probe also passes on
  chatz's copy, so the flake is genuinely gone here and not merely fixed
  upstream.

## [1.13.3] - 2026-08-06

Guards chatz's own files against the pending servicepack framework update.
No code change.

### Changed

- **`.servicepackupdateignore` now covers everything servicepack ships that
  chatz owns or must not have.** The framework update is an rsync of the
  servicepack tree over this one, so a file it ships and chatz lacks is
  **added**, not merely updated — and chatz is a **private** repo while
  servicepack is public. Without these entries the update would have:
  - added `.github/workflows/mirror-and-archive.yml`, which force-pushes the
    repo to public GitLab and Codeberg and saves it to the Wayback Machine.
    On a private repo that is a disclosure, and the archive does not forget.
  - added `.github/workflows/issue-pull.yml`, a cron relaying issues from
    mirrors that do not exist.
  - added `.agents/`, the agent skill describing *servicepack*, which would
    tell any agent reading this repo it is looking at the framework.
  - overwritten `.github/workflows/pipeline.yml` (chatz sets
    `has_codegen: true` for the gorm/OpenAPI/GenUI codegen drift gate),
    `.gitleaks.toml`, `.dockerignore` and `.github/dependabot.yml`, each of
    which chatz has tuned for its own tree.

  Verified by dry-running the real update against a clean clone of the
  framework tag: 40 files transfer, 7 have content changes, all of them
  framework internals, and nothing chatz owns is touched.

### Fixed

- **Two `internal/pkg/mcp` tests raced the retry timer they asserted on.**
  `TestConnectAsync_FailureSchedulesExactlyOneRetry` and
  `TestConnectAsync_ManualReconnectCancelsPendingRetry` waited for the server
  to reach `StateFailed` and then read `mgr.retries` immediately — but
  `applyFailure` sets the failed status via `setStatusIfCurrent` and only
  *then* calls `scheduleRetry`, so the state is observable before the timer is
  armed. The gap is small enough to pass locally indefinitely and to lose under
  a loaded full-suite run, which is how it surfaced: both failed in a release
  gate that had never flagged them.

  They now wait on the timer itself, which is the condition they actually
  assert. Confirmed by widening the gap to 150ms in `applyFailure`: the old
  form fails, the new one passes. The delay was reverted; only the tests
  changed.

## [1.13.2] - 2026-08-05

Real-provider coverage for the non-streaming transport. Test-only.

### Added

- **`tests/real/` now exercises `elelem.WithStreaming(false)` against live
  upstreams**, on both an OpenAI-shaped and an Anthropic-shaped endpoint. That
  path shipped in elelem v0.4.0 covered by unit tests and fixtures only — and a
  scripted driver cannot disagree with itself, so only a real provider can prove
  the two transports agree. Two tests:
  - `TestRealUpstreamsStreamingOffMatchesStreamingOn` — same prompt both ways.
    Asserts `OnDelta` fires on the non-streaming path (the reason
    `Driver.Complete` takes a delta callback rather than returning a message),
    that token accounting is populated either way, and that the finish reason
    matches across transports. Answer TEXT is deliberately not compared — a
    model may word two replies differently.
  - `TestRealUpstreamsToolCallMatchesAcrossTransports` — a real tool call both
    ways. This is where the paths genuinely differ: streaming assigns a call's
    index as its fragments open, while a non-streaming response carries no
    index at all and derives one from array position. Getting that wrong pairs
    results to calls that do not exist, and only surfaces a round later as the
    provider rejecting the next request. Arguments are parsed as JSON rather
    than string-compared, because one provider returns `{"host": "prod-1"}`
    streamed and `{"host":"prod-1"}` not.

Both were checked by breaking the vendored driver — dropping the delta callback
and dropping the tool-call id on the non-streaming path each fail the matching
test while leaving the streaming case green.

## [1.13.1] - 2026-08-05

README voice, plus a docs-only dependency bump. No behaviour change.

### Changed

- The README now reads like the rest of the psyb0t projects instead of a
  whitepaper. Same facts, same structure, same tables, same links — the config
  table and every env-var default are untouched.
- elelem and essessey to v0.4.1, both of which are README-only releases
  themselves.

## [1.13.0] - 2026-08-05

Tracks elelem and essessey v0.4.0. No user-visible behaviour change; the LLM
usage recorder now covers the non-streaming provider path as well.

### Changed

- **elelem v0.3.1 → v0.4.0, essessey v0.3.0 → v0.4.0.** elelem collapsed its
  request launchers to `Run` / `RunInto` (`Complete`, `Stream` and
  `CompleteInto` are gone) and added `WithStreaming` for backends that cannot
  serve a streaming call. chatz never called the removed methods outside one
  real-provider test, which now uses `OnDelta(...).Run(ctx)`.
- **`elelem.Driver` gained `Complete`**, so `internal/pkg/usage.Recorder`
  implements it too — and instruments it identically. Tokens are spent and the
  `llm_usage` row is owed whether or not the transport streamed, so a
  `Complete` that merely forwarded would have made a backend's entire usage
  vanish from the metrics the moment streaming was turned off, with nothing
  failing to say so. Both paths now run through one `observe` helper, and a
  table-driven test asserts the same Prometheus series on each.

### Fixed

- The `//nolint:ireturn` on `internal/pkg/mcp.transportFor` sat directly above
  the function with no doc comment and stopped being recognised as the run's
  dependencies changed, which then failed the lint gate on `nolintlint`. It is
  now part of a proper doc comment block, which is the form the rest of the
  repo uses.

## [1.12.2] - 2026-08-05

Removes a data race in the MCP manager's test seam, and lands the test-file
naming convention across the repo. No behaviour change.

### Fixed

- **`internal/pkg/mcp` kept its connect/retry timings in mutable package
  globals that tests reassigned.** That forced every test touching them to
  skip `t.Parallel()` to avoid clobbering a sibling, and STILL left a real
  race: a background retry goroutine reads those values while another test
  writes them. They are now per-`Manager` fields, set once by `NewManager`
  and read-only afterwards, so the whole class of problem is gone and the
  tests run parallel again. This is what made the suite flake once under
  heavy load during the v1.12.1 release.
- The hand-rolled `for time.Now().Before(deadline)` polling loops in those
  tests are now `require.Eventually`, which reports what it was waiting for
  instead of a bare timeout.

### Changed

- **Every test file now names the source file it covers.** Twelve did not:
  `manager_{retry,reconnect,status}_test.go` merged into `manager_test.go`,
  `heartbeat_test.go` + `request_capture_test.go` into `chat_test.go`,
  `{genparams,prompt}_test.go` into `stream_test.go`, `rename_test.go` into
  the new `chats_test.go`, `{identity,requestid,routing}_test.go` into the
  new `server_test.go`, and `handler_mcp_secrets_test.go` renamed to
  `handler_mcp_test.go`. Test bodies are unchanged apart from the polling
  rewrite above; only their homes moved.
- `docs/PLAN.md` is marked historical. It is the pre-build design plan and
  its §A — "lift the SSE package out of brain into a generic in-repo
  package" — describes work that shipped and then moved to its own module,
  so it now carries a header saying what it is and where the three
  materially-changed decisions actually landed.

## [1.12.1] - 2026-08-05

Makes the coverage gate measure what the tests actually cover. CI had been
red since v1.10.0 — five releases — on this one check. No product change.

### Fixed

- **The coverage gate measured the untagged Go run only, and reported 19.3%
  for a codebase the test suites cover 62.3% of.** chatz is a service: most
  of its statements are HTTP handlers, DB wiring, service lifecycle and MCP
  plumbing that the integration suite drives end to end, not code a unit test
  calls directly. `scripts/make/test_coverage.sh` now runs the coverage pass
  with the `integration` tag under the host Docker socket, so the profile
  includes the tests that do the covering. That suite is no longer run twice —
  the coverage pass IS the integration run.
- `-coverpkg` now names `./internal/...,./cmd/...` explicitly. The default
  put the `tests/testinfra` harness — container setup no test can ever
  "cover" — into the denominator, costing ~8 points for nothing.
- **`MIN_TEST_COVERAGE` is set to 60 in chatz's own Makefile** rather than
  inheriting servicepack's library-oriented default of 90. It is a ratchet
  floor just under the measured figure, documented in place as only ever
  moving up.

## [1.12.0] - 2026-08-05

Replaces the in-tree SSE package with `github.com/psyb0t/essessey`. The wire
format is byte-identical — no REST, SSE, config or database change.

### Changed

- **Removed `internal/pkg/sse/` (1,619 lines) in favour of
  [essessey](https://github.com/psyb0t/essessey) v0.3.0.** The protocol,
  publisher, streamers, sink and parser were all general-purpose code with
  nothing chatz-specific in them, and a second project needed the same thing.
  Every symbol maps one-to-one: `sse.Publisher` → `essessey.Publisher`,
  `sse.NewWriterSink` → `essessey/sse.NewWriterSink`, and `sse.Parse(ctx, r)`
  → `essessey.Reassemble(ctx, sse.NewSource(r))`. `Event.Data` is now
  `json.RawMessage` rather than `string`.
- **The content-block index arithmetic moves to `essessey/elelemstream`.**
  `chat.adapterState` in `internal/pkg/core/chat/chat.go` had grown to own two
  unrelated jobs — translating elelem's callbacks into correctly-indexed
  content blocks, and chatz's own stall heartbeat plus per-round logging. Only
  the second is chatz's. It now keeps its own callbacks and binds the shared
  adapter for the first; both run, because elelem v0.3.0's `On*` setters
  append rather than replace.
- Upgraded `github.com/psyb0t/elelem` to v0.3.1.

### Fixed

- **The `integration`, `real`, `e2e` and `qa` test suites did not compile.**
  They were not migrated when `elelem.Message.Content` became a list of
  content parts in v1.11.0, and nothing caught it: files behind a build tag
  are invisible to both `go build ./...` and `golangci-lint`, so that release
  went green with four suites broken. `.golangci.yml` now sets
  `run.build-tags` for all four, which surfaced 39 further issues in those
  files — all fixed here.
- `tests/httpserver` decoded message envelopes with unchecked type assertions,
  so a response-shape change would panic the test rather than fail it.

## [1.11.0] - 2026-08-05

Adopts elelem v0.2.0, whose prompt and message API changed shape. Internal
only — no REST, SSE, config or database change.

### Changed

- **Upgraded `github.com/psyb0t/elelem` to v0.2.0.** A chat turn's whole
  conversation is now one `elelem.Prompt` rather than a hand-assembled message
  slice. `chats.promptMessages` becomes `chats.buildPrompt`, and
  `chat.Request.Messages` becomes `chat.Request.Prompt`.
- `elelem.Message.Content` is an ordered list of content parts instead of a
  string. The `messages.content` column stays text, because every message
  chatz produces is text; `internal/pkg/core/chats/stream.go` converts at the
  boundary. Accepting image or document input later means storing the parts
  instead — the conversion sites say so.

### Fixed

- **A stored assistant turn that only called tools no longer gains an empty
  text part** when read back from the database. `messageFromRow` leaves
  content empty for an empty column rather than manufacturing a blank part.
- **`make generate` failed with `EACCES: permission denied, mkdir
  '/work/web/node_modules/.pnpm'`,** which had been failing the codegen check
  in CI on every run since v1.10.0. Docker creates the `node_modules` named
  volume owned by root while the container runs as the invoking user, so the
  first pnpm install into a fresh volume could not write. The `web-install`
  make target already chowned the volume; `dev_run_web` in
  `scripts/make/lib/dev.sh` did not — which is why it only ever reproduced
  where the volume was new.

## [1.10.2] - 2026-08-05

Moves the pnpm config to where pnpm 10+ reads it. No dependency version
changes — `web/pnpm-lock.yaml` is untouched.

### Fixed

- **`pnpm install --frozen-lockfile` failed with
  `ERR_PNPM_LOCKFILE_CONFIG_MISMATCH`,** which broke `make generate` and the
  codegen-drift check. The `overrides` block still lived in
  `web/package.json`, and pnpm 10 moved that setting into
  `pnpm-workspace.yaml`. pnpm 11 therefore read zero overrides while the
  lockfile recorded three, and refused to proceed. The lockfile itself was
  never stale.
- The move matters beyond CI. With the overrides unread, regenerating the
  lockfile silently resolves `js-yaml` back to 4.2.0 and `cookie` to 0.6.0 —
  the versions those pins exist to move past. Declaring them where pnpm reads
  them keeps the remediation effective; the committed lockfile already had
  4.3.0 and 0.7.2 and still does.
- Approved `esbuild`'s install script via `allowBuilds`. pnpm 10 stopped
  running dependency install scripts unless approved, and pnpm 11 spells the
  setting `allowBuilds` rather than `onlyBuiltDependencies`. esbuild's
  postinstall fetches its platform binary, without which the web build cannot
  run. This restores what pnpm 9 did by default rather than granting anything
  new.

## [1.10.1] - 2026-08-05

Fixes a vendor tree that was incomplete in git, so v1.10.0 does not build from
a fresh clone.

### Fixed

- **`.gitignore` silently excluded part of the vendored source.** The `build`
  pattern was unanchored, and git matches such a pattern at any depth, so
  `vendor/github.com/moby/moby/api/types/build/` was never committed. The tree
  builds locally, where the files are on disk, and fails anywhere that only has
  what git holds — `cannot find module providing package
  github.com/moby/moby/api/types/build`. The pattern is now `/build`, and a
  trailing `!vendor/**` keeps every rule in the file out of the vendored tree,
  since those rules describe this repo's own artifacts.
- Committed the three missing `.go` files plus five upstream docs and fixtures
  the same patterns had excluded.

## [1.10.0] - 2026-08-05

`pkg/elelem` is now its own module. The LLM engine that grew up inside chatz
lives at [github.com/psyb0t/elelem](https://github.com/psyb0t/elelem) and is
consumed as an ordinary dependency. No behavioural change to chatz — it is the
same code, reached by a different import path.

### Removed

- **`pkg/elelem` and everything under it.** The engine, both drivers, the test
  doubles and the conformance suite moved out wholesale. Anything importing
  `github.com/psyb0t/chatz/pkg/elelem/...` should now import
  `github.com/psyb0t/elelem/...`; package and symbol names are unchanged, so
  the migration is the import path and nothing else. The 1.9.0 notes flagged
  this move in advance.
- `make generate-mocks`. The Driver mock is generated in the elelem repo now,
  so it is no longer chatz's to regenerate.
- The `elelem-stays-standalone` rule from `.golangci.yml`. It existed to keep
  `pkg/elelem` free of chatz imports so the extraction would be a pure move;
  the module boundary enforces that by construction now.

### Changed

- Depends on `github.com/psyb0t/elelem v0.1.1`. Every internal import updated.
- SSE debug logging in `internal/pkg/sse/publisher.go` collapses to a single
  `Debug` call site, with the per-payload-shape fields split into small
  helpers. The type switch cannot merge the identical-looking cases — a
  multi-type case binds the value at the switch expression's static type, and
  the three block-start payloads carry three different `ContentBlock` types —
  so the remaining duplication is one line per case. Delta payloads are still
  logged by LENGTH only, never by content.

## [1.9.0] - 2026-08-04

A correctness and hardening pass over `pkg/elelem` before it is extracted into
its own repository. Three independent reviews of the engine, the drivers, and
the untrusted-input surface produced 7 critical and 15 high-severity findings;
every one is fixed, and every behavioural fix carries a test that was checked
to fail against the unfixed code.

### Fixed

- **The retry decorator gave up on the failure it exists to absorb.** Both
  providers report a mid-stream failure in band, inside an HTTP 200 — the
  transport succeeded, the generation did not. Classifying by status read that
  as "nothing to retry", so a single attempt was made. Anthropic's
  `overloaded_error` arrives exactly this way during a capacity event.
  `classifyRetry` now consults the provider's own error code before the status,
  and the OpenAI driver builds a `ProviderError` for in-band errors instead of
  passing an opaque one through.
- **Tool output could stall a request for seconds of uninterruptible CPU.**
  `WithMaxToolResultTokens` was applied only after tokenizing the whole string,
  and the tokenizer is quadratic in the length of an unbroken word-character
  run. 128 KiB of such a run — an inline data URI, a minified asset — measured
  ~14.7s against ~22ms for the same bytes containing spaces, with no
  cancellation point on that path. Input is now bounded by size before it is
  tokenized.
- **The tool-result token cap was exceeded on every truncated result.** The
  retained length was a proportional estimate and the truncation marker's own
  tokens were never counted, so a cap of 20 produced 25. The result is now
  shrunk until it fits including the marker, and truncating twice is a no-op.
- **`WithMaxConcurrentTools` bounded execution but not goroutines.** The
  semaphore was acquired inside the spawned goroutine, so a response declaring
  5000 tool calls created 5000 goroutines regardless of the limit — and the
  call count is chosen by the provider. Distinct tool calls per round and
  accumulated argument bytes are now capped too.
- **Malformed tool-call streams reached the provider.** A call with no id, a
  duplicate id, or one index reused for two different calls was accepted and
  rejected by the provider on the *next* request. These are now dropped or
  split at ingest with a logged `reason`. A duplicate id additionally collapsed
  the caller's gate — one `ToolCallDecision` denied both calls.
- **`ProviderReasoning` accepted any content block on replay.** The writer emits
  only thinking blocks; the reader accepted any type and placed it at the front
  of the assistant turn. Because the field round-trips through the application's
  storage, a stored text block returned as the assistant's own words on every
  later turn. The reader now applies the writer's allowlist.
- **`errors.Is` answered differently per provider.** Only the OpenAI driver
  attached portable sentinels, so a rate limit satisfied
  `errors.Is(err, commonerrors.ErrRateLimited)` for one provider and not the
  other. The mapping moved to `elelem.ProviderSentinel` and both drivers use it.
- **A `Retry-After` header could disable the wait it requested.** The value was
  multiplied by `time.Second` without a range check, so a large integer wrapped
  negative. Parsing moved to `elelem.ParseRetryAfter`, which clamps and never
  returns a negative duration.
- **Credentials in a `WithBaseURL` endpoint reached the logs.** The SDKs embed
  the request URL in the text of every error they build, and drivers log those
  errors. Userinfo is now stripped before the SDK sees it; the API-key header
  path was already clean.
- **An aborted tool execution left an unusable transcript.** The assistant
  message declaring the calls stayed while no results were ever written, so
  feeding `Response.Messages` back produced a request the provider rejects.
  Unanswerable calls are now removed.
- **An errored response advertised a continuable tool loop.** `Response`
  documents a nil `ExecuteToolCalls` as the loop's terminating condition, but
  one was attached alongside `ErrMaxRoundsExceeded`.
- **`Response.Model` was empty** when a driver reported no model in usage, even
  though the request named one.
- **Anthropic forwarded a non-positive `MaxOutputTokens`** to a guaranteed
  provider 400 instead of naming the parameter locally.
- **A failed tool result was invisible to the model on OpenAI.** Anthropic sends
  `is_error` natively and the Chat Completions API has no such field, so the
  flag was dropped — the model's ability to notice its own tool failed depended
  on which driver was configured. It now rides in the result content.
- **`Params.Extra` could overwrite the translated request body.** A key named
  `messages`, `model` or `stream` replaced what the driver had just built, with
  no error. Those fields are now refused; vendor extras still pass through.
- **Out-of-range numeric parameters converted silently.** A float beyond the
  `int64` range passed the integrality check and converted to a plausible
  negative; a provider-supplied tool-call index narrowed on 32-bit targets.

### Added

- **`is_injection` on `messages`, and the `system` role.** A tool's message
  injection that a run *ended on* is now stored, so an interrupted turn is not
  missing the instruction that was pending when it stopped. The flag keeps it
  out of a rebuilt prompt. A stored `system` row is replayable only when it
  carries the flag — an unmarked one is refused, since anything able to write
  that row could otherwise address the model as the system prompt.
- **`elelem.ProviderSentinel`, `elelem.ParseRetryAfter` and
  `elelem.SanitizeBaseURL`** are exported for driver authors, so a third-party
  driver gets the same error semantics, header parsing and credential handling
  as the built-in ones.
- **A trust-boundary section in `pkg/elelem/README.md`** naming both untrusted
  inputs and the three consequences a caller would not guess: a tool can inject
  a system message, a handler's error text is sent to the provider, and
  `WithTimeout` is the only bound on an endless stream.

### Changed

- **Message injections are scoped to the run that created them.** They are
  pinned against history limiting until the model has answered — a budget
  reached mid-loop can no longer discard instruction the next round was meant to
  act on — and dropped from history on the way back in. **Breaking** for a
  caller that relied on `WithMessages`, `WithHistory` or `WithHistoryFrom`
  replaying a stored injection: feeding `Response.Messages` back no longer
  re-sends them. Read `Response.Injections` for a record.
- **History limiting no longer re-tokenizes the whole transcript per dropped
  unit.** Compacting a 400-message transcript went from ~865ms to ~8.6ms and
  from ~587MB to ~5.9MB allocated.
- **`Tool.Timeout` documents what it always did** — it bounds the whole tool
  run including hooks and injector, not the handler alone. The hooks are inside
  the bound deliberately: a deadline starting after them would leave a hanging
  `PreRun` with nothing able to interrupt it.

## [1.8.0] - 2026-08-01

Request-scoped logging that survives being read back, so a token spend can name
the request that caused it, plus a dependency sweep that closed a live logging
bug.

### Added

- **Token spend is attributable to a request.** `llm_usage.request_id` is now
  written. The column existed but was always empty: the correlation id lived
  only as a logger attribute, and there is no way to read an attribute back off
  a logger. It is a context value now, so a row can be traced to the request
  that caused it. Empty outside a request (a background job), which is the
  honest answer for those.
- **Process-wide log fields.** Every line the binary emits carries `service`,
  and `commit` when the build injected one.

### Changed

- **Request-scoped identity moved from the logger to the context.**
  `request_id`, `user_id` and `is_admin` are set once at the HTTP boundary and
  applied to every log line written under that context, exactly as before — but
  they are now readable values rather than write-only logger attributes. The
  attribute names live in `internal/pkg/logging` so the usage recorder can read
  the request id without importing the HTTP layer.
- **Breaking (importers of `pkg/elelem/elelemtest` only).** The driver contract
  suite moved to `pkg/elelem/elelemtest/conformance`:
  `elelemtest.RunConformance` → `conformance.Run`, and
  `elelemtest.ConformanceOptions` → `conformance.Options`. Nothing else in
  `elelemtest` moved, and no application-facing behaviour changed.

  The split is not cosmetic: `conformance` imports `testing` and testify, while
  `elelemtest` now imports neither. Application code reaches `elelemtest` for
  the scripted driver that keeps tests offline, and that import was dragging
  the test framework into the shipped binary.
- **CI codegen-drift check now runs through the shared workflow** via its
  `has_codegen` input, replacing the hand-rolled job.

### Security

- **A failing log sink no longer silences the ones after it.** The
  `slog-configurator` upgrade fixes a fan-out handler that returned at the first
  error; since `slog` discards what a handler returns, one broken sink took
  every later sink down with it and said nothing. chatz stacks a redacting file
  sink on top of the default, so only append-ordering was preventing it.
- **Dependency advisories cleared.** Upgraded past `GO-2026-5158` (OpenTelemetry),
  `GO-2026-4503` (edwards25519) and `GO-2026-5841` (klauspost/compress). The one
  remaining advisory has no fix and does not apply — see the README's
  "Security notes".
- **The dependency age gate no longer applies to first-party modules.** It
  refuses any third-party release younger than seven days, which is what the
  gate is for; a module this project's own author published minutes earlier has
  no upstream account to compromise, and a check routinely stepped around by
  hand is not a check. Pinning, checksums, vendoring and the vulnerability scan
  still apply to those modules.

## [1.7.0] - 2026-07-28

Provider-neutral LLM execution, native Anthropic support, reproducible
generation, and an offline-safe test boundary.

### Added

- **Public provider-neutral LLM engine.** `pkg/elelem` now owns request
  snapshots, streaming assembly, bounded tool loops, whole-unit history
  limiting, retry accounting, structured output, and reusable test seams. It
  ships OpenAI-compatible and Anthropic drivers behind one `Driver` contract.
- **`pkg/elelem/elelemtest` — one home for the test doubles.** A scripted
  `Driver` for tests whose subject consumes model output (the turn loop, tool
  calls, streaming), `RunConformance` for people writing a `Driver`, and the
  generated `elelemtest/mocks.MockDriver` for tests that only need to verify a
  call. `pkg/elelem/README.md` has the pick-one table.
- **Tests can no longer reach a real provider by accident.** Under `go test`
  every configured upstream resolves to the scripted double, so tests exercise
  the app's own driver wiring instead of hand-building one alongside it. With
  no script installed the first `Stream` fails offline rather than dialling
  out. Set `CHATZ_FORCE_REAL_LLM=true` to opt back in — that is what the
  real-LLM suite (`make test-real`) does.
- **`make generate-repos` / `make generate-api`** — regenerate one target on
  demand. `make generate` is now `go generate ./...` plus the web builds; each
  generated package declares its own generator in a `gen.go`, so adding one
  needs no Makefile change. CI runs `make generate` and fails on any diff.
- **Native Anthropic upstreams.** `CHATZ_UPSTREAMS` entries can select
  `"provider":"anthropic"` and reference `ANTHROPIC_API_KEY`, while omitted
  providers retain the OpenAI-compatible behavior.
- **Age-gated Go tooling and vulnerability scan.** The `pkg-*` targets manage
  pinned application/tool dependencies and regenerate `go.sum` plus `vendor/`
  in Docker; `make audit` runs the pinned Go vulnerability scanner.
- **Migration `0000013` adds `messages.provider_reasoning`**, storing the
  provider's native reasoning payload alongside the rendered text so a turn can
  be replayed to the provider in the shape it originally produced. Applied by
  `make migrate` like every other migration; the down migration drops it.
- **History dropped to fit the token budget is now visible.** `DropOldestUnits`
  discarded conversation turns silently. It now logs each dropped unit at DEBUG
  and a WARN summary with `messages_before` / `messages_after` /
  `messages_dropped`, plus a WARN when it is still over budget with nothing left
  it is permitted to drop. Every line carries a stable `reason` value from
  `pkg/elelem/log_reason.go`.

### Fixed

- **Frontend generation used two dependency trees.** `make generate` now uses
  the same Docker named `node_modules` volume for install and Svelte build, so a
  stale host dependency directory cannot hide Rollup's platform package.
- **Release checks now exercise the intended application binary and fail on
  leaked E2E resources.** `make build` builds `./cmd`, browser-test cleanup
  errors fail the owning test, and real-provider tests accept Compose-style
  quoted values from the local `.env` file.

### Changed

- Chatz now delegates provider conversation mechanics to `pkg/elelem` while
  retaining MCP discovery, SSE publication, durable-turn persistence, usage
  records, and exact post-limit debug logging at the application boundary.
- History persistence now follows message origin instead of a seed-length
  slice, so engine-side limiting and transcript repair cannot shift the durable
  turn boundary.

## [1.6.0] - 2026-07-27

Two optional URL-parameter affordances for demos/screencasts, and a fix for the
collapsed-sidebar toggle rendering as an empty button.

### Added

- **`?zoom=<n>` URL parameter** — scales the whole app shell (clamped 0.5–3)
  while keeping it fit to exactly one viewport: the shell divides its
  width/height by the factor and applies CSS `zoom`, so the magnified result is
  still one viewport and nothing overflows or clips. Useful for screencasts,
  demos, and low-vision viewing. Wired in `web/src/routes/+layout.svelte`
  (reads the param on mount, sets a `--zoom` custom property on `.app`).
- **`?sidebar_collapsed=true` URL parameter** — starts the sidebar collapsed to
  its rail from first paint, so the chat pane gets full width immediately
  (screencasts / embeds). `web/src/routes/+layout.svelte`.

### Fixed

- **Collapsed-sidebar rail toggle rendered as an empty rounded box.** The global
  `button { padding: 8px 16px }` applied to the 1.9rem toggle and flex-shrank
  its icon to 0 width, so no glyph showed. Fixed with `padding: 0` on the toggle
  and `flex-shrink: 0` on its `svg` in `web/src/lib/components/Sidebar.svelte`.

### Changed

- Collapsed-rail toggle now shows a hamburger (three-line) glyph with a resting
  border, replacing the near-invisible bare chevron.
- Expanded sidebar content is wrapped in a fixed-width `.sidebar__inner`
  (16rem, `overflow: hidden`) so labels don't reflow or wrap mid-way through the
  collapse↔expand width animation.

No doc sync needed — the new affordances are optional URL query params not
covered by README or `.env.example`; no flags, env vars, or endpoints changed.

## [1.5.1] - 2026-07-24

Test-suite refactor: split the oversized integration test file, table-drive
repeated cases, extract shared helpers/fixtures, and name magic literals. No
runtime, API, or behavior changes.

### Changed

- **Split the 2541-line `tests/httpserver/server_integration_test.go` by
  concern** into `server_helpers_test.go` plus seven focused
  `*_integration_test.go` files (chatflow, turns, showcase, mcp, messages,
  pagination, auth). Shared setup is now `t.Helper()`s (`newAuthedClient`,
  `seedTurns`, `assertPaginates`, `installRejectAssistantTrigger`); repeated
  route/error-code literals are named consts (error codes reuse
  `aichteeteapee.ErrorCode*`). Every test asserts exactly what it did before.
- **Table-drove repeated cases** in `internal/pkg/core/chat/chat_test.go`
  (epilogue stop-reason, content-block ordering) and collapsed the duplicated
  pagination page-walks into a shared `assertPaginates` helper.
- **Extracted shared web test fixtures** (`web/src/lib/render/__fixtures__/`,
  `web/src/test/sse.ts`) so the analytical-component list, demo spec, and SSE
  frame encoder live in one place instead of being copy-pasted across specs;
  split the oversized `analytics.test.ts` assertions into focused cases.
- **Named repeated test literals** as consts across the mcp, real, llmclient,
  http-server, logging, and e2e test packages (model ids, MCP URLs, header
  names via `aichteeteapee`, SSE/console event names), split one mixed-concern
  masking test, dropped a degenerate single-row table, and trimmed stale
  comments that referenced removed code.

No doc sync needed — internal test refactor only; no flags, env vars, endpoints,
or exported behavior changed.

## [1.5.0] - 2026-07-24

Browser e2e ported to Go testcontainers, four more showcase scenarios, a
collapsible-sidebar redesign, JavaScript dependency CVE remediation, and removal
of the `/demo` chat commands.

### Added

- **Browser e2e suite in Go (testcontainers).** `make test-e2e` runs
  self-contained browser drivers under the `e2e` build tag in `tests/e2e/`. Each
  test brings up a throwaway Postgres, the production app image (built from the
  repo `Dockerfile`), a fake upstream, and — where a driver needs it — an MCP
  fixture on one network, then drives a real headless browser
  (`psyb0t/stealthy-auto-browse`) action-by-action and tears the stack down on
  cleanup. Drivers cover the showcase dashboard + reload durability, the theme
  toggle / tool-card / settings / model-picker surface, the admin users page,
  the per-chat and admin MCP flows, the mobile drawer, and a real streamed turn
  (`real_chat`, which self-skips unless a live upstream is configured). Shared
  fixtures live in `tests/testinfra/` (`e2e.go`, `browser.go`); run it against a
  live upstream with `E2E_REAL=1 make test-e2e`.
- **Four more recording-showcase scenarios.** `make run-showcase` now also
  recognizes platform-infrastructure, onboarding-experiment, cloud-finance, and
  incident-reliability prompts alongside the operations/sales/customer catalog,
  each grounded in its own synthetic tool results.
- **Collapsible desktop sidebar.** The sidebar collapses to a slim rail with a
  smoother transition and a rounded toggle; the animation is skipped under
  `prefers-reduced-motion`.

### Changed

- **Browser e2e is now Go/testcontainers instead of Python/bash/compose.** The
  `make test` and `make test-coverage` batteries run the Go e2e suite via
  `make test-e2e`; each suite still creates and tears down its own containers.
- **JavaScript dependency CVE remediation.** Version-scoped `pnpm.overrides`
  bump `postcss`, `js-yaml`, and `cookie` to their nearest patched releases
  without pulling in unrelated majors.
- **The root `Makefile` is now project-owned.** It is listed in
  `.servicepackupdateignore` so framework updates no longer overwrite the
  project's own make targets.

### Removed

- **Breaking. The `/demo` and `/demo_tools` chat commands are gone.** Use
  showcase mode (`CHATZ_SHOWCASE_MODE=true`, `make run-showcase`) to exercise
  the render pipeline without an LLM — it streams the same thinking, tool cards,
  and GenUI dashboards through the same SSE path, and its turns persist. The
  composer no longer special-cases `/demo`, and the Python/bash/compose e2e
  harness (`web/e2e/`, `docker-compose.e2e*.yml`) those commands drove is
  removed in favor of the Go suite above.

## [1.4.0] - 2026-07-24

Deterministic recording showcase, GenUI overflow and reveal polish, and a
chat-load race fix.

### Added

- **Deterministic recording showcase.** `make run-showcase` enables exact
  matching for an embedded operations, sales, and customer-risk catalog. It
  keeps normal model discovery and MCP setup active, streams deterministic
  thinking and synthetic tool activity before GenUI dashboards for catalog
  matches, and persists those exchanges for refreshes and multi-turn recordings.
  Showcase turns now use model- and tool-like pacing; each dashboard metric,
  recommendation, and listed entity is backed by its visible synthetic tool
  result.
- **GenUI components reveal on completion.** A component fades and scales in
  once its streamed patch is fully applied, instead of popping in abruptly;
  the transition is skipped under `prefers-reduced-motion`.

### Changed

- **`make test` now runs the full local battery.** `make test` runs the Go
  unit tests, the real-Postgres integration tests, the web typecheck + unit
  tests, and the curated browser e2e flows (demo, tool cards, showcase);
  `make test-coverage` runs that same battery plus the Go coverage gate, so a
  CI push exercises everything. Each suite still creates and tears down its own
  containers, and the granular targets (`make test-integration`, `make
  web-test`, `make web-e2e*`) remain for running a single suite on its own.
- **The e2e browser harness is collision-safe across concurrent runs.**
  `web/e2e/run-api.sh` and the dedicated `api-demo-tools.sh` /
  `api-chat-mcp.sh` scripts now use a unique Compose project per invocation
  and let Docker assign the browser's host port, and teardown targets the
  exact container ID that run started rather than a fixed name — overlapping
  or parallel runs no longer fight over the same container name or port.

### Fixed

- **Generated dashboards no longer widen the chat shell.** Chart and grid
  layouts shrink to their actual parent cell, and the browser regression covers
  document and message-pane horizontal overflow after the full analytical
  gallery streams.
- **A chat load that resolves after a new turn has started no longer
  clobbers the live conversation.** Opening a chat and sending a message
  before its history finished loading could let the stale fetch land after
  the send and overwrite the in-progress optimistic timeline with an empty
  snapshot; the conversation store now tracks a load revision and drops any
  load whose result is no longer current.

## [1.3.0] - 2026-07-23

Analytical GenUI, durable interrupted sends, and reliable local diagnostics.

### Added

- **Analytical GenUI catalog.** The json-render catalog now includes responsive
  time-series, area, sparkline, bar, donut, funnel, gauge, scatter, heatmap,
  histogram, box-plot, treemap, network, and large log-viewer components for
  rendering operational, usage, cost, event, and database-backed analysis inline
  in chat. The real-browser `/demo` flow verifies every analytical component,
  non-empty chart geometry, viewport fit, and both themes.

### Changed

- **GenUI prompt generation is explicit and complete.** `make genui-prompt`
  regenerates the backend-embedded model prompt from the frontend catalog, and
  `make generate` now includes that step automatically.

### Fixed

- **Stopped streams retain the user message.** A non-demo sent user turn is durable and
  immediately appears in its chat and sidebar; assistant and tool rows remain
  hidden unless the stream finishes and their completion transaction succeeds.
- **Chart rendering respects the active theme and chat owns scrolling.** An
  unstyled SVG label now receives a theme-safe fallback, and the app shell keeps
  vertical scrolling in the message pane instead of exposing a blank document
  scrollbar after rendering charts.
- **`chatz.log` is a regular host file.** `make run` creates it before Compose
  starts and refuses an existing directory or symlink, preventing Docker from
  silently mounting a directory where the app expects its JSON log file.

## [1.2.0] - 2026-07-22

Reliable multi-turn chat continuity, bounded prompt history, safer MCP editing,
and Docker-backed web tooling.

### Added

- **Per-chat prompt-history budgets.** Outbound history is capped at 100,000
  tokens by default. The earliest system message remains sticky and counts
  toward the cap; newer messages are retained from newest to oldest only while
  they fit.
- **Atomic completed turns.** Each persisted message now records its turn and
  completion state, preventing later requests from reading an in-progress
  assistant response as chat history.

### Changed

- **Web tooling now runs in Docker.** The Makefile uses the dev image and a
  dedicated Docker volume for web dependencies, including type checks, tests,
  formatting, API generation, and the Vite development server.
- **Prompt payload logging is debug-only.** Checked-in logging defaults to
  `info`; setting `LOG_LEVEL=debug` in a local `.env` logs the full outbound
  prompt history for diagnosis.

### Fixed

- **MCP Authorization headers remain secret in the UI.** Stored Authorization
  credentials are encrypted at rest and return as a masked placeholder when an
  MCP server is listed or edited; resubmitting that placeholder preserves the
  stored credential.

## [1.1.1] - 2026-07-21

Servicepack framework updated to v1.2.6; no application-facing behavior changed.

### Changed

- **Servicepack framework bumped to v1.2.6.** Pulls in the framework's
  dependency-safe updater (a thin `scripts/make/servicepack/servicepack_update.sh`
  bootstrap that hands off to `scripts/make/servicepack/do_update.sh` from the
  freshly downloaded framework, so `make servicepack-update` no longer wholesale-
  overwrites this project's `go.mod`/`go.sum` and downgrades its dependencies) and
  a service-manager test fix. `go.mod` gained the framework's `modernize` tool
  entry; no project dependency changed version.

### Fixed

- **Restored the project's `.golangci.yml` after the framework update wiped it.**
  The framework overwrote `.golangci.yml` because the project's
  `.servicepackupdateignore` only listed it in a commented-out example. The
  project's linter config (depguard allowlist for its own deps, the `ireturn`
  carve-out for generated strict handlers under `internal/pkg/http/server/`, and
  the lint-exclusion paths for framework-owned dirs) is restored, and
  `.golangci.yml` is now an active entry in `.servicepackupdateignore` so future
  framework updates leave it alone.

## [1.1.0] - 2026-07-21

### Added

- **BPE-tokenized demo streaming.** `/demo` streams its canned markdown +
  generative-UI showcase tokenized with the real o200k_base BPE codec
  (`internal/pkg/tiktoken`, embedded — no network fetch), emitting one SSE
  delta per token so the render pipeline sees the exact same token boundaries
  a live model turn would produce, instead of arbitrary fixed-width text
  chunks.

### Changed

- **Demo commands consolidated.** `/demo_genui` and `/demo_markdown` are
  replaced by a single `/demo` command that streams a combined showcase
  (markdown + several ` ```spec ` generative-UI blocks) from one embedded
  file. `/demo_tools` (the scripted thinking + tool-call turn) is unchanged.
  A `/demo` turn is a pure UI-render exercise and persists nothing, matching
  its prior behavior; `/demo_tools` still persists the exchange like a real
  turn.

### Fixed

- **Demo command hint matched the wrong prefix.** The composer's
  no-upstream-configured affordance checked for a `/demo_` prefix, which
  never matched the new bare `/demo` command — sending it with no model
  configured silently did nothing. The check now matches `/demo` as a
  prefix, covering both `/demo` and `/demo_tools`.

## [1.0.0] - 2026-07-21

Initial release of chatz (renamed from chatter): a self-hosted, single-binary streaming AI chat app.

### Breaking

- **Project renamed from "chatter" to "chatz".** All environment variables now use the `CHATZ_` prefix instead of `CHATTER_`. The application log file renamed from `chatter.log` to `chatz.log`. Existing deployments must update their environment variable names and any references to the old project name.

### Added

- **Reusable "New chat".** Opening a new chat (the sidebar button, or landing
  on `/`) always resolves to a real, ownership-checked chat via
  `POST /api/v1/chats/empty` — creating one if the caller doesn't already have
  an unmessaged chat, or returning the existing one if they do. At most one
  empty chat is kept per user, so repeated "New chat" clicks before typing
  anything always land back on the same chat instead of piling up unused
  rows. The chat gets its title from its first message and starts appearing
  in chat history at that point — an empty chat is never listed. Sending a
  message in any chat (new or existing) bumps it to the top of the sidebar,
  matching the list's most-recent-activity ordering.
- **Responsive mobile sidebar drawer.** On phone-width viewports
  (`max-width: 640px`) the sidebar becomes an off-canvas drawer: a hamburger in
  the header slides it in over the content with a dimming backdrop, and it closes
  on an outside tap (the backdrop), the in-drawer close button, or navigation.
  The desktop full/rail grid is unchanged.
- **Mobile composer + popover polish.** Below `max-width: 1020px` the composer
  reflows from one row into two (an options row for settings/MCP/model above
  the textarea + send row) so the input keeps a usable width on narrow phones
  instead of the controls crowding it out. A new `clampToViewport` Svelte
  action (`web/src/lib/actions/clampToViewport.ts`) measures each popover's
  (settings, MCP picker, model picker) actual rendered position and nudges it
  back on-screen if it would overflow either horizontal edge, the way a
  desktop context menu flips side near a screen edge. Also fixes the message
  list fighting a manual scroll-up: re-attaching "stick to bottom" mode no
  longer forces an immediate scroll on its own — it only takes effect on the
  next new message.
- **Per-chat MCP servers.** A composer control opens a popup listing the chat's
  MCP servers with live connection status (polled) and a per-chat enable toggle;
  a server contributes its tools to a turn only if it is BOTH globally enabled
  AND enabled for that chat. New endpoints
  `GET /api/v1/chats/{chatId}/mcp-servers` and
  `PATCH /api/v1/chats/{chatId}/mcp-servers/{serverId}` (ownership-checked);
  per-chat state persists as a `disabled_mcp_server_ids` JSONB column on `chats`.
- **Web e2e as action-by-action Python drivers.** Browser tests
  (`web/e2e/api_*.py`, driven by `run-api.sh`) POST one browser action at a time
  to a real headless browser against the real embedded-SPA stack and assert after
  each step, failing fast at the exact action. Drivers cover the genui/tools demo
  flows, per-chat + admin MCP, users, a general smoke, the mobile sidebar drawer
  (booted at a phone viewport), and a real streamed chat turn (opt-in, against
  the configured upstream).
- **Streaming chat** over Anthropic-style SSE. `POST /api/v1/chats` creates a
  chat and streams the first assistant turn (the new chat id rides the stream's
  `message_start` event); `POST /api/v1/chats/{chatId}` continues it. Prior-turn
  history resent per turn is token-capped (`CHATTER_MAX_HISTORY_TOKENS`).
- **Embedded SPA, one origin.** The SvelteKit static SPA is built and embedded
  in the Go binary (`go:embed`) and served at `/` with an SPA fallback, next to
  the `/api/v1` JSON API.
- **Generative UI (json-render).** Assistant/tool output can carry fenced
  ` ```spec ` blocks (json-render JSONL, one RFC-6902 patch per line); the
  browser detects fences client-side and renders live components from a shared
  catalog inline in the message flow. Markdown rendering for the prose.
- **Interleaved streaming render.** A turn is modelled as an ORDERED list of
  content blocks (text, thinking, tool) appended in stream-arrival order, so the
  UI shows `text → tool → text → tool → …` exactly as the model emits it — never
  "all tool cards clumped on top, all prose below". Each block renders in place:
  prose as markdown/genui, tool calls as expandable cards, reasoning as thinking
  blocks.
- **Thinking (reasoning) blocks.** Reasoning deltas from upstreams that expose
  them (`reasoning` / `reasoning_content`, e.g. gpt-oss) stream live as their own
  collapsible thinking block ahead of the answer text — separate from the final
  message, not persisted.
- **Tool cards.** Each `tool_use` renders an expandable card showing the tool
  name, the streamed arguments, and the result, transitioning
  `CALLING → DONE/ERROR` as the `tool_result` arrives; errored results render an
  error state.
- **Demo commands.** `/demo_genui` and `/demo_markdown` stream canned responses,
  and `/demo_tools` replays a scripted thinking + tool-call turn (all intercepted
  before the model) so the render pipeline — including interleaving, thinking,
  and tool cards — is exercisable with no LLM or MCP server configured.
- **Admin-provisioned auth.** First-run `/setup` creates the sole admin; admins
  provision and delete additional users (`DELETE /api/v1/users/{userId}`, which
  cascades their sessions + chats and refuses self-deletion); no public
  registration. Opaque session tokens in an `HttpOnly; SameSite=Lax` cookie.
  Optional passwordless single-user mode (`CHATTER_AUTH_PASSWORDLESS`)
  auto-logs-in the sole admin while exactly one user exists.
- **Request-scoped identity in logs.** An auth middleware stamps `user_id` +
  `is_admin` onto the per-request logger, so every log line emitted during an
  authenticated request — including the per-request access line — records who
  made it.
- **Model discovery across upstreams.** `CHATTER_UPSTREAMS` (JSON array of
  OpenAI-compatible endpoints) is probed via each `/models` endpoint and merged
  into one model list; empty falls back to a built-in OpenAI upstream keyed on
  `OPENAI_API_KEY`. API keys referenced by env-var name, never stored inline.
- **MCP tool servers.** Add stdio or HTTP MCP servers (admin), or import a
  Claude-style `.mcp.json`; their tools are aggregated into the chat tool
  catalog. HTTP header secrets and stdio env are encrypted at rest (AES-256-GCM
  AEAD) via `CHATTER_SECRETS_KEY`.
- **Usage accounting + metrics.** A usage decorator wraps every upstream call,
  recording Prometheus metrics and persisting one `llm_usage` row per call
  (best-effort, detached ctx). The recorded metrics are served at `/metrics` on
  `CHATTER_METRICS_LISTENADDRESS` (default `:9091`) and pprof at
  `CHATTER_PROFILING_LISTENADDRESS` (default `:6060`) — both run as internal-only
  listeners alongside the API and are never mounted on the public ingress; an
  empty address disables the listener.
- **Spec-first API + tooling.** `api/api.yml` (OpenAPI 3) drives oapi-codegen
  (strict Echo server + Go client) and the web app's typed client
  (`openapi-typescript` + `openapi-fetch`). gorm-gen repositories for the DB
  layer. Age-gated pnpm dependency lifecycle for the `web/` workspace.
- **Docker compose stack + browser e2e.** `make run` builds and starts
  Postgres + the backend (which embeds the SPA). `make web-e2e` drives the full
  stack through a real headless browser and asserts the embed + serve + SSE
  chain round-trips same-origin.

### Changed

- **`GET /api/v1/chats` no longer lists empty (unmessaged) chats.** Only chats
  with at least one message appear in history; the reusable "New chat" empty
  chat (see Added) is invisible until its first message.
- **Domain→wire converters moved into the packages that own each type.** The
  `X → api.X` projections live in `auth.UserToAPI`, `upstreams.ModelToAPI`,
  `mcp.ServerToAPI`/`ToolToAPI`, and `chats.ChatSummaryToAPI` / `MessageToAPI` /
  `ChatSettingsToAPI` / `ChatMCPServerToAPI`; HTTP handlers just call them (only
  the error-envelope helper stays in the HTTP layer).
- **oapi-codegen upgraded to v2.8.0** (kin-openapi v0.142); the forked
  strict-server template was dropped in favour of the stock template now that its
  SSE-flush fix has landed upstream.
- **Web e2e migrated off declarative YAML.** The YAML browser-flow scripts and
  the YAML host-runner were removed in favour of the Python drivers above;
  `make web-e2e` now runs a driver via `compose-e2e.sh`.
- **HTTP handlers no longer reach the database directly.** Request handlers go
  through core services only: MCP-server persistence moved into
  `mcp.ServerStore` and user list/delete into the `auth` service, and the
  `Query` repository handle was removed from the server's handler deps — so the
  wire layer no longer imports the gorm repositories.
- **Reuse shared psyb0t libraries instead of hand-rolled constants.** HTTP
  header names and error-code strings come from
  `github.com/psyb0t/aichteeteapee` (`HeaderNameXRequestID`, `ErrorCode*`,
  `ErrorCodeFromHTTPStatus`); a missing resource is the single
  `common-go/errors.ErrNotFound`, and a `ctxerrors` error-map collapses
  `gorm.ErrRecordNotFound` to it so the HTTP layer makes one not-found check.
- **Sidebar credit.** The Chatter wordmark now carries a "by psyb0t" link
  (opens `https://github.com/psyb0t/`).

### Fixed

- **Composer input vertical alignment.** The chat input centers within its
  bordered box (`align-items: center`) instead of sitting flush to the bottom.

[1.9.0]: https://github.com/psyb0t/chatz/releases/tag/v1.9.0
[1.15.0]: https://github.com/psyb0t/chatz/releases/tag/v1.15.0
[1.8.0]: https://github.com/psyb0t/chatz/releases/tag/v1.8.0
[1.7.0]: https://github.com/psyb0t/chatz/releases/tag/v1.7.0
[1.6.0]: https://github.com/psyb0t/chatz/releases/tag/v1.6.0
[1.5.1]: https://github.com/psyb0t/chatz/releases/tag/v1.5.1
[1.5.0]: https://github.com/psyb0t/chatz/releases/tag/v1.5.0
[1.4.0]: https://github.com/psyb0t/chatz/releases/tag/v1.4.0
[1.3.0]: https://github.com/psyb0t/chatz/releases/tag/v1.3.0
[1.2.0]: https://github.com/psyb0t/chatz/releases/tag/v1.2.0
[1.1.1]: https://github.com/psyb0t/chatz/releases/tag/v1.1.1
[1.1.0]: https://github.com/psyb0t/chatz/releases/tag/v1.1.0
[1.0.0]: https://github.com/psyb0t/chatter/releases/tag/v1.0.0
[0.4.0]: https://github.com/psyb0t/chatz/releases/tag/v0.4.0
[0.3.0]: https://github.com/psyb0t/chatz/releases/tag/v0.3.0
[0.2.0]: https://github.com/psyb0t/chatz/releases/tag/v0.2.0
