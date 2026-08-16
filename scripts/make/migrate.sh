#!/bin/bash
# Runs DB migrations via the app's `migrate` subcommand in the dev container.
# Reads DB config from the environment (APP_DB_* / see .env.example).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Running Migrations (Docker)"
dev_run go run ./cmd migrate
success "Migrations complete"
