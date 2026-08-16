#!/bin/bash
# Shared "full battery" runner, sourced by the test.sh + test_coverage.sh
# overrides so `make test` and `make test-coverage` exercise the same suites.
# Each non-unit suite creates and tears down its own containers via
# testcontainers auto-termination -- integration + browser e2e both run under
# dev_run_dind (host docker socket, host network). The caller must have sourced
# servicepack/common.sh (for `section`) AND lib/dev.sh (for `dev_run_dind`,
# used by the e2e script the test-e2e run shells out to).
set -euo pipefail

# run_integration_suite runs the real-Postgres integration tests on their own.
#
# test_coverage.sh does NOT call this: its coverage pass already runs the same
# tagged suite, so calling both would run the multi-minute suite twice for one
# number. See the comment there for why the coverage pass has to include it.
run_integration_suite() {
	section "Integration tests (real Postgres via testcontainers)"
	make test-integration
}

# run_web_and_e2e_suites runs everything past the Go suites, cheapest-first so
# a quick failure aborts before the multi-minute browser e2e stage. Fail-fast:
# under `set -e`, the first failing target stops the whole run.
run_web_and_e2e_suites() {
	section "Web typecheck + unit tests"
	make web-check
	make web-test

	section "Browser e2e (Go testcontainers: showcase, smoke, users, MCP, mobile)"
	bash "${SCRIPT_DIR}/test_e2e.sh"
}

# run_extra_suites is the whole non-unit battery, for a caller whose Go run did
# not already include the integration tags.
run_extra_suites() {
	run_integration_suite
	run_web_and_e2e_suites
}
