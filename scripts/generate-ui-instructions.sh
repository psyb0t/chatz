#!/bin/bash
# Generates the LLM-facing GenUI (json-render) instructions from chatz's
# component catalog and writes them to a .gen.txt the Go backend embeds and
# appends to every chat turn's system prompt. Run via `make genui-prompt`.
#
# The json-render library (@json-render/core catalog.prompt()) emits a full
# UI-generator system prompt documenting features chatz's client-only,
# static-display renderer does NOT support: /state seeding, repeat/$item/$bind
# dynamic lists, actions (pushState/removeState), events (`on`), visibility
# conditions, and "invent realistic sample data" (which contradicts a chat
# assistant that must ground every value in the real conversation). We keep the
# generated component catalog + the ```spec JSONL format and strip the rest —
# mirroring brain's scripts/generate-ui-instructions.sh.
set -euo pipefail
trap 'log ERROR "command failed exit=$?"' ERR

log() {
	local level="$1"
	shift
	printf '{"time":"%s","level":"%s","file":"%s","line":%d,"func":"%s","msg":"%s"}\n' \
		"$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "${level}" \
		"${BASH_SOURCE[1]##*/}" "${BASH_LINENO[0]}" "${FUNCNAME[1]:-main}" "$*" >&2
}

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${REPO_ROOT}/internal/pkg/core/chats/prompts/genui_instructions.gen.txt"
mkdir -p "$(dirname "${OUT}")"

# Node resolves the mjs's bare imports against web/node_modules, so run it from
# there (the deps @json-render/core + @json-render/svelte + zod are installed for
# the frontend already. The Make target runs this in the dev container.
RAW="$(cd "${REPO_ROOT}/web" && node scripts/gen-ui-instructions.mjs --mode inline)"

# Drop the whole-section blocks for features chatz can't render. A dropped
# section runs from its header until the next KEEP header (OUTPUT FORMAT /
# AVAILABLE COMPONENTS / RULES).
CLEANED="$(printf '%s\n' "${RAW}" | awk '
  /^INITIAL STATE:$/         { skip=1; next }
  /^DYNAMIC LISTS/           { skip=1; next }
  /^ARRAY STATE ACTIONS:$/   { skip=1; next }
  /^AVAILABLE ACTIONS:$/     { skip=1; next }
  /^EVENTS /                 { skip=1; next }
  /^VISIBILITY CONDITIONS:$/ { skip=1; next }
  /^DYNAMIC PROPS:$/         { skip=1; next }
  /^STATE WATCHERS:$/        { skip=1; next }
  /^OUTPUT FORMAT/ || /^AVAILABLE COMPONENTS/ || /^RULES:$/ { skip=0 }
  !skip { print }
')"

# Line-level trims within the kept sections: drop the standalone UI-generator
# role line (chatz sets its own role), the repeat/state worked example, the
# /state clause, and the RULES entries about state/repeat/visible/on/sample-data.
CLEANED="$(printf '%s\n' "${CLEANED}" |
	sed '/^You are a UI generator that outputs JSON\.$/d' |
	sed '/^Example output (each line is a separate JSON object):$/,/^Note: state patches appear/d' |
	sed 's/, then stream \/elements and \/state patches interleaved so the UI fills in progressively as it streams/, then stream \/elements patches/' |
	sed '/^5\. Output \/state patches/d' |
	sed '/^11\. CRITICAL: The "visible" field/d' |
	sed '/^12\. CRITICAL: The "on" field/d' |
	sed '/^13\. When the user asks for a UI that displays data/d' |
	sed '/^14\. When building repeating content/d' |
	sed '/^16\. For data-rich UIs, use multi-column/d' |
	sed '/^17\. Always include realistic, professional-looking sample data/d' |
	awk 'NF { blank=0; print; next } !blank { blank=1; print }')"

printf '%s\n' "${CLEANED}" >"${OUT}"
log info "generated ${OUT} ($(wc -c <"${OUT}") bytes, $(wc -l <"${OUT}") lines)"
