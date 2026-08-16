#!/bin/bash
# Builds the application command. The sibling repogen command is a code
# generator and must not be passed to this single-binary build target.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

readonly APP_COMMAND="./cmd"
readonly BUILD_DIR="build"

trap 'error "build failed at ${BASH_SOURCE[0]}:${LINENO} (exit $?)"' ERR

section "Building Application (Docker)"
info "Building ${APP_NAME} from ${APP_COMMAND}"

dev_run mkdir -p "${BUILD_DIR}"
dev_run env CGO_ENABLED=0 go build \
	-trimpath \
	-ldflags "-s -w -X main.appName=${APP_NAME}" \
	-o "${BUILD_DIR}/${APP_NAME}" \
	"${APP_COMMAND}"

success "Binary built successfully: ${BUILD_DIR}/${APP_NAME}"
