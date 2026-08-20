# Project Makefile — chatz-specific targets on top of the servicepack
# framework. All language tooling runs in the dev container (Dockerfile.dev).
#
# Framework targets (test / dep) are transparently redirected into Docker: their
# recipes call $(call find_script_path,NAME.sh), and the lookup prefers
# ./scripts/make/NAME.sh (our Docker overrides) over the framework's
# ./scripts/make/servicepack/NAME.sh. So `make test` already runs in Docker.
#
# test-coverage runs INSIDE the dev container (like `make test`,
# `make test-integration` and `make test-api`), never on the bare host: chatz's
# scripts/make/test_coverage.sh wraps servicepack's coverage gate in
# dev_run_dind (host docker socket plus --network host). The integration tier
# needs BOTH the Python MCP SDK (only in the dev image) and the testcontainers
# Postgres (whose host-published ports are reachable on localhost:<mapped> only
# under --network host), and under code-workflow the CI runner provides no
# Go/Python toolchain of its own. The gate instruments every package under
# -coverpkg with the integration tag and excludes generated code, cmd mains, and
# mocks from the measured floor. The web typecheck/unit tests and the api tier
# run alongside it in CI via the test command in
# .github/workflows/pipeline.yml, not inside the coverage pass.
#
# MIN is a ratchet floor, not a target: it sits under the real measured number
# and only ever moves UP. Raise it when the measured figure clears the next
# step; never lower it to make a red build go green.
MIN_TEST_COVERAGE = 85

include Makefile.servicepack

.PHONY: dev-image shell run run-showcase stop migrate lint lint-go lint-web \
	test test-go test-web test-coverage \
	generate generate-repos generate-api genui-prompt \
	test-unit test-integration test-real test-api \
	pkg-lock pkg-add pkg-add-tool pkg-update pkg-upgrade pkg-remove audit \
	web-install web-build web-embed web-dev web-gen-api format-web lint-fix-web \
	web-pkg-lock web-pkg-add \
	web-pkg-update web-pkg-upgrade web-pkg-remove

# go:embed target for the built SPA. `make web-embed` syncs web/build/ here so a
# local `go build` serves the real UI (the prod Dockerfile copies web/build into
# this dir at image-build time). Only dist/index.html is committed (placeholder);
# dist/_app/ and the rest are gitignored + regenerated.
WEBASSETS_DIST := internal/pkg/webassets/dist
DEV_RUN := docker run --rm -u $(shell id -u):$(shell id -g) -e HOME=/tmp \
	-v $(PWD):/work -w /work $(APP_NAME)-dev
WEB_RUN := docker run --rm -u $(shell id -u):$(shell id -g) -e HOME=/tmp \
	-v $(PWD):/work -v $(APP_NAME)-web-node-modules:/work/web/node_modules \
	-w /work $(APP_NAME)-dev
WEB_DEPS_INIT := docker run --rm -v $(APP_NAME)-web-node-modules:/node_modules \
	$(APP_NAME)-dev chown -R $(shell id -u):$(shell id -g) /node_modules
WEB_DEV_RUN := docker run --rm -it -u $(shell id -u):$(shell id -g) -e HOME=/tmp \
	-e CHATZ_API_TARGET \
	-p 5173:5173 -v $(PWD):/work -v $(APP_NAME)-web-node-modules:/work/web/node_modules \
	-w /work $(APP_NAME)-dev

# ---------------------------------------------------------------------------
# Web frontend (SvelteKit static SPA under ./web) — pnpm only, never npm.
# The age-gated web-pkg-* targets bump minimumReleaseAge in
# web/pnpm-workspace.yaml to a 7-day floor derived from `date -u -d '7 days ago'`
# BEFORE the pnpm op, and every install passes --ignore-scripts
# (postinstall is the #1 JS supply-chain attack vector).
# ---------------------------------------------------------------------------

WEB_WORKSPACE := web/pnpm-workspace.yaml
# Bump minimumReleaseAge to the whole-minutes distance between "7 days ago"
# and now — a static 7-day floor frozen until the next conscious dep mutation.
WEB_BUMP_AGE = \
	age=$$(( ( $$(date -u +%s) - $$(date -u -d '7 days ago' +%s) ) / 60 )); \
	sed -i -E "s/^minimumReleaseAge:.*/minimumReleaseAge: $$age/" $(WEB_WORKSPACE); \
	echo "bumped minimumReleaseAge -> $$age minutes"

