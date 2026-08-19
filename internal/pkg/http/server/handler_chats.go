package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/core/chats"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
)

// Pagination bounds for the chats/messages list endpoints. The OpenAPI
// min/max/default are advisory (no runtime validator on query params — only
// on request bodies), so the handler enforces them before calling the
// service.
const (
	defaultChatsLimit    = 100
	maxChatsLimit        = 100
	defaultMessagesLimit = 50
	maxMessagesLimit     = 200
	minLimit             = 1
	minOffset            = 0
)

// paginationParams resolves limit/offset query params against their
// defaults, returning a client-facing message and false on the first
// boundary violation (garbage limit/offset must 400, never silently clamp).
func paginationParams(
	limitParam, offsetParam *int,
	defaultLimit, maxLimit int,
) (int, int, string, bool) {
	limit := defaultLimit
	if limitParam != nil {
		limit = *limitParam
	}

	offset := 0
	if offsetParam != nil {
		offset = *offsetParam
	}

	if limit < minLimit || limit > maxLimit {
		return 0, 0, fmt.Sprintf(
			"limit must be between %d and %d", minLimit, maxLimit,
		), false
	}

	if offset < minOffset {
		return 0, 0, "offset must be 0 or greater", false
	}

	return limit, offset, "", true
}

// ListChats lists a page of the caller's chats, newest activity first.
func (s *Server) ListChats(
	ctx context.Context,
	req api.ListChatsRequestObject,
) (api.ListChatsResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	limit, offset, msg, ok := paginationParams(
		req.Params.Limit, req.Params.Offset, defaultChatsLimit, maxChatsLimit,
	)
	if !ok {
		return listChatsBadRequest(msg), nil
	}

	if req.Params.Search != nil &&
		len([]rune(*req.Params.Search)) > chats.MaxChatSearchRunes {
		return listChatsBadRequest("search is too long"), nil
	}

	options := chats.ListOptions{}
	if req.Params.Search != nil {
		options.Search = *req.Params.Search
	}

	list, total, err := s.deps.Chats.ListWithOptions(
		ctx,
		user.ID,
		options,
		limit,
		offset,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list chats")
	}

	items := make([]api.ChatSummary, 0, len(list))
	for _, c := range list {
		items = append(items, chats.ChatSummaryToAPI(c))
	}

	return api.ListChats200JSONResponse(api.ChatList{
		Items:  items,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}), nil
}

func listChatsBadRequest(message string) api.ListChats400JSONResponse {
	return api.ListChats400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeValidationFailed, message),
		),
	}
}

// GetOrCreateEmptyChat returns the caller's reusable "empty chat" (creating
// one if none exists) — see chats.Service.GetOrCreateEmpty.
func (s *Server) GetOrCreateEmptyChat(
	ctx context.Context,
	_ api.GetOrCreateEmptyChatRequestObject,
) (api.GetOrCreateEmptyChatResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	chat, err := s.deps.Chats.GetOrCreateEmpty(ctx, user.ID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "get or create empty chat")
	}

	return api.GetOrCreateEmptyChat200JSONResponse(api.Chat{
		Id:       chat.ID,
		Title:    chat.Title,
		Model:    chat.ModelID,
		Settings: chats.ChatSettingsToAPI(chat),
	}), nil
}

// GetChat returns a chat's metadata (ownership-checked). Messages are fetched
// separately via ListChatMessages.
func (s *Server) GetChat(
	ctx context.Context,
	req api.GetChatRequestObject,
) (api.GetChatResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	chat, err := s.deps.Chats.Get(ctx, req.ChatId, user.ID)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return chatNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "get chat")
	}

	return api.GetChat200JSONResponse(api.Chat{
		Id:       chat.ID,
		Title:    chat.Title,
		Model:    chat.ModelID,
		Settings: chats.ChatSettingsToAPI(chat),
	}), nil
}

