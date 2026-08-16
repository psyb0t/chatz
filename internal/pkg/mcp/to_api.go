package mcp

import (
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
)

// ServerToAPI projects a stored MCP server row plus its live status to the wire
// shape. Optional command/url/error/reason/toolCount fields stay nil unless the
// row or status actually carries them.
func ServerToAPI(m *models.MCPServer, st Status) api.MCPServer {
	out := api.MCPServer{
		Id:        m.ID,
		Name:      m.Name,
		Transport: api.MCPServerTransport(m.Transport),
		Enabled:   m.Enabled,
		Status:    api.MCPServerStatus(displayState(m.Enabled, st.State)),
	}

	if m.Command != "" {
		cmd := m.Command
		out.Command = &cmd
	}

	if m.URL != "" {
		url := m.URL
		out.Url = &url
	}

	if st.Error != "" {
		errMsg := st.Error
		out.StatusError = &errMsg
	}

	if st.Reason != "" {
		reason := api.MCPServerStatusReason(st.Reason)
		out.StatusReason = &reason
	}

	if st.State == StateConnected {
		count := st.ToolCount
		out.ToolCount = &count
	}

	setHealthTelemetry(&out, st)

	return out
}

func setHealthTelemetry(out *api.MCPServer, st Status) {
	if !st.LastConnectionAttemptAt.IsZero() {
		attemptedAt := st.LastConnectionAttemptAt
		out.LastConnectionAttemptAt = &attemptedAt
	}

	if !st.LastSuccessfulConnectionAt.IsZero() {
		succeededAt := st.LastSuccessfulConnectionAt
		out.LastSuccessfulConnectionAt = &succeededAt
	}

	if !st.LastConnectionFailureAt.IsZero() {
		failedAt := st.LastConnectionFailureAt
		out.LastConnectionFailureAt = &failedAt
	}

	if st.LastConnectionLatency > 0 {
		latencyMS := int(st.LastConnectionLatency.Milliseconds())
		out.LastConnectionLatencyMs = &latencyMS
	}

	if st.LastError != "" {
		lastError := st.LastError
		out.LastError = &lastError
	}
}

// ToolToAPI projects a discovered tool to the wire shape.
func ToolToAPI(t Tool) api.MCPTool {
	// parameters is a required, non-null field; a tool with no input schema
	// surfaces as an empty object rather than null.
	params := t.InputSchema
	if params == nil {
		params = map[string]any{}
	}

	return api.MCPTool{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  params,
	}
}

// displayState resolves the status shown to the admin: a disabled server is
// always "disabled" regardless of any stale live state; an enabled server with
// no recorded state yet (e.g. right after boot, before its async connect
// settles) reads as "connecting".
func displayState(enabled bool, state State) State {
	if !enabled {
		return StateDisabled
	}

	if state == "" {
		return StateConnecting
	}

	return state
}
