#!/bin/bash
# chatz coverage gate. Runs the SAME framework coverage script, but INSIDE the
# dev container via dev_run_dind (host docker socket + --network host) rather
# than on the bare host. The integration tier needs both the Python MCP SDK
# (only in the dev image) and the testcontainers Postgres (whose host-published
# ports are only reachable on localhost:<mapped> under --network host), so the
# coverage run must happen in the container, exactly like `make test-integration`
# and `make test-api`. The framework script runs `go test` directly, assuming it
# is already inside the container; dev_run_dind provides that.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Running Tests with Coverage Check (Docker + DIND)"
dev_run_dind env "MIN_TEST_COVERAGE=${MIN_TEST_COVERAGE:-90}" \
	bash scripts/make/servicepack/test_coverage.sh
