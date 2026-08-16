#!/bin/bash
set -euo pipefail

trap 'echo "[ERROR] ${BASH_SOURCE[0]}:${LINENO} — command failed (exit $?)" >&2' ERR

mapfile -t MODULES < <(
	GOFLAGS='' go list -mod=mod -m \
		-f '{{if and (not .Main) (not .Indirect)}}{{.Path}}{{end}}' all |
		sed '/^$/d'
)

for module in "${MODULES[@]}"; do
	bash scripts/check_go_age.sh "${module}@latest"
done

for module in "${MODULES[@]}"; do
	GOFLAGS='' go get "${module}@latest"
done
