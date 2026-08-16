package chats

import (
	"context"
	"errors"

	"github.com/google/uuid"
	chatcore "github.com/psyb0t/chatz/internal/pkg/core/chat"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ChatMCPServer is one MCP server as a chat sees it: identity, live global
// connection status, and whether its tools are enabled for this chat. Per-chat
// enablement is independent of the server's global enabled flag.
type ChatMCPServer struct {
	ID      uuid.UUID
	Name    string
	Status  string
	Enabled bool
}

// ListMCPServers returns every MCP server with its live global status and
// whether it is enabled for this chat (ownership-checked). Returns
// commerr.ErrNotFound when the chat is missing or owned by another user.
func (s *Service) ListMCPServers(
	ctx context.Context,
	chatID, userID uuid.UUID,
) ([]ChatMCPServer, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	repo := s.query.MCPServer

	rows, err := repo.WithContext(ctx).Order(repo.Name).Find()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list mcp servers")
	}

	disabled := disabledSet(chat)

	out := make([]ChatMCPServer, 0, len(rows))
	for _, srv := range rows {
		out = append(out, s.chatMCPServerView(srv, disabled))
	}

	return out, nil
}

// SetMCPServerEnabled enables or disables one MCP server's tools for a chat
// (ownership-checked). Returns commerr.ErrNotFound when the chat or the
// server does not exist.
func (s *Service) SetMCPServerEnabled(
	ctx context.Context,
	chatID, userID, serverID uuid.UUID,
	enabled bool,
) (*ChatMCPServer, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	repo := s.query.MCPServer

	srv, err := repo.WithContext(ctx).Where(repo.ID.Eq(serverID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, commerr.ErrNotFound
		}

		return nil, ctxerrors.Wrap(err, "get mcp server")
	}

	chat.DisabledMCPServerIDs = withServer(
		chat.DisabledMCPServerIDs, serverID, enabled,
	)

	// Explicit Select so an emptied set persists as '[]' (Updates skips a
	// zero-value field otherwise, silently ignoring "re-enable the last one").
	chatRepo := s.query.Chat
	if _, err := chatRepo.WithContext(ctx).
		Where(chatRepo.ID.Eq(chatID)).
		Select(chatRepo.DisabledMCPServerIDs).
		Updates(chat); err != nil {
		return nil, ctxerrors.Wrap(err, "update chat mcp servers")
	}

	view := s.chatMCPServerView(srv, disabledSet(chat))

	return &view, nil
}

// disabledSet builds the chat's disabled-server-id lookup.
func disabledSet(chat *models.Chat) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(chat.DisabledMCPServerIDs))
	for _, id := range chat.DisabledMCPServerIDs {
		set[id] = true
	}

	return set
}

// withServer returns the disabled-id set with serverID removed (enabled) or
// added (disabled). Always a non-nil slice so an empty set persists as '[]'
// rather than NULL.
func withServer(
	ids datatypes.JSONSlice[uuid.UUID],
	serverID uuid.UUID,
	enabled bool,
) datatypes.JSONSlice[uuid.UUID] {
	out := make([]uuid.UUID, 0, len(ids)+1)

	for _, id := range ids {
		if id != serverID {
			out = append(out, id)
		}
	}

	if !enabled {
		out = append(out, serverID)
	}

	return datatypes.NewJSONSlice(out)
}

// chatMCPServerView projects a server row to the chat's view of it.
func (s *Service) chatMCPServerView(
	srv *models.MCPServer,
	disabled map[uuid.UUID]bool,
) ChatMCPServer {
	return ChatMCPServer{
		ID:      srv.ID,
		Name:    srv.Name,
		Status:  s.serverStatus(srv),
		Enabled: !disabled[srv.ID],
	}
}

// serverStatus resolves a server's live status: a globally-disabled server is
// "disabled"; otherwise the manager's connection state (defaulting to
// "connecting" before the first async connect settles).
func (s *Service) serverStatus(srv *models.MCPServer) string {
	if !srv.Enabled {
		return string(mcp.StateDisabled)
	}

	st := s.mcp.Status(srv.Name)
	if st.State == "" {
		return string(mcp.StateConnecting)
	}

	return string(st.State)
}

// toolExecutorFor returns the tool executor a chat's turns run against. With no
// per-chat disabled servers it's the MCP manager unchanged; otherwise it's a
// filter that hides tools belonging to the servers this chat disabled (resolved
// from IDs to server names once, up front). The model never sees a disabled
// server's tools, so it never calls them.
//

func (s *Service) toolExecutorFor(
	ctx context.Context,
	chat *models.Chat,
) chatcore.ToolExecutor {
	if len(chat.DisabledMCPServerIDs) == 0 {
		return s.mcp
	}

	blocked := s.disabledServerNames(ctx, chat)
	if len(blocked) == 0 {
		return s.mcp
	}

	return filteredToolExecutor{inner: s.mcp, blockedServers: blocked}
}

// disabledServerNames resolves a chat's disabled MCP server IDs to the server
// names that key the tool namespace. A lookup error degrades to no filtering
// with a warning: the disable is a per-chat preference over admin-approved
// servers, not a security boundary, so a DB hiccup must not abort the turn.
func (s *Service) disabledServerNames(
	ctx context.Context,
	chat *models.Chat,
) map[string]bool {
	rows, err := s.query.MCPServer.WithContext(ctx).Find()
	if err != nil {
		ctxscope.GetLogger(ctx).Warn(
			"resolve disabled mcp servers failed, not filtering",
			"chat_id", chat.ID,
			"err", err,
			"reason", "mcp_disabled_resolve_failed",
		)

		return nil
	}

	disabled := make(map[uuid.UUID]bool, len(chat.DisabledMCPServerIDs))
	for _, id := range chat.DisabledMCPServerIDs {
		disabled[id] = true
	}

	blocked := make(map[string]bool, len(disabled))
	for _, r := range rows {
		if disabled[r.ID] {
			blocked[r.Name] = true
		}
	}

	return blocked
}

// filteredToolExecutor wraps a ToolExecutor, hiding every tool whose server is
// in blockedServers. Call passes straight through — the model only receives the
// filtered tool list, so a hidden server's tools are never requested.
type filteredToolExecutor struct {
	inner          chatcore.ToolExecutor
	blockedServers map[string]bool
}

func (f filteredToolExecutor) Tools(ctx context.Context) []mcp.Tool {
	all := f.inner.Tools(ctx)

	out := make([]mcp.Tool, 0, len(all))
	for _, t := range all {
		if f.blockedServers[t.Server] {
			continue
		}

		out = append(out, t)
	}

	return out
}

func (f filteredToolExecutor) Call(
	ctx context.Context,
	qualifiedName string,
	args map[string]any,
) (*mcp.ToolResult, error) {
	//nolint:wrapcheck // transparent passthrough to the wrapped executor
	return f.inner.Call(ctx, qualifiedName, args)
}
