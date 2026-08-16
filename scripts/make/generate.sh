#!/bin/bash
# Runs every code generator in the dev container.
#
# The Go-toolchain generators are NOT enumerated here -- each output package
# declares its own generator in a gen.go (`//go:generate ...`), so one
# `go generate ./...` discovers all of them and a newly generated package needs
# no change to this script. Only the pnpm builds, which nothing drives via
# go:generate, are still spelled out.
#
# go generate runs under dev_run_web because one of those directives (the
# embedded GenUI prompt) shells out to node against web/node_modules, and only
# that helper mounts the volume holding it -- hence the web install first.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"

section "Generating (Docker)"

if [[ -f web/package.json ]]; then
	info "web dependencies (the GenUI prompt generator runs node against them)"
	dev_run_web pnpm --dir web install --frozen-lockfile --ignore-scripts
else
	info "skip web deps: web not present yet"
fi

info "go generate ./... (gorm repos, openapi server+client, elelem mock, GenUI prompt)"
dev_run_web go generate ./...

if [[ -f web/ui-lib/package.json ]]; then
	info "ui-lib .mjs bundle"
	dev_run pnpm --dir web/ui-lib install --frozen-lockfile --ignore-scripts
	dev_run pnpm --dir web/ui-lib build
else
	info "skip ui-lib: web/ui-lib not present yet"
fi

if [[ -f web/package.json ]]; then
	info "svelte static build (embedded)"
	dev_run_web pnpm --dir web build
else
	info "skip frontend: web not present yet"
fi

success "Generation complete"