dev-image: ## Build the dev container image
	@bash "$(call find_script_path,dev_image.sh)"

shell: ## Open an interactive shell in the dev container
	@docker run --rm -it -u $(shell id -u):$(shell id -g) -e HOME=/tmp \
		-v $(PWD):/work -w /work $(APP_NAME)-dev bash

run: ## Run Chatz via docker compose (Postgres by default; SQLite from .env)
	@bash "$(call find_script_path,run.sh)"

run-showcase: ## Run the normal stack with exact-message showcase interception
	@CHATZ_SHOWCASE_MODE=true bash "$(call find_script_path,run.sh)"

stop: ## Stop the stack
	@docker compose down

generate: ## Run all codegen in Docker (go generate ./... + the web builds)
	@bash "$(call find_script_path,generate.sh)"

# Path-scoped targets exist ONLY to regenerate one thing on demand. The
# generator commands live in each output package's gen.go, so adding a
# generated package needs a gen.go -- never a new line here.
generate-repos: dev-image ## Regenerate just the gorm repositories
	$(DEV_RUN) go generate ./internal/pkg/db/repositories/...

generate-api: dev-image ## Regenerate just the HTTP server + client
	$(DEV_RUN) go generate ./internal/pkg/http/api/... ./pkg/http/api/client/...

migrate: ## Run DB migrations in Docker
	@bash "$(call find_script_path,migrate.sh)"

lint: lint-go lint-web ## Lint everything: Go/shell and the web app

lint-go: ## Lint Go and shell (shfmt, shellcheck, go fix, golangci-lint) in Docker
	@$(MAKE) dev-image
	@$(call run_dev_script,lint.sh)

test-integration: ## Run integration tests (testcontainers, DIND) in Docker
	@bash "$(call find_script_path,test_integration.sh)"

test: test-go test-web ## Run all unit tests: Go and web

test-go: ## Run the Go unit tests (no build tag, no infra) in Docker
	@bash "$(call find_script_path,test.sh)"

test-coverage: ## Backend Go coverage gate plus the web unit tests (coverage is Go-only)
	@MIN_TEST_COVERAGE=$(MIN_TEST_COVERAGE) bash "$(call find_script_path,test_coverage.sh)"
	@$(MAKE) test-web

test-unit: dev-image ## Run race-enabled Go tests in Docker (optional PKG=./path/...)
	$(DEV_RUN) go test -race $(or $(PKG),./...)

test-real: ## Run the real-LLM tests (dev container, host net, uses the same .env as make run)
	@bash "$(call find_script_path,test_real.sh)"

test-api: ## Run API tests (full app image + browser, Go testcontainers, DIND)
	@bash "$(call find_script_path,test_api.sh)"

# --- Go dependencies: age-gated and always vendored -------------------------

pkg-lock: dev-image ## Refresh go.mod/go.sum/vendor under existing pins
	$(DEV_RUN) env GOFLAGS= go mod tidy
	$(DEV_RUN) env GOFLAGS= go mod vendor

pkg-add: dev-image ## Add a pinned Go dependency (usage: make pkg-add PKG=module@version)
	@test -n "$(PKG)" || (echo "usage: make pkg-add PKG=module@version" >&2; exit 1)
	$(DEV_RUN) bash scripts/check_go_age.sh "$(PKG)"
	$(DEV_RUN) env GOFLAGS= go get "$(PKG)"
	$(MAKE) pkg-lock

pkg-add-tool: dev-image ## Add a pinned Go dev tool (optional AGE_PKG=owning-module@version)
	@test -n "$(PKG)" || (echo "usage: make pkg-add-tool PKG=module@version" >&2; exit 1)
	$(DEV_RUN) bash scripts/check_go_age.sh "$(or $(AGE_PKG),$(PKG))"
	$(DEV_RUN) env GOFLAGS= go get -tool "$(PKG)"
	$(MAKE) pkg-lock

pkg-update: dev-image ## Update one Go dependency after its latest release passes the age gate
	@test -n "$(PKG)" || (echo "usage: make pkg-update PKG=module" >&2; exit 1)
	$(DEV_RUN) bash scripts/check_go_age.sh "$(PKG)@latest"
	$(DEV_RUN) env GOFLAGS= go get "$(PKG)@latest"
	$(MAKE) pkg-lock

