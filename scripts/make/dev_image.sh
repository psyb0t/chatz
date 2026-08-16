#!/bin/bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Building Dev Image"
info "docker build -f Dockerfile.dev -t ${DEV_IMAGE}"
build_dev_image
success "Dev image ready: ${DEV_IMAGE}"
