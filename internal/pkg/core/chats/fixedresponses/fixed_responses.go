// Package fixedresponses maps exact, recording-ready chat prompts to canned
// assistant turns embedded at build time. When showcase mode is enabled the
// chat handler intercepts a matching prompt and replays the embedded thinking +
// tool-call + dashboard turn instead of calling the LLM, so the UI-rendering
// pipeline (thinking blocks, tool cards, ```spec json-render dashboards) is
// exercised end-to-end without a live model. Any non-matching prompt falls
// through to the real LLM path. See showcase.go for the catalog.
package fixedresponses

import "embed"

//go:embed *.txt
var fs embed.FS
