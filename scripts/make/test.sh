#!/bin/bash
# Override of scripts/make/servicepack/test.sh (find_script prefers this
# upper-level file). `make test` runs the Go UNIT tests only — no build tag, no
# external infra — so it stays fast and hermetic. The tagged tiers each have
# their own target: `make test-integration` (integration tag, in-process
# handlers over a real Postgres via testcontainers), `make test-api` (api tag,
# the full app image driven end to end), and `make test-real` (real tag, live
# LLM). `make test-coverage` runs the integration tier plus the Go coverage
# gate.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Go unit tests (Docker)"
info "go test -race ./... in ${DEV_IMAGE}"
dev_run go test -race ./...
success "Unit tests passed"
