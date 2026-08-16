// Package mcp connects chatz to MCP servers (stdio + http) and imports their
// configuration. Servers live in the DB (admin-managed via the UI) or are
// imported from a Claude-style .mcp.json here. Header + env secrets are sealed
// at rest via internal/pkg/secrets — plaintext never touches the DB.
package mcp

import (
	"encoding/json"
	"sort"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/ctxerrors"
	"gorm.io/datatypes"
)

// mcpJSONFile is the Claude-style .mcp.json shape: a map of server name to its
// launch config.
type mcpJSONFile struct {
	MCPServers map[string]mcpJSONServer `json:"mcpServers"`
}

// mcpJSONServer covers both transports. command/args/env => stdio; url/headers
// => http. type is advisory (we infer from the fields present).
type mcpJSONServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// ParseMCPJSON parses raw .mcp.json bytes into MCPServer rows ready to persist,
// sealing env + header secrets with box. createdBy stamps the importing admin
// (nil is allowed). Servers come back sorted by name for deterministic output.
func ParseMCPJSON(
	raw []byte,
	box *secrets.Box,
	createdBy *uuid.UUID,
) ([]*models.MCPServer, error) {
	var file mcpJSONFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, ctxerrors.Wrap(err, "unmarshal .mcp.json")
	}

	if len(file.MCPServers) == 0 {
		return nil, ErrNoServers
	}

	names := make([]string, 0, len(file.MCPServers))
	for name := range file.MCPServers {
		names = append(names, name)
	}

	sort.Strings(names)

	servers := make([]*models.MCPServer, 0, len(names))

	for _, name := range names {
		server, err := file.MCPServers[name].toModel(name, box, createdBy)
		if err != nil {
			return nil, ctxerrors.Wrapf(err, "server %q", name)
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// toModel converts one .mcp.json entry into an MCPServer. command wins over url
// when both are present (a stdio server can't also be http).
func (e mcpJSONServer) toModel(
	name string,
	box *secrets.Box,
	createdBy *uuid.UUID,
) (*models.MCPServer, error) {
	server := &models.MCPServer{
		Name:      name,
		Enabled:   true,
		CreatedBy: createdBy,
	}

	switch {
	case e.Command != "":
		server.Transport = models.MCPTransportStdio
		server.Command = e.Command

		if len(e.Args) > 0 {
			argsJSON, err := json.Marshal(e.Args)
			if err != nil {
				return nil, ctxerrors.Wrap(err, "marshal args")
			}

			server.Args = datatypes.JSON(argsJSON)
		}

		envEnc, err := box.SealMap(e.Env)
		if err != nil {
			return nil, ctxerrors.Wrap(err, "seal env")
		}

		server.EnvEnc = envEnc
	case e.URL != "":
		server.Transport = models.MCPTransportHTTP
		server.URL = e.URL

		headersEnc, err := box.SealMap(e.Headers)
		if err != nil {
			return nil, ctxerrors.Wrap(err, "seal headers")
		}

		server.HeadersEnc = headersEnc
	default:
		return nil, ErrInvalidServer
	}

	return server, nil
}
