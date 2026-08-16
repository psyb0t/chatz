#!/bin/bash
# Real-LLM tests run in the dev container on the host network so they can reach
# the configured LLM gateway. Docker Compose strips enclosing .env quotes,
# whereas docker run --env-file keeps them. This runner normalizes only
# enclosing quotes so it receives the same values as `make run`. Tagged `real`
# keeps the probe out of the default unit run.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/servicepack/common.sh"
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/lib/dev.sh"

section "Running Real-LLM Tests (Docker)"

env_file_args=()
normalized_env_file=""

cleanup() {
	if [[ -n "${normalized_env_file}" && -e "${normalized_env_file}" ]]; then
		rm -f "${normalized_env_file}"
	fi
}
trap cleanup EXIT

if [[ ! -f .env ]]; then
	warning ".env not found — the real-LLM tests need CHATZ_UPSTREAMS + its api key env (the same .env make run uses)"
else
	normalized_env_file="$(mktemp)"
	sed -E \
		-e "s/^([^=]+)='(.*)'$/\\1=\\2/" \
		-e 's/^([^=]+)="(.*)"$/\1=\2/' \
		.env >"${normalized_env_file}"
	env_file_args=(--env-file "${normalized_env_file}")
fi

ensure_dev_image
sock=/var/run/docker.sock
docker_gid="$(stat -c '%g' "${sock}" 2>/dev/null || echo 0)"
docker run --rm \
	-u "$(id -u):$(id -g)" \
	--group-add "${docker_gid}" \
	--network host \
	"${env_file_args[@]}" \
	-e HOME=/tmp \
	-v "$(pwd)":"$(pwd)" -w "$(pwd)" \
	-v "${sock}":"${sock}" \
	"${DEV_IMAGE}" \
	go test -race -tags real -count=1 -timeout=600s ./tests/real/...

success "Real-LLM tests passed"
