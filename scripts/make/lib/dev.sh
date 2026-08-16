#!/bin/bash
# Shared dev-container helpers, sourced by the upper-level scripts/make/*.sh
# overrides (find_script prefers these over scripts/make/servicepack/*). Every
# project command runs inside the Dockerfile.dev sandbox — bind-mounted
# workspace, host UID:GID, ephemeral --rm — never on the host.
set -euo pipefail

APP_NAME="$(head -n 1 go.mod | awk '{print $2}' | awk -F'/' '{print $NF}')"
DEV_IMAGE="${APP_NAME}-dev"

# ensure_dev_image builds the dev image if it is missing. Rebuild explicitly
# with `make dev-image` after changing Dockerfile.dev.
ensure_dev_image() {
	if ! docker image inspect "${DEV_IMAGE}" >/dev/null 2>&1; then
		build_dev_image
	fi
}

build_dev_image() {
	docker build -f Dockerfile.dev -t "${DEV_IMAGE}" .
}

# dev_run runs a command in a throwaway dev container: host UID:GID so files
# land owned by the caller, HOME=/tmp so GOPATH/GOCACHE/pnpm-store are writable,
# workspace bind-mounted at /work. No docker socket, no host network.
dev_run() {
	ensure_dev_image
	docker run --rm \
		-u "$(id -u):$(id -g)" \
		-e HOME=/tmp \
		-v "$(pwd)":/work -w /work \
		"${DEV_IMAGE}" "$@"
}

# ensure_web_volume makes the node_modules volume writable by the caller.
#
# Docker creates a named volume owned by root, while every dev_run_web container
# runs as the host UID — so the FIRST pnpm install into a fresh volume dies with
# `EACCES: permission denied, mkdir '/work/web/node_modules/.pnpm'`. It is
# invisible on a developer machine, where an earlier `make web-install` already
# chowned the volume (that target runs the same fix), and fails every single
# time in CI, where the volume is always new. Deliberately runs WITHOUT -u,
# because root is what chown needs.
ensure_web_volume() {
	ensure_dev_image
	docker run --rm \
		-v "${APP_NAME}-web-node-modules":/node_modules \
		"${DEV_IMAGE}" \
		chown -R "$(id -u):$(id -g)" /node_modules
}

# dev_run_web keeps frontend dependencies in the same named volume used by the
# Makefile's web targets. Without it, generation can accidentally read a stale
# host web/node_modules tree through the workspace bind mount.
dev_run_web() {
	ensure_web_volume
	docker run --rm \
		-u "$(id -u):$(id -g)" \
		-e HOME=/tmp \
		-v "$(pwd)":/work \
		-v "${APP_NAME}-web-node-modules":/work/web/node_modules \
		-w /work \
		"${DEV_IMAGE}" "$@"
}

# dev_run_dind is dev_run plus the host docker socket and same-path mount, for
# integration tests that spawn sibling containers (testcontainers). Uses the
# host network so testcontainers' host-published ports (localhost:<mapped>) are
# directly reachable from the test process — network:host is permitted for
# dev/CI integration runs per hardening-containers.md (never in prod).
dev_run_dind() {
	ensure_dev_image
	local sock=/var/run/docker.sock
	local docker_gid
	docker_gid="$(stat -c '%g' "${sock}" 2>/dev/null || echo 0)"
	docker run --rm \
		-u "$(id -u):$(id -g)" \
		--group-add "${docker_gid}" \
		--network host \
		-e HOME=/tmp \
		-e TESTCONTAINERS_RYUK_DISABLED=true \
		-v "$(pwd)":"$(pwd)" -w "$(pwd)" \
		-v "${sock}":"${sock}" \
		"${DEV_IMAGE}" "$@"
}

# dev_run_dind_envfile resolves .env through Docker Compose before passing it to
# docker run. Compose strips dotenv quoting while docker run keeps quote bytes;
# without this, a quoted JSON CHATZ_UPSTREAMS value reaches the app as invalid
# JSON during real e2e runs. Callers guard on `.env` existing.
dev_run_dind_envfile() {
	ensure_dev_image
	local sock=/var/run/docker.sock
	local docker_gid
	local resolved_env_file
	local status
	docker_gid="$(stat -c '%g' "${sock}" 2>/dev/null || echo 0)"
	resolved_env_file="$(mktemp)"
	if ! awk -F= '
		NR == FNR {
			key = $1
			sub(/^[[:space:]]*export[[:space:]]+/, "", key)
			sub(/[[:space:]]*$/, "", key)
			if (key ~ /^[A-Za-z_][A-Za-z0-9_]*$/) {
				wanted[key] = 1
			}
			next
		}
		$1 in wanted { print }
	' .env <(docker compose --env-file .env config --environment) \
		>"${resolved_env_file}"; then
		rm -f "${resolved_env_file}"
		return 1
	fi

	if docker run --rm \
		-u "$(id -u):$(id -g)" \
		--group-add "${docker_gid}" \
		--network host \
		--env-file "${resolved_env_file}" \
		-e HOME=/tmp \
		-e TESTCONTAINERS_RYUK_DISABLED=true \
		-v "$(pwd)":"$(pwd)" -w "$(pwd)" \
		-v "${sock}":"${sock}" \
		"${DEV_IMAGE}" "$@"; then
		status=0
	else
		status=$?
	fi

	rm -f "${resolved_env_file}"

	return "${status}"
}