func chatNotFound() api.GetChat404JSONResponse {
	return api.GetChat404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

// ListChatMessages lists a page of a chat's messages, oldest-first
// (ownership-checked).
func (s *Server) ListChatMessages(
	ctx context.Context,
	req api.ListChatMessagesRequestObject,
) (api.ListChatMessagesResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	limit, offset, msg, ok := paginationParams(
		req.Params.Limit, req.Params.Offset,
		defaultMessagesLimit, maxMessagesLimit,
	)
	if !ok {
		return listChatMessagesBadRequest(msg), nil
	}

	msgs, total, err := s.deps.Chats.ListMessages(
		ctx, req.ChatId, user.ID, limit, offset,
	)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return listChatMessagesNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "list chat messages")
	}

	logger := ctxscope.GetLogger(ctx)

	items := make([]api.Message, 0, len(msgs))
	for _, m := range msgs {
		item, err := chats.MessageToAPI(m)
		if err != nil {
			logger.Warn("skip malformed message in history",
				"message_id", m.ID, "chat_id", req.ChatId,
				"err", err, "reason", "tool_calls_unmarshal_failed")

			continue
		}

		items = append(items, item)
	}

	return api.ListChatMessages200JSONResponse(api.MessageList{
		Items:  items,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}), nil
}

func listChatMessagesBadRequest(
	message string,
) api.ListChatMessages400JSONResponse {
	return api.ListChatMessages400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeValidationFailed, message),
		),
	}
}

func listChatMessagesNotFound() api.ListChatMessages404JSONResponse {
	return api.ListChatMessages404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

// CreateChat creates a chat and streams the first assistant turn (SSE). The new
// chat id rides in the message_start event's stream_id.
func (s *Server) CreateChat(
	ctx context.Context,
	req api.CreateChatRequestObject,
) (api.CreateChatResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil || req.Body.Message == "" || req.Body.Model == "" {
		return createChatBadRequest("model and message are required"), nil
	}

	body, err := s.deps.Chats.Create(
		ctx, user.ID, req.Body.Message, req.Body.Model,
	)
	if err != nil {
		if errors.Is(err, chats.ErrUnknownModel) {
			return createChatBadRequest("unknown model"), nil
		}

		return nil, ctxerrors.Wrap(err, "create chat")
	}

	return api.CreateChat200TexteventStreamResponse{Body: body}, nil
}

// ContinueChat continues a chat and streams the next assistant turn (SSE).
func (s *Server) ContinueChat(
	ctx context.Context,
	req api.ContinueChatRequestObject,
) (api.ContinueChatResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil || req.Body.Message == "" {
		return continueChatBadRequest("message is required"), nil
	}

	body, err := s.deps.Chats.Continue(
		ctx, req.ChatId, user.ID, req.Body.Message, req.Body.Model,
	)
	if err != nil {
		switch {
		case errors.Is(err, commerr.ErrNotFound):
			return continueChatNotFound(), nil
		case errors.Is(err, chats.ErrUnknownModel):
			return continueChatBadRequest("model unavailable"), nil
		}

		return nil, ctxerrors.Wrap(err, "continue chat")
	}

	return api.ContinueChat200TexteventStreamResponse{Body: body}, nil
}

func createChatBadRequest(message string) api.CreateChat400JSONResponse {
	return api.CreateChat400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

func continueChatBadRequest(message string) api.ContinueChat400JSONResponse {
	return api.ContinueChat400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

func continueChatNotFound() api.ContinueChat404JSONResponse {
	return api.ContinueChat404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

// RenameChat sets a chat's title (ownership-checked).
func (s *Server) RenameChat(
	ctx context.Context,
	req api.RenameChatRequestObject,
) (api.RenameChatResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil || strings.TrimSpace(req.Body.Title) == "" {
		return renameChatBadRequest("title is required"), nil
	}

	chat, err := s.deps.Chats.Rename(ctx, req.ChatId, user.ID, req.Body.Title)
	if err != nil {
		switch {
		case errors.Is(err, commerr.ErrNotFound):
			return renameChatNotFound(), nil
		case errors.Is(err, chats.ErrEmptyTitle):
			return renameChatBadRequest("title must not be empty"), nil
		}

		return nil, ctxerrors.Wrap(err, "rename chat")
	}

	return api.RenameChat200JSONResponse(chats.ChatSummaryToAPI(chat)), nil
}

func renameChatBadRequest(message string) api.RenameChat400JSONResponse {
	return api.RenameChat400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

func renameChatNotFound() api.RenameChat404JSONResponse {
	return api.RenameChat404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

// Bounds for the per-chat generation settings; the OpenAPI min/max are advisory
// (no runtime validator), so the handler enforces them before persisting.
const (
	minTemperature = 0.0
	maxTemperature = 2.0
	minTopP        = 0.0
	maxTopP        = 1.0
	minTokenCount  = 1
)

// UpdateChatSettings replaces a chat's model-generation settings (full
// replacement — an omitted field clears that setting).
func (s *Server) UpdateChatSettings(
	ctx context.Context,
	req api.UpdateChatSettingsRequestObject,
) (api.UpdateChatSettingsResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return updateChatSettingsBadRequest("settings body is required"), nil
	}

	if msg, ok := validateChatSettings(req.Body); !ok {
		return updateChatSettingsValidationFailed(msg), nil
	}

	chat, err := s.deps.Chats.UpdateSettings(
		ctx, req.ChatId, user.ID, toChatSettings(req.Body),
	)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return updateChatSettingsNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "update chat settings")
	}

	return api.UpdateChatSettings200JSONResponse(
		*chats.ChatSettingsToAPI(chat),
	), nil
}

// PreviewChatContext returns the exact prompt context selection for an unsent
// message. The service owns transcript loading, tokenization, and selection.
func (s *Server) PreviewChatContext(
	ctx context.Context,
	req api.PreviewChatContextRequestObject,
) (api.PreviewChatContextResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return previewChatContextBadRequest(
			"context preview body is required",
		), nil
	}

	preview, err := s.deps.Chats.PreviewContext(
		ctx,
		req.ChatId,
		user.ID,
		req.Body.Message,
	)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return previewChatContextNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "preview chat context")
	}

	return api.PreviewChatContext200JSONResponse(
		chats.PromptContextToAPI(preview),
	), nil
}

