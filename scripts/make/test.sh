#!/bin/bash
# Override of scripts/make/servicepack/test.sh (find_script prefers this
# upper-level file). `make test` runs the full local battery: Go unit tests,
# real-Postgres integration tests, the web typecheck + unit tests, and the
# browser e2e suite (Go testcontainers). `make test-coverage` runs the same
# battery plus the Go coverage gate. The granular targets (test-integration,
# web-test, test-e2e) stay available for running any single suite on its own.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"
source "${SCRIPT_DIR}/lib/testsuite.sh"

section "Go unit tests (Docker)"
info "go test -race ./... in ${DEV_IMAGE}"
dev_run go test -race ./...

run_extra_suites
success "Full test battery passed"
