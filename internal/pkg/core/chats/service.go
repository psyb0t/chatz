// Package chats is the chat business-logic layer: it owns the chat + message
// repositories and the model registry, and drives a turn end-to-end (demo
// interception, model resolution, the agentic turn loop, SSE streaming, and
// persistence). HTTP handlers stay dumb — they parse the request, call a method
// here, and map the result (or a sentinel error) to the API response.
package chats

import (
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
)

// Service coordinates the chat repositories, the model upstreams, and the MCP
// tool manager to fulfill the chat API operations.
type Service struct {
	query        *repositories.Query
	models       *upstreams.Registry
	mcp          *mcp.Manager
	showcaseMode bool
}

// New builds the chat service over the given repositories, model registry, and
// MCP manager. Each outbound prompt has a 100000-token default cap; a chat may
// override it with max_history_tokens (see UpdateSettings).
func New(
	query *repositories.Query,
	models *upstreams.Registry,
	mcpMgr *mcp.Manager,
	showcaseMode bool,
) *Service {
	return &Service{
		query:        query,
		models:       models,
		mcp:          mcpMgr,
		showcaseMode: showcaseMode,
	}
}
