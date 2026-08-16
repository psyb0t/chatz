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

// ArchiveChat moves one caller-owned chat out of the active list.
func (s *Server) ArchiveChat(
	ctx context.Context,
	req api.ArchiveChatRequestObject,
) (api.ArchiveChatResponseObject, error) {
	chat, err := s.setChatArchived(ctx, req.ChatId, true)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.ArchiveChat404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("chat not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "archive chat")
	}

	return api.ArchiveChat200JSONResponse(chats.ChatSummaryToAPI(chat)), nil
}

// UnarchiveChat restores one caller-owned chat to the active list.
func (s *Server) UnarchiveChat(
	ctx context.Context,
	req api.UnarchiveChatRequestObject,
) (api.UnarchiveChatResponseObject, error) {
	chat, err := s.setChatArchived(ctx, req.ChatId, false)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.UnarchiveChat404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("chat not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "unarchive chat")
	}

	return api.UnarchiveChat200JSONResponse(chats.ChatSummaryToAPI(chat)), nil
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

func (s *Server) setChatArchived(
	ctx context.Context,
	chatID uuid.UUID,
	archived bool,
) (*models.Chat, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	chat, err := s.deps.Chats.SetArchived(ctx, chatID, user.ID, archived)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "set chat archive state")
	}

	return chat, nil
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

func chatWorkflowBadRequest(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse(
		envelope(aichteeteapee.ErrorCodeValidationFailed, message),
	)
}

func chatWorkflowConflict(message string) api.ConflictJSONResponse {
	return api.ConflictJSONResponse(
		envelope(aichteeteapee.ErrorCodeConflict, message),
	)
}

// AssignChatProject attaches one caller-owned chat to one caller-owned
// project.
func (s *Server) AssignChatProject(
	ctx context.Context,
	req api.AssignChatProjectRequestObject,
) (api.AssignChatProjectResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return api.AssignChatProject400JSONResponse{
			BadRequestJSONResponse: chatWorkflowBadRequest(
				"projectId is required",
			),
		}, nil
	}

	chat, err := s.deps.Chats.AssignProject(
		ctx,
		req.ChatId,
		req.Body.ProjectId,
		user.ID,
	)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.AssignChatProject404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound(
					"chat or project not found",
				),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "assign chat project")
	}

	return api.AssignChatProject200JSONResponse(
		chats.ChatSummaryToAPI(chat),
	), nil
}

// ClearChatProject removes one caller-owned chat's project assignment.
func (s *Server) ClearChatProject(
	ctx context.Context,
	req api.ClearChatProjectRequestObject,
) (api.ClearChatProjectResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	chat, err := s.deps.Chats.ClearProject(ctx, req.ChatId, user.ID)
	if err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.ClearChatProject404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("chat not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "clear chat project")
	}

	return api.ClearChatProject200JSONResponse(
		chats.ChatSummaryToAPI(chat),
	), nil
}

// ListProjects returns the caller's projects in stable name order.
func (s *Server) ListProjects(
	ctx context.Context,
	_ api.ListProjectsRequestObject,
) (api.ListProjectsResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	projects, err := s.deps.Chats.ListProjects(ctx, user.ID)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list projects")
	}

	response := make(api.ListProjects200JSONResponse, 0, len(projects))
	for _, project := range projects {
		response = append(response, chats.ProjectToAPI(project))
	}

	return response, nil
}

// CreateProject creates a caller-owned project.
func (s *Server) CreateProject(
	ctx context.Context,
	req api.CreateProjectRequestObject,
) (api.CreateProjectResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return api.CreateProject400JSONResponse{
			BadRequestJSONResponse: chatWorkflowBadRequest(
				"project name is required",
			),
		}, nil
	}

	project, err := s.deps.Chats.CreateProject(ctx, user.ID, req.Body.Name)
	if err != nil {
		switch {
		case errors.Is(err, commerr.ErrInvalidArgument):
			return api.CreateProject400JSONResponse{
				BadRequestJSONResponse: chatWorkflowBadRequest(
					"invalid project name",
				),
			}, nil
		case errors.Is(err, commerr.ErrConflict):
			return api.CreateProject409JSONResponse{
				ConflictJSONResponse: chatWorkflowConflict(
					"project name already exists",
				),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "create project")
	}

	return api.CreateProject201JSONResponse(chats.ProjectToAPI(project)), nil
}

// RenameProject changes a caller-owned project's name.
func (s *Server) RenameProject(
	ctx context.Context,
	req api.RenameProjectRequestObject,
) (api.RenameProjectResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if req.Body == nil {
		return api.RenameProject400JSONResponse{
			BadRequestJSONResponse: chatWorkflowBadRequest(
				"project name is required",
			),
		}, nil
	}

	project, err := s.deps.Chats.RenameProject(
		ctx,
		req.ProjectId,
		user.ID,
		req.Body.Name,
	)
	if err != nil {
		switch {
		case errors.Is(err, commerr.ErrInvalidArgument):
			return api.RenameProject400JSONResponse{
				BadRequestJSONResponse: chatWorkflowBadRequest(
					"invalid project name",
				),
			}, nil
		case errors.Is(err, commerr.ErrNotFound):
			return api.RenameProject404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("project not found"),
			}, nil
		case errors.Is(err, commerr.ErrConflict):
			return api.RenameProject409JSONResponse{
				ConflictJSONResponse: chatWorkflowConflict(
					"project name already exists",
				),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "rename project")
	}

	return api.RenameProject200JSONResponse(chats.ProjectToAPI(project)), nil
}

// DeleteProject removes a caller-owned project while preserving its chats.
func (s *Server) DeleteProject(
	ctx context.Context,
	req api.DeleteProjectRequestObject,
) (api.DeleteProjectResponseObject, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.deps.Chats.DeleteProject(
		ctx,
		req.ProjectId,
		user.ID,
	); err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.DeleteProject404JSONResponse{
				NotFoundJSONResponse: chatWorkflowNotFound("project not found"),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "delete project")
	}

	return api.DeleteProject204Response{}, nil
}
