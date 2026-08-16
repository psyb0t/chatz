// Package prompts holds generated, embedded LLM system-prompt fragments for the
// chat turn loop.
package prompts

import (
	_ "embed"
	"strings"
)

// genUIInstructions is the raw embedded GenUI (json-render) usage guide,
// generated from the web component catalog. Never edit the .gen.txt by hand —
// regenerate with `make generate` (the directive lives in this package's
// gen.go).
//
//go:embed genui_instructions.gen.txt
var genUIInstructions string

// GenUIInstructions returns the GenUI usage guide (surrounding whitespace
// trimmed) that teaches the model to emit ```spec component blocks instead of
// markdown/HTML. It is appended to every chat turn's system prompt.
func GenUIInstructions() string {
	return strings.TrimSpace(genUIInstructions)
}
