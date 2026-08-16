package server

import (
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
)

// envelope builds the wire error body. code is an aichteeteapee.ErrorCode
// (UPPER_SNAKE_CASE; clients switch on it) — the canonical HTTP error-code set
// lives in aichteeteapee, so these strings are never hand-rolled here. The
// status→code mapping is aichteeteapee.ErrorCodeFromHTTPStatus.
//
// Domain→wire converters live in the package that owns each domain type
// (auth.UserToAPI, upstreams.ModelToAPI, mcp.ServerToAPI/ToolToAPI,
// chats.ChatSummaryToAPI/MessageToAPI/ChatSettingsToAPI/ChatMCPServerToAPI);
// handlers just call them. This envelope is the one exception — it is the HTTP
// error wire format itself, not a domain projection.
func envelope(code, message string) api.Error {
	return api.Error{Code: code, Message: message}
}