// validateChatSettings enforces the per-field bounds, returning a client-facing
// message and false on the first violation.
func validateChatSettings(b *api.ChatSettings) (string, bool) {
	if msg, ok := validateSampling(b); !ok {
		return msg, false
	}

	if msg, ok := validateTokenCaps(b); !ok {
		return msg, false
	}

	if b.ReasoningEffort != nil && !validReasoningEffort(*b.ReasoningEffort) {
		return "reasoningEffort must be minimal, low, medium, or high", false
	}

	return "", true
}

// validateSampling bounds the temperature + top_p knobs.
func validateSampling(b *api.ChatSettings) (string, bool) {
	if b.Temperature != nil &&
		(*b.Temperature < minTemperature || *b.Temperature > maxTemperature) {
		return "temperature must be between 0 and 2", false
	}

	if b.TopP != nil && (*b.TopP < minTopP || *b.TopP > maxTopP) {
		return "topP must be between 0 and 1", false
	}

	return "", true
}

// validateTokenCaps bounds the max-output + max-history token caps.
func validateTokenCaps(b *api.ChatSettings) (string, bool) {
	if b.MaxOutputTokens != nil && *b.MaxOutputTokens < minTokenCount {
		return "maxOutputTokens must be a positive integer", false
	}

	if b.MaxHistoryTokens != nil && *b.MaxHistoryTokens < minTokenCount {
		return "maxHistoryTokens must be a positive integer", false
	}

	return "", true
}

