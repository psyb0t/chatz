// Package fixedresponses maps exact demo prompts to fixed assistant turns
// embedded at build time. When demo mode is enabled, matching prompts return
// embedded thinking, tool calls, and dashboards instead of calling the LLM.
// Other prompts use the configured model. See showcase.go for the catalog.
package fixedresponses

import "embed"

//go:embed *.txt
var fs embed.FS
