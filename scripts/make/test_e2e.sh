#!/bin/bash
# Browser e2e tests: a real headless browser (stealthy-auto-browse) driving the
# full prod stack (app image built from the repo Dockerfile + a temporary
# Postgres or SQLite database + fake upstream, plus an MCP fixture or a real
# upstream when a driver needs it), every container spawned + torn down by
# testcontainers. Runs in the dev
# container WITH the host docker socket (DIND) and host network so the browser
# container's published port is reachable from the test process -- same wiring
# as test_integration.sh. Tagged `e2e` so it stays out of the default unit run.
#
# The real_chat driver needs a live upstream, so it self-skips unless
# CHATZ_UPSTREAMS + its api key are in the environment. By default this runner
# does NOT inject them, so the battery never spends real tokens and real_chat
# skips cleanly. Set E2E_REAL=1 to load .env (the same one `make run` uses) so
# real_chat runs a genuine streamed turn; the other six always run against the
# fake upstream.
#
# E2E_PARALLEL bounds how many full stacks run at once (each is pg + app +
# browser, so parallelism is memory-hungry); E2E_TIMEOUT covers the one-time app
# image build plus every driver; E2E_RUN accepts a Go test-name regex for a
# focused flow without bypassing this Docker harness. E2E_DB_DRIVERS defaults
# to both supported stores; set it to one space-separated value when debugging.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

E2E_PARALLEL="${E2E_PARALLEL:-2}"
E2E_TIMEOUT="${E2E_TIMEOUT:-1800s}"
E2E_REAL="${E2E_REAL:-}"
E2E_RUN="${E2E_RUN:-}"
E2E_DB_DRIVERS="${E2E_DB_DRIVERS:-postgres sqlite}"

test_args=(
	-race
	-tags e2e
	-count=1
	-parallel "${E2E_PARALLEL}"
	-timeout "${E2E_TIMEOUT}"
)

if [[ -n "${E2E_RUN}" ]]; then
	test_args+=(-run "${E2E_RUN}")
fi

section "Running Browser E2E Tests (Docker + DIND)"
info "parallel: ${E2E_PARALLEL}, timeout: ${E2E_TIMEOUT}"
if [[ -n "${E2E_RUN}" ]]; then
	info "test filter: ${E2E_RUN}"
fi

for db_driver in ${E2E_DB_DRIVERS}; do
	case "${db_driver}" in
	postgres | sqlite)
		;;
	*)
		error "E2E_DB_DRIVERS accepts only postgres and sqlite, got ${db_driver}"
		exit 1
		;;
	esac

	info "database driver: ${db_driver}"
	if [[ -n "${E2E_REAL}" && -f .env ]]; then
		info "E2E_REAL set + .env present: real_chat will hit the live upstream"
		dev_run_dind_envfile env "CHATZ_E2E_DB_DRIVER=${db_driver}" \
			go test "${test_args[@]}" ./tests/e2e/... ./tests/testinfra
	else
		info "fake upstream only: real_chat self-skips (set E2E_REAL=1 + .env to run it)"
		dev_run_dind env "CHATZ_E2E_DB_DRIVER=${db_driver}" \
			go test "${test_args[@]}" ./tests/e2e/... ./tests/testinfra
	fi
done

success "Browser e2e tests passed"
