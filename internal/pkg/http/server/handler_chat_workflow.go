package server

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/core/chats"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

// DeleteChat soft-deletes one caller-owned chat.
func (s *Server) DeleteChat(
	ctx context.Context,
	req api.DeleteChatRequestObject,
) (api.DeleteChatResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.deps.Chats.Delete(ctx, req.ChatId, user.ID); err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.DeleteChat404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("chat not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "delete chat")
	}

	return api.DeleteChat204Response{}, nil
}

// PinChat puts one caller-owned chat ahead of unpinned chats.
func (s *Server) PinChat(
	ctx context.Context,
	req api.PinChatRequestObject,
) (api.PinChatResponseObject, error) {
	chat, err := s.setChatPinned(ctx, req.ChatId, true)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.PinChat404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("chat not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "pin chat")
	}

	return api.PinChat200JSONResponse(chats.ChatSummaryToAPI(chat)), nil
}

// UnpinChat returns one caller-owned chat to normal ordering.
func (s *Server) UnpinChat(
	ctx context.Context,
	req api.UnpinChatRequestObject,
) (api.UnpinChatResponseObject, error) {
	chat, err := s.setChatPinned(ctx, req.ChatId, false)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.UnpinChat404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("chat not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "unpin chat")
	}

	return api.UnpinChat200JSONResponse(chats.ChatSummaryToAPI(chat)), nil
}

func (s *Server) setChatPinned(
	ctx context.Context,
	chatID uuid.UUID,
	pinned bool,
) (*models.Chat, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	chat, err := s.deps.Chats.SetPinned(ctx, chatID, user.ID, pinned)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "set chat pin state")
	}

	return chat, nil
}

func chatWorkflowNotFound(message string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(
		envelope(aichteeteapee.ErrorCodeNotFound, message),
	)
}
