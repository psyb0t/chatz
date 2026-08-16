package fixedresponses

import (
	"strings"
	"time"
)

// Kind tags what a resolved demo response streams: canned answer text, or a
// scripted agentic turn (thinking + tool-call blocks).
type Kind string

const (
	KindText  Kind = "text"
	KindTools Kind = "tools"
)

// Response is a resolved demo command: canned Text (KindText) streamed as-is,
// or a scripted Steps sequence (KindTools) replayed as thinking/text/tool SSE
// blocks so the tool-card + interleaving UI is exercised without an LLM or a
// live MCP server. Persist makes a canned response a durable chat turn.
type Response struct {
	Kind           Kind
	Text           string
	Steps          []Step
	Persist        bool
	InitialDelay   time.Duration
	TextChunkDelay time.Duration
}

// StepKind tags one scripted step in a KindTools demo.
type StepKind string

const (
	StepThinking StepKind = "thinking"
	StepText     StepKind = "text"
	StepTool     StepKind = "tool"
)

// Step is one ordered block in a scripted demo turn: a thinking block, an
// answer text block, or a tool call (with its result). Rendered in order so the
// demo shows text → tool → text → tool interleaving, not clumped blocks.
type Step struct {
	Kind        StepKind
	DelayBefore time.Duration
	// Text is the block body for StepThinking and StepText.
	Text string
	// Tool is the call for StepTool.
	Tool *ToolStep
}

// ToolStep is one canned tool call: the streamed args, result, and result
// timing mirroring a real tool_use + tool_result exchange.
type ToolStep struct {
	ToolUseID   string
	Name        string
	ArgsJSON    string
	ResultText  string
	IsError     bool
	ResultDelay time.Duration
}

// AnswerText returns the concatenated answer-text steps — what gets persisted
// as the assistant message (thinking + tool plumbing are ephemeral, matching a
// real turn where only the final text is stored).
func (r Response) AnswerText() string {
	if r.Kind == KindText {
		return r.Text
	}

	var out strings.Builder

	for _, step := range r.Steps {
		if step.Kind == StepText {
			out.WriteString(step.Text)
		}
	}

	return out.String()
}
