package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

// ListUsers lists all users. Admin only.
func (s *Server) ListUsers(
	ctx context.Context,
	_ api.ListUsersRequestObject,
) (api.ListUsersResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	users, err := s.deps.Auth.ListUsers(ctx)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list users")
	}

	out := make(api.ListUsers200JSONResponse, 0, len(users))
	for _, u := range users {
		out = append(out, auth.UserToAPI(u))
	}

	return out, nil
}

// CreateUser provisions a user. Admin only.
func (s *Server) CreateUser(
	ctx context.Context,
	req api.CreateUserRequestObject,
) (api.CreateUserResponseObject, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if req.Body == nil {
		return createUserBadRequest("missing body"), nil
	}

	isAdmin := req.Body.IsAdmin != nil && *req.Body.IsAdmin

	user, err := s.deps.Auth.CreateUser(
		ctx, req.Body.Username, req.Body.Password, isAdmin,
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserExists):
			return api.CreateUser409JSONResponse{
				ConflictJSONResponse: api.ConflictJSONResponse(
					envelope(
						aichteeteapee.ErrorCodeConflict,
						"username already exists",
					),
				),
			}, nil
		case errors.Is(err, auth.ErrInvalidInput):
			return createUserBadRequest("username and password required"), nil
		default:
			return nil, ctxerrors.Wrap(err, "create user")
		}
	}

	return api.CreateUser201JSONResponse(auth.UserToAPI(user)), nil
}

// DeleteUser removes a user; the DB cascades their sessions + chats. Admin
// only, and an admin cannot delete their own account (which would orphan the
// workspace / lock themselves out).
func (s *Server) DeleteUser(
	ctx context.Context,
	req api.DeleteUserRequestObject,
) (api.DeleteUserResponseObject, error) {
	admin, err := requireAdmin(ctx)
	if err != nil {
		return nil, err
	}

	if admin.ID == req.UserId {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest, "cannot delete your own account",
		)
	}

	if err := s.deps.Auth.DeleteUser(ctx, req.UserId); err != nil {
		if errors.Is(err, commerr.ErrNotFound) {
			return api.DeleteUser404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse(
					envelope(aichteeteapee.ErrorCodeNotFound, "user not found"),
				),
			}, nil
		}

		return nil, ctxerrors.Wrap(err, "delete user")
	}

	return api.DeleteUser204Response{}, nil
}

func createUserBadRequest(message string) api.CreateUser400JSONResponse {
	return api.CreateUser400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}
