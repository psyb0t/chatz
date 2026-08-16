package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
	"gorm.io/datatypes"
)

const maskedAuthorizationValue = "[SECRET]"

// ListMCPServers lists configured MCP servers. Admin only.
func (s *Server) ListMCPServers(
	ctx context.Context,
	_ api.ListMCPServersRequestObject,
) (api.ListMCPServersResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	rows, err := s.deps.MCPServers.List(ctx)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list mcp servers")
	}

	out := make(api.ListMCPServers200JSONResponse, 0, len(rows))
	for _, m := range rows {
		out = append(out, s.mcpServerResponse(ctx, m))
	}

	return out, nil
}

// CreateMCPServer adds an MCP server, sealing its secrets, persisting it, and
// connecting. Admin only.
func (s *Server) CreateMCPServer(
	ctx context.Context,
	req api.CreateMCPServerRequestObject,
) (api.CreateMCPServerResponseObject, error) {
	admin, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return createMCPBadRequest("missing body"), nil
	}

	srv, err := s.mcpServerFromRequest(req.Body, admin.ID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build mcp server")
	}

	if err := s.deps.MCPServers.Create(ctx, srv); err != nil {
		return nil, ctxerrors.Wrap(err, "persist mcp server")
	}

	s.connectMCPAsync(ctx, srv)

	return api.CreateMCPServer201JSONResponse(
		s.mcpServerResponse(ctx, srv),
	), nil
}

func createMCPBadRequest(message string) api.CreateMCPServer400JSONResponse {
	return api.CreateMCPServer400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

// DeleteMCPServer removes an MCP server. Admin only.
func (s *Server) DeleteMCPServer(
	ctx context.Context,
	req api.DeleteMCPServerRequestObject,
) (api.DeleteMCPServerResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	srv, err := s.deps.MCPServers.Get(ctx, req.ServerId)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.DeleteMCPServer404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(
					envelope(
						aichteeteapee.ErrorCodeNotFound,
						"server not found",
					),
				),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "get mcp server")
	}

	if err := s.deps.MCPServers.Delete(ctx, req.ServerId); err != nil {
		return nil, ctxerrors.Wrap(err, "delete mcp server")
	}

	if s.deps.MCP != nil {
		s.deps.MCP.Remove(ctx, srv.Name)
	}

	return api.DeleteMCPServer204Response{}, nil
}

// ImportMCP imports MCP servers from a Claude-style .mcp.json. Admin only.
func (s *Server) ImportMCP(
	ctx context.Context,
	req api.ImportMCPRequestObject,
) (api.ImportMCPResponseObject, error) {
	admin, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return importMCPBadRequest("missing body"), nil
	}

	servers, err := mcp.ParseMCPJSON(
		[]byte(req.Body.Content), s.deps.Secrets, &admin.ID,
	)
	if err != nil {
		if errors.Is(err, mcp.ErrNoServers) ||
			errors.Is(err, mcp.ErrInvalidServer) {
			return importMCPBadRequest(err.Error()), nil
		}

		return nil, ctxerrors.Wrap(err, "parse .mcp.json")
	}

	imported := make([]api.MCPServer, 0, len(servers))

	for _, srv := range servers {
		if err := s.deps.MCPServers.Create(ctx, srv); err != nil {
			return nil, ctxerrors.Wrapf(err, "persist mcp server %q", srv.Name)
		}

		s.connectMCPAsync(ctx, srv)
		imported = append(imported, s.mcpServerResponse(ctx, srv))
	}

	return api.ImportMCP200JSONResponse(api.MCPImportResult{
		Imported: imported,
	}), nil
}

