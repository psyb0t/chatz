#!/bin/bash
# Override of scripts/make/servicepack/dep.sh — tidy + vendor in the dev
# container so no module cache / vendor churn touches the host.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Updating Dependencies (Docker)"
info "go mod tidy"
dev_run env GOFLAGS= go mod tidy
info "go mod vendor"
dev_run env GOFLAGS= go mod vendor
success "Dependencies vendored"
