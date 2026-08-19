package server

import (
	"context"
	"errors"

	"github.com/psyb0t/aichteeteapee"
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

func chatWorkflowNotFound(message string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse(
		envelope(aichteeteapee.ErrorCodeNotFound, message),
	)
}
