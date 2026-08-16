#!/bin/bash
# Integration tests run in the dev container WITH the host docker socket so
# testcontainers can spawn sibling containers (real Postgres etc.). Tagged
# `integration` so they stay out of the default `make test` unit run.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Running Integration Tests (Docker + DIND)"
dev_run_dind go test -race -tags integration -count=1 -timeout=600s ./tests/...
success "Integration tests passed"