func validReasoningEffort(e api.ChatSettingsReasoningEffort) bool {
	switch e {
	case api.ChatSettingsReasoningEffortMinimal,
		api.ChatSettingsReasoningEffortLow,
		api.ChatSettingsReasoningEffortMedium,
		api.ChatSettingsReasoningEffortHigh:
		return true
	default:
		return false
	}
}

// toChatSettings maps the validated request body to the domain settings; a nil
// pointer / absent field stays unset.
func toChatSettings(b *api.ChatSettings) chats.Settings {
	out := chats.Settings{
		Temperature:      b.Temperature,
		TopP:             b.TopP,
		MaxOutputTokens:  b.MaxOutputTokens,
		MaxHistoryTokens: b.MaxHistoryTokens,
	}

	if b.ReasoningEffort != nil {
		out.ReasoningEffort = string(*b.ReasoningEffort)
	}

	return out
}

func updateChatSettingsBadRequest(
	message string,
) api.UpdateChatSettings400JSONResponse {
	return api.UpdateChatSettings400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

// updateChatSettingsValidationFailed reports a well-formed settings field
// whose value is outside its allowed range (as opposed to a structurally
// missing body, which stays aichteeteapee.ErrorCodeBadRequest).
func updateChatSettingsValidationFailed(
	message string,
) api.UpdateChatSettings400JSONResponse {
	return api.UpdateChatSettings400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeValidationFailed, message),
		),
	}
}

func updateChatSettingsNotFound() api.UpdateChatSettings404JSONResponse {
	return api.UpdateChatSettings404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

func previewChatContextBadRequest(
	message string,
) api.PreviewChatContext400JSONResponse {
	return api.PreviewChatContext400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

func previewChatContextNotFound() api.PreviewChatContext404JSONResponse {
	return api.PreviewChatContext404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

// ListChatMCPServers lists the MCP servers with each one's live status and
// whether it is enabled for this chat (ownership-checked).
func (s *Server) ListChatMCPServers(
	ctx context.Context,
	req api.ListChatMCPServersRequestObject,
) (api.ListChatMCPServersResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	servers, err := s.deps.Chats.ListMCPServers(ctx, req.ChatId, user.ID)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return listChatMCPServersNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "list chat mcp servers")
	}

	out := make(api.ListChatMCPServers200JSONResponse, 0, len(servers))
	for _, sv := range servers {
		out = append(out, chats.ChatMCPServerToAPI(sv))
	}

	return out, nil
}

func listChatMCPServersNotFound() api.ListChatMCPServers404JSONResponse {
	return api.ListChatMCPServers404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat not found"),
		),
	}
}

// UpdateChatMCPServer enables or disables one MCP server for this chat
// (ownership-checked).
func (s *Server) UpdateChatMCPServer(
	ctx context.Context,
	req api.UpdateChatMCPServerRequestObject,
) (api.UpdateChatMCPServerResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return updateChatMCPServerBadRequest("enabled is required"), nil
	}

	sv, err := s.deps.Chats.SetMCPServerEnabled(
		ctx, req.ChatId, user.ID, req.ServerId, req.Body.Enabled,
	)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return updateChatMCPServerNotFound(), nil
		}

		return nil, ctxerrors.Wrap(err, "update chat mcp server")
	}

	return api.UpdateChatMCPServer200JSONResponse(
		chats.ChatMCPServerToAPI(*sv),
	), nil
}

func updateChatMCPServerBadRequest(
	message string,
) api.UpdateChatMCPServer400JSONResponse {
	return api.UpdateChatMCPServer400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

func updateChatMCPServerNotFound() api.UpdateChatMCPServer404JSONResponse {
	return api.UpdateChatMCPServer404JSONResponse{
		NotFoundJSONResponse: api.NotFoundJSONResponse(
			envelope(aichteeteapee.ErrorCodeNotFound, "chat/server not found"),
		),
	}
}
