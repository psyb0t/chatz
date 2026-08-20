#!/bin/bash
# API tests: a real headless browser (stealthy-auto-browse) driving the full
# prod stack (app image built from the repo Dockerfile + a temporary Postgres or
# SQLite database + fake upstream, plus an MCP fixture or a real upstream when a
# driver needs it), every container spawned + torn down by testcontainers. This
# is the only tier that builds and runs the actual chatz binary end to end. Runs
# in the dev container WITH the host docker socket (DIND) and host network so the
# browser container's published port is reachable from the test process -- same
# wiring as test_integration.sh. Tagged `api` so it stays out of the default
# unit run.
#
# The real_chat driver needs a live upstream, so it self-skips unless
# CHATZ_UPSTREAMS + its api key are in the environment. By default this runner
# does NOT inject them, so the battery never spends real tokens and real_chat
# skips cleanly. Set API_REAL=1 to load .env (the same one `make run` uses) so
# real_chat runs a genuine streamed turn; the other drivers always run against
# the fake upstream.
#
# API_PARALLEL bounds how many full stacks run at once (each is pg + app +
# browser, so parallelism is memory-hungry); API_TIMEOUT covers the one-time app
# image build plus every driver; API_RUN accepts a Go test-name regex for a
# focused flow without bypassing this Docker harness. API_DB_DRIVERS defaults
# to both supported stores; set it to one space-separated value when debugging.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

API_PARALLEL="${API_PARALLEL:-2}"
API_TIMEOUT="${API_TIMEOUT:-1800s}"
API_REAL="${API_REAL:-}"
API_RUN="${API_RUN:-}"
API_DB_DRIVERS="${API_DB_DRIVERS:-postgres sqlite}"

test_args=(
	-race
	-tags api
	-count=1
	-parallel "${API_PARALLEL}"
	-timeout "${API_TIMEOUT}"
)

if [[ -n "${API_RUN}" ]]; then
	test_args+=(-run "${API_RUN}")
fi

section "Running API Tests (Docker + DIND)"
info "parallel: ${API_PARALLEL}, timeout: ${API_TIMEOUT}"
if [[ -n "${API_RUN}" ]]; then
	info "test filter: ${API_RUN}"
fi

for db_driver in ${API_DB_DRIVERS}; do
	case "${db_driver}" in
	postgres | sqlite)
		;;
	*)
		error "API_DB_DRIVERS accepts only postgres and sqlite, got ${db_driver}"
		exit 1
		;;
	esac

	info "database driver: ${db_driver}"
	if [[ -n "${API_REAL}" && -f .env ]]; then
		info "API_REAL set + .env present: real_chat will hit the live upstream"
		dev_run_dind_envfile env "CHATZ_API_DB_DRIVER=${db_driver}" \
			go test "${test_args[@]}" ./tests/api/... ./tests/testinfra
	else
		info "fake upstream only: real_chat self-skips (set API_REAL=1 + .env to run it)"
		dev_run_dind env "CHATZ_API_DB_DRIVER=${db_driver}" \
			go test "${test_args[@]}" ./tests/api/... ./tests/testinfra
	fi
done

success "API tests passed"
