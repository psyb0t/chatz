package mcp

import "errors"

// MCP errors. Declared with errors.New so they stay comparable across
// ctxerrors.Wrap layers via errors.Is.
var (
	// ErrNoServers is returned when a .mcp.json has no mcpServers entries.
	ErrNoServers = errors.New("mcp: no servers in file")

	// ErrInvalidServer is returned for an entry that is neither a stdio server
	// (has command) nor an http server (has url).
	ErrInvalidServer = errors.New("mcp: server has neither command nor url")

	// ErrUnsupportedTransport is returned when a server's transport is neither
	// stdio nor http.
	ErrUnsupportedTransport = errors.New("mcp: unsupported transport")

	// ErrInvalidToolName is returned when a qualified tool name is not of the
	// form <server>__<tool>.
	ErrInvalidToolName = errors.New("mcp: invalid qualified tool name")

	// ErrServerNotFound is returned when a call targets a server not registered
	// with the manager.
	ErrServerNotFound = errors.New("mcp: server not found")
)