func importMCPBadRequest(message string) api.ImportMCP400JSONResponse {
	return api.ImportMCP400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

// connectMCPAsync kicks off a background connect for a persisted server so the
// request returns immediately. The manager records connecting → connected/
// failed status; a failure never blocks or drops the saved row.
func (s *Server) connectMCPAsync(ctx context.Context, srv *models.MCPServer) {
	if s.deps.MCP == nil {
		return
	}

	s.deps.MCP.ConnectAsync(ctx, srv)
}

// mcpStatus reads a server's live connection status, guarding a nil manager
// (returns the zero Status, which the response layer maps by enabled-ness).
func (s *Server) mcpStatus(name string) mcp.Status {
	if s.deps.MCP == nil {
		return mcp.Status{}
	}

	return s.deps.MCP.Status(name)
}

// mcpServerResponse builds the admin API view of a server. Authorization header
// values stay sealed at rest and are masked before reaching the edit form.
func (s *Server) mcpServerResponse(
	ctx context.Context,
	srv *models.MCPServer,
) api.MCPServer {
	out := mcp.ServerToAPI(srv, s.mcpStatus(srv.Name))

	if args := parseMCPArgs(ctx, srv); len(args) > 0 {
		out.Args = &args
	}

	env, headers := s.mcpSecrets(ctx, srv)
	if len(env) > 0 {
		out.Env = &env
	}

	if len(headers) > 0 {
		maskedHeaders := maskAuthorizationHeaders(headers)
		out.Headers = &maskedHeaders
	}

	return out
}

func maskAuthorizationHeaders(headers map[string]string) map[string]string {
	maskedHeaders := make(map[string]string, len(headers))
	for name, value := range headers {
		if strings.EqualFold(name, aichteeteapee.HeaderNameAuthorization) {
			maskedHeaders[name] = maskedAuthorizationHeaderValue(value)

			continue
		}

		maskedHeaders[name] = value
	}

	return maskedHeaders
}

func maskedAuthorizationHeaderValue(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return maskedAuthorizationValue
	}

	return parts[0] + " " + maskedAuthorizationValue
}

// parseMCPArgs decodes a server's stored stdio args (JSON array). A decode
// failure is logged and treated as no args rather than failing the response.
func parseMCPArgs(ctx context.Context, srv *models.MCPServer) []string {
	if len(srv.Args) == 0 {
		return nil
	}

	var args []string
	if err := json.Unmarshal(srv.Args, &args); err != nil {
		ctxscope.GetLogger(ctx).Warn("parse mcp args failed",
			"server", srv.Name, "err", err, "reason", "mcp_parse_args_failed")

		return nil
	}

	return args
}

// mcpSecrets decrypts a server's sealed env + headers for the admin edit form.
// A nil box (secrets disabled) or a decrypt failure yields an empty map rather
// than an error — the list must not break because one row won't decrypt.
func (s *Server) mcpSecrets(
	ctx context.Context,
	srv *models.MCPServer,
) (map[string]string, map[string]string) {
	if s.deps.Secrets == nil {
		return nil, nil
	}

	logger := ctxscope.GetLogger(ctx)

	env, err := s.deps.Secrets.OpenMap(srv.EnvEnc)
	if err != nil {
		logger.Warn("open mcp env failed",
			"server", srv.Name, "err", err, "reason", "mcp_open_env_failed")

		env = nil
	}

	headers, err := s.deps.Secrets.OpenMap(srv.HeadersEnc)
	if err != nil {
		logger.Warn("open mcp headers failed",
			"server", srv.Name, "err", err, "reason", "mcp_open_headers_failed")

		headers = nil
	}

	return env, headers
}

func (s *Server) mcpServerFromRequest(
	body *api.CreateMCPServerRequest,
	createdBy uuid.UUID,
) (*models.MCPServer, error) {
	srv := &models.MCPServer{
		Name:      body.Name,
		Transport: models.MCPTransport(body.Transport),
		Enabled:   true,
		CreatedBy: &createdBy,
	}

	switch body.Transport {
	case api.CreateMCPServerRequestTransportStdio:
		if err := s.applyStdio(srv, body); err != nil {
			return nil, err
		}
	case api.CreateMCPServerRequestTransportHttp:
		if err := s.applyHTTP(srv, body); err != nil {
			return nil, err
		}
	}

	return srv, nil
}