pkg-upgrade: dev-image ## Upgrade direct Go dependencies after every latest release passes the age gate
	$(DEV_RUN) bash scripts/upgrade_go_dependencies.sh
	$(MAKE) pkg-lock

pkg-remove: dev-image ## Remove one direct Go dependency (usage: make pkg-remove PKG=module)
	@test -n "$(PKG)" || (echo "usage: make pkg-remove PKG=module" >&2; exit 1)
	$(DEV_RUN) env GOFLAGS= go mod edit -droprequire="$(PKG)"
	$(MAKE) pkg-lock

audit: dev-image ## Scan reachable Go code against the Go vulnerability database
	$(DEV_RUN) go tool govulncheck ./...

# --- Web frontend: build / dev / check / format -----------------------------

web-install: dev-image ## Install web deps from the lockfile (frozen, no scripts)
	$(WEB_DEPS_INIT)
	$(WEB_RUN) pnpm --dir web install --frozen-lockfile --ignore-scripts

web-build: web-install ## Build the web static SPA (output: web/build)
	$(WEB_RUN) pnpm --dir web build

web-embed: web-build ## Build the SPA and sync it into the go:embed dist dir
	$(WEB_RUN) bash -c 'rm -rf "$(WEBASSETS_DIST)/_app"; cp -R web/build/. "$(WEBASSETS_DIST)/"'
	@echo "embedded web/build -> $(WEBASSETS_DIST)"

web-dev: web-install ## Run the web dev server (vite) at http://localhost:5173
	$(WEB_DEV_RUN) pnpm --dir web dev --host 0.0.0.0

web-gen-api: web-install ## Generate the web API types from api/api.yml (src/lib/api/schema.d.ts)
	$(WEB_RUN) pnpm --dir web gen:api

genui-prompt: web-install ## Regenerate just the backend's embedded GenUI prompt
	$(WEB_RUN) go generate ./internal/pkg/core/chats/prompts/...

lint-web: web-install ## Lint the web app: prettier --check + svelte-check (strict)
	$(WEB_RUN) bash -c 'pnpm --dir web exec prettier --check . && pnpm --dir web check'

test-web: web-install ## Run the web unit tests (vitest, SSE parser + conversation store)
	$(WEB_RUN) pnpm --dir web test

format-web: web-install ## Format the web app (prettier --write)
	$(WEB_RUN) pnpm --dir web format

lint-fix-web: web-install ## Auto-fix + type-check the web app (prettier --write + svelte-check)
	$(WEB_RUN) bash -c 'pnpm --dir web format && pnpm --dir web check'

# --- Web frontend: age-gated dependency lifecycle ---------------------------

web-pkg-lock: dev-image ## Refresh web/pnpm-lock.yaml under the current age-gate (no bump)
	$(WEB_RUN) pnpm --dir web install --lockfile-only --ignore-scripts

web-pkg-add: dev-image ## Bump age-gate then add a web dep (usage: make web-pkg-add PKG=name[@ver])
	@test -n "$(PKG)" || (echo "usage: make web-pkg-add PKG=name[@ver]" >&2; exit 1)
	@$(WEB_RUN) bash -c '$(WEB_BUMP_AGE)'
	$(WEB_RUN) pnpm --dir web add --ignore-scripts $(PKG)

web-pkg-update: dev-image ## Bump age-gate then upgrade ONE web dep (usage: make web-pkg-update PKG=name)
	@test -n "$(PKG)" || (echo "usage: make web-pkg-update PKG=name" >&2; exit 1)
	@$(WEB_RUN) bash -c '$(WEB_BUMP_AGE)'
	$(WEB_RUN) pnpm --dir web update --latest --ignore-scripts $(PKG)

web-pkg-upgrade: dev-image ## Bump age-gate then upgrade ALL web deps to latest
	@$(WEB_RUN) bash -c '$(WEB_BUMP_AGE)'
	$(WEB_RUN) pnpm --dir web update --latest --ignore-scripts

web-pkg-remove: dev-image ## Bump age-gate then remove a web dep (usage: make web-pkg-remove PKG=name)
	@test -n "$(PKG)" || (echo "usage: make web-pkg-remove PKG=name" >&2; exit 1)
	@$(WEB_RUN) bash -c '$(WEB_BUMP_AGE)'
	$(WEB_RUN) pnpm --dir web remove $(PKG)
