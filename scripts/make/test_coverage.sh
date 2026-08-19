#!/bin/bash
# Override of scripts/make/servicepack/test_coverage.sh (find_script prefers
# this upper-level file). Runs the same full battery as `make test` (web +
# e2e) plus the Go coverage gate. CI runs this target, so a push exercises
# everything. coverage.txt is created + removed inside the container, so no
# artifact lands on the host.
#
# The coverage pass runs WITH the integration tag, under dev_run_dind so
# testcontainers can spawn the real Postgres those tests need. That is the
# whole point: chatz is a service, not a library, and its HTTP handlers, DB
# layer and MCP plumbing are covered by tests that drive the real stack rather
# than by unit tests. Measuring the untagged run alone reported 19.3% for a
# codebase the suites actually cover 62.3% of, and the gate sat red for five
# releases because of it.
#
# -coverpkg names PRODUCT code only. Letting it default would put the tests/
# harness (testinfra's container setup, which no test can ever "cover") into
# the denominator and drag the number down for nothing.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/servicepack/common.sh"
source "${SCRIPT_DIR}/lib/dev.sh"
source "${SCRIPT_DIR}/lib/testsuite.sh"

MIN_TEST_COVERAGE="${MIN_TEST_COVERAGE:-90}"
COVERAGE_PKGS="./internal/...,./cmd/..."
# Read by the pipeline's badge job — `coverage_file:` in
# .github/workflows/pipeline.yml names this exact path. Only the framework's
# test_coverage.sh used to produce it, and this file replaces that script
# wholesale, so without writing it here the badge has no source.
readonly COVERAGE_PERCENT_FILE="coverage-percent.txt"

section "Go unit + integration tests with coverage gate (Docker + DIND)"
info "threshold: ${MIN_TEST_COVERAGE}% of ${COVERAGE_PKGS}"
# The percentage file is deliberately NOT in the EXIT trap: the badge job reads
# it after this script exits. The workspace is bind-mounted, so the container's
# write lands on the host. coverage.txt itself is still cleaned up.
# shellcheck disable=SC2016 # ${MIN}/${PKGS}/${PCT_FILE} expand in the container's bash
dev_run_dind env MIN="${MIN_TEST_COVERAGE}" PKGS="${COVERAGE_PKGS}" \
	PCT_FILE="${COVERAGE_PERCENT_FILE}" bash -c '
	set -euo pipefail
	trap "rm -f coverage.txt" EXIT
	go test -race -tags integration -count=1 -timeout=900s \
		-coverpkg="${PKGS}" -coverprofile=coverage.txt ./...
	go tool cover -func=coverage.txt | tee /dev/stderr |
		awk -v min="${MIN}" -v pctfile="${PCT_FILE}" "/^total:/ {
			pct = \$3 + 0
			printf \"%.1f\n\", pct > pctfile
			if (pct < min) {
				printf \"FAIL: coverage %.1f%% < %s%%\n\", pct, min > \"/dev/stderr\"
				exit 1
			}
		}"
'
info "Go coverage meets ${MIN_TEST_COVERAGE}%"

# The integration suite already ran, inside the coverage pass above.
run_web_and_e2e_suites
success "Full test battery + coverage passed"