func (s *Server) applyStdio(
	srv *models.MCPServer,
	body *api.CreateMCPServerRequest,
) error {
	if body.Command != nil {
		srv.Command = *body.Command
	}

	if body.Args != nil {
		raw, err := json.Marshal(*body.Args)
		if err != nil {
			return ctxerrors.Wrap(err, "marshal args")
		}

		srv.Args = datatypes.JSON(raw)
	}

	if body.Env != nil {
		enc, err := s.deps.Secrets.SealMap(*body.Env)
		if err != nil {
			return ctxerrors.Wrap(err, "seal env")
		}

		srv.EnvEnc = enc
	}

	return nil
}

func (s *Server) applyHTTP(
	srv *models.MCPServer,
	body *api.CreateMCPServerRequest,
) error {
	if body.Url != nil {
		srv.URL = *body.Url
	}

	if body.Headers != nil {
		enc, err := s.deps.Secrets.SealMap(*body.Headers)
		if err != nil {
			return ctxerrors.Wrap(err, "seal headers")
		}

		srv.HeadersEnc = enc
	}

	return nil
}

// restoreMaskedAuthorizationHeaders restores the sealed Authorization value
// when an edit form submits the exact display mask it received. Omitting the
// header deletes it, while any other value intentionally replaces it.
func (s *Server) restoreMaskedAuthorizationHeaders(
	sealed []byte,
	requested map[string]string,
) (map[string]string, error) {
	existing, err := s.deps.Secrets.OpenMap(sealed)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "open existing headers")
	}

	merged := make(map[string]string, len(requested))
	for name, value := range requested {
		existingValue, found := authorizationHeaderValue(existing, name)
		if found && value == maskedAuthorizationHeaderValue(existingValue) {
			merged[name] = existingValue

			continue
		}

		merged[name] = value
	}

	return merged, nil
}

func authorizationHeaderValue(
	headers map[string]string,
	name string,
) (string, bool) {
	if !strings.EqualFold(name, aichteeteapee.HeaderNameAuthorization) {
		return "", false
	}

	for existingName, value := range headers {
		if strings.EqualFold(
			existingName,
			aichteeteapee.HeaderNameAuthorization,
		) {
			return value, true
		}
	}

	return "", false
}

// ReconnectMCPServer kicks off a background reconnect for one server. Admin
// only. Returns the server with its (connecting) status.
func (s *Server) ReconnectMCPServer(
	ctx context.Context,
	req api.ReconnectMCPServerRequestObject,
) (api.ReconnectMCPServerResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	srv, err := s.deps.MCPServers.Get(ctx, req.ServerId)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return reconnectMCPNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "get mcp server")
	}

	s.connectMCPAsync(ctx, srv)

	return api.ReconnectMCPServer200JSONResponse(
		s.mcpServerResponse(ctx, srv),
	), nil
}

func reconnectMCPNotFound() api.ReconnectMCPServer404JSONResponse {
	return api.ReconnectMCPServer404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "server not found"),
		),
	}
}

// UpdateMCPServer edits a server, reseals its secrets, persists, and reconnects
// (or disables when enabled=false). Admin only.
func (s *Server) UpdateMCPServer(
	ctx context.Context,
	req api.UpdateMCPServerRequestObject,
) (api.UpdateMCPServerResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if req.Body == nil {
		return updateMCPBadRequest("missing body"), nil
	}

	srv, err := s.deps.MCPServers.Get(ctx, req.ServerId)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return updateMCPNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "get mcp server")
	}

	oldName := srv.Name
	if err := s.applyMCPUpdate(srv, req.Body); err != nil {
		return nil, ctxerrors.Wrap(err, "apply mcp update")
	}

	if err := s.deps.MCPServers.Save(ctx, srv); err != nil {
		return nil, ctxerrors.Wrap(err, "persist mcp server")
	}

	s.rewireMCP(ctx, oldName, srv)

	return api.UpdateMCPServer200JSONResponse(
		s.mcpServerResponse(ctx, srv),
	), nil
}

