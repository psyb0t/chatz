#!/bin/bash
set -euo pipefail

trap 'echo "[ERROR] ${BASH_SOURCE[0]}:${LINENO} — command failed (exit $?)" >&2' ERR

readonly MINIMUM_AGE_DAYS=7
readonly SECONDS_PER_DAY=86400
readonly MODULE_VERSION="${1:-}"

if [[ -z "${MODULE_VERSION}" || "${MODULE_VERSION}" != *@* ]]; then
	echo "usage: $0 module@version" >&2
	exit 1
fi

readonly MODULE="${MODULE_VERSION%@*}"
readonly REQUESTED_VERSION="${MODULE_VERSION##*@}"

if [[ -z "${MODULE}" || -z "${REQUESTED_VERSION}" ]]; then
	echo "module and version are required" >&2
	exit 1
fi

# First-party modules skip the wait. The gate exists because a malicious release
# of SOMEONE ELSE'S package is usually caught and yanked within hours, so it is
# worth not being whoever installs it in hour zero. That threat model does not
# describe a module this project's own author wrote and pushed minutes earlier:
# there is no upstream maintainer account to compromise and no unknown publisher.
# Waiting a week to consume your own commit buys nothing, and a gate people
# routinely step around by hand stops being a gate at all.
#
# The rest of the supply-chain stack still applies to these: pinned in go.mod,
# checksummed in go.sum, vendored, and scanned by `make audit`.
readonly FIRST_PARTY_PREFIX="github.com/psyb0t/"

if [[ "${MODULE}" == "${FIRST_PARTY_PREFIX}"* ]]; then
	echo "age gate skipped: ${MODULE} is first-party"
	exit 0
fi

MODULE_INFO="$(GOFLAGS='' go list -mod=mod -m -json "${MODULE}@${REQUESTED_VERSION}")"
RESOLVED_VERSION="$(sed -n 's/^[[:space:]]*"Version": "\([^"]*\)",*/\1/p' <<<"${MODULE_INFO}")"
RELEASED_AT="$(sed -n 's/^[[:space:]]*"Time": "\([^"]*\)",*/\1/p' <<<"${MODULE_INFO}")"

if [[ -z "${RESOLVED_VERSION}" || -z "${RELEASED_AT}" ]]; then
	echo "could not resolve release metadata for ${MODULE_VERSION}" >&2
	exit 1
fi

if ! RELEASED_EPOCH="$(
	date -u -D '%Y-%m-%dT%H:%M:%SZ' -d "${RELEASED_AT}" +%s
)"; then
	echo "could not parse release time for ${MODULE}@${RESOLVED_VERSION}" >&2
	exit 1
fi

readonly RELEASED_EPOCH
NOW_EPOCH="$(date -u +%s)"
readonly NOW_EPOCH
readonly CUTOFF_EPOCH="$((NOW_EPOCH - MINIMUM_AGE_DAYS * SECONDS_PER_DAY))"

if ((RELEASED_EPOCH > CUTOFF_EPOCH)); then
	echo "refusing ${MODULE}@${RESOLVED_VERSION}: released less than ${MINIMUM_AGE_DAYS} days ago" >&2
	exit 1
fi

echo "age gate passed: ${MODULE}@${RESOLVED_VERSION} (${RELEASED_AT})"
