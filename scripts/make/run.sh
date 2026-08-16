#!/bin/bash
# Runs Chatz via docker compose. Postgres is the default; SQLite skips that
# dependency and stores its single-process database in Chatz's /data volume.
# Build without the layer cache so every make run compiles the current source.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"

readonly LOG_FILE_PATH="chatz.log"
readonly DB_DRIVER_POSTGRES="postgres"
readonly DB_DRIVER_SQLITE="sqlite"

trap 'error "command failed at ${BASH_SOURCE[0]}:${LINENO}"' ERR

section "Running Stack (docker compose)"

if [[ ! -f .env ]]; then
	warning ".env not found — copy .env.example to .env first"
fi

if [[ -e "${LOG_FILE_PATH}" && ! -f "${LOG_FILE_PATH}" ]] || [[ -L "${LOG_FILE_PATH}" ]]; then
	error "${LOG_FILE_PATH} must be a regular file, not a directory or symlink."
	exit 1
fi

if ! touch "${LOG_FILE_PATH}"; then
	error "failed to create ${LOG_FILE_PATH}"
	exit 1
fi

db_driver="${CHATZ_DB_DRIVER:-}"
if [[ -z "${db_driver}" && -f .env ]]; then
	while IFS= read -r line; do
		if [[ "${line}" == CHATZ_DB_DRIVER=* ]]; then
			db_driver="${line#*=}"
			break
		fi
	done <.env
fi

db_driver="${db_driver:-${DB_DRIVER_POSTGRES}}"
case "${db_driver}" in
"${DB_DRIVER_POSTGRES}" | "${DB_DRIVER_SQLITE}")
	;;
*)
	error "CHATZ_DB_DRIVER must be ${DB_DRIVER_POSTGRES} or ${DB_DRIVER_SQLITE}, got ${db_driver}"
	exit 1
	;;
esac

info "docker compose build --no-cache"
docker compose build --no-cache

info "docker compose up"
if [[ "${db_driver}" == "${DB_DRIVER_SQLITE}" ]]; then
	info "starting SQLite mode without the postgres dependency"
	exec docker compose up --no-deps "$@" chatz
fi

exec docker compose up "$@"