// rewireMCP drops a server's prior live connection (its name may have changed)
// then reconnects it in the background or marks it disabled.
func (s *Server) rewireMCP(
	ctx context.Context,
	oldName string,
	srv *models.MCPServer,
) {
	if s.deps.MCP == nil {
		return
	}

	s.deps.MCP.Remove(ctx, oldName)

	if !srv.Enabled {
		s.deps.MCP.Disable(ctx, srv.Name)

		return
	}

	s.deps.MCP.ConnectAsync(ctx, srv)
}

func (s *Server) applyMCPUpdate(
	srv *models.MCPServer,
	body *api.UpdateMCPServerRequest,
) error {
	if body.Name != nil {
		srv.Name = *body.Name
	}

	if body.Transport != nil {
		srv.Transport = models.MCPTransport(*body.Transport)
	}

	if body.Command != nil {
		srv.Command = *body.Command
	}

	if body.Url != nil {
		srv.URL = *body.Url
	}

	if body.Enabled != nil {
		srv.Enabled = *body.Enabled
	}

	return s.applyMCPUpdateSecrets(srv, body)
}

func (s *Server) applyMCPUpdateSecrets(
	srv *models.MCPServer,
	body *api.UpdateMCPServerRequest,
) error {
	if body.Args != nil {
		raw, err := json.Marshal(*body.Args)
		if err != nil {
			return ctxerrors.Wrap(err, "marshal args")
		}

		srv.Args = datatypes.JSON(raw)
	}

	if body.Env != nil {
		enc, err := s.deps.Secrets.SealMap(*body.Env)
		if err != nil {
			return ctxerrors.Wrap(err, "seal env")
		}

		srv.EnvEnc = enc
	}

	if body.Headers != nil {
		headers, err := s.restoreMaskedAuthorizationHeaders(
			srv.HeadersEnc, *body.Headers,
		)
		if err != nil {
			return ctxerrors.Wrap(err, "restore masked authorization header")
		}

		enc, err := s.deps.Secrets.SealMap(headers)
		if err != nil {
			return ctxerrors.Wrap(err, "seal headers")
		}

		srv.HeadersEnc = enc
	}

	return nil
}

func updateMCPBadRequest(message string) api.UpdateMCPServer400JSONResponse {
	return api.UpdateMCPServer400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

func updateMCPNotFound() api.UpdateMCPServer404JSONResponse {
	return api.UpdateMCPServer404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "server not found"),
		),
	}
}

// ListMCPServerTools lists a connected server's tools (name/description/input
// schema). Admin only. A server that isn't currently connected yields an empty
// list — tools exist only while the server is live.
func (s *Server) ListMCPServerTools(
	ctx context.Context,
	req api.ListMCPServerToolsRequestObject,
) (api.ListMCPServerToolsResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	srv, err := s.deps.MCPServers.Get(ctx, req.ServerId)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return listMCPToolsNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "get mcp server")
	}

	tools, err := s.mcpServerTools(ctx, srv.Name)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list mcp server tools")
	}

	out := make(api.ListMCPServerTools200JSONResponse, 0, len(tools))
	for _, t := range tools {
		out = append(out, mcp.ToolToAPI(t))
	}

	return out, nil
}

func listMCPToolsNotFound() api.ListMCPServerTools404JSONResponse {
	return api.ListMCPServerTools404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "server not found"),
		),
	}
}

// mcpServerTools reads a server's live tools, guarding a nil manager.
func (s *Server) mcpServerTools(
	ctx context.Context,
	name string,
) ([]mcp.Tool, error) {
	if s.deps.MCP == nil {
		return nil, nil
	}

	tools, err := s.deps.MCP.ServerTools(ctx, name)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "server tools")
	}

	return tools, nil
}
