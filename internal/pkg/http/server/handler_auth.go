package server

import (
	"context"
	"errors"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/ctxerrors"
)

// GetAuthStatus reports whether setup is needed and whether the caller is
// authenticated. Public.
//
// When no valid session is present, it attempts single-user passwordless
// auto-login: if CHATZ_AUTH_PASSWORDLESS is enabled AND the install has
// exactly one user, it mints a session for that sole admin and sets the
// session cookie. The guard lives entirely server-side in PasswordlessLogin
// (passwordless && userCount == 1) — a second user disables it and login is
// required for everyone.
func (s *Server) GetAuthStatus(
	ctx context.Context,
	_ api.GetAuthStatusRequestObject,
) (api.GetAuthStatusResponseObject, error) {
	needsSetup, err := s.deps.Auth.NeedsSetup(ctx)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "needs setup")
	}

	status := api.AuthStatus{NeedsSetup: needsSetup}

	if user := userFromContext(ctx); user != nil {
		status.Authenticated = true
		apiUser := auth.UserToAPI(user)
		status.User = &apiUser

		return api.GetAuthStatus200JSONResponse{Body: status}, nil
	}

	return s.passwordlessStatus(ctx, status)
}

// passwordlessStatus attempts single-user passwordless auto-login for an
// unauthenticated caller and, on success, returns the status carrying the sole
// admin and a fresh session cookie. When passwordless is unavailable (disabled
// or a multi-user install) it returns the anonymous status unchanged.
func (s *Server) passwordlessStatus(
	ctx context.Context,
	status api.AuthStatus,
) (api.GetAuthStatusResponseObject, error) {
	token, user, err := s.deps.Auth.PasswordlessLogin(ctx)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordlessUnavailable) {
			return api.GetAuthStatus200JSONResponse{Body: status}, nil
		}

		return nil, ctxerrors.Wrap(err, "passwordless login")
	}

	status.Authenticated = true
	apiUser := auth.UserToAPI(user)
	status.User = &apiUser

	return api.GetAuthStatus200JSONResponse{
		Body: status,
		Headers: api.GetAuthStatus200ResponseHeaders{
			SetCookie: sessionCookie(token),
		},
	}, nil
}

// Setup bootstraps the first admin user and logs them in. Public, one-shot.
func (s *Server) Setup(
	ctx context.Context,
	req api.SetupRequestObject,
) (api.SetupResponseObject, error) {
	if req.Body == nil {
		return setupBadRequest("missing body"), nil
	}

	user, err := s.deps.Auth.Bootstrap(
		ctx, req.Body.Username, req.Body.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrSetupClosed):
			return api.Setup409JSONResponse{
				ConflictJSONResponse: api.ConflictJSONResponse(
					envelope(
						aichteeteapee.ErrorCodeConflict,
						"setup already completed",
					),
				),
			}, nil
		case errors.Is(err, auth.ErrInvalidInput):
			return setupBadRequest("username and password are required"), nil
		default:
			return nil, ctxerrors.Wrap(err, "bootstrap")
		}
	}

	token, _, err := s.deps.Auth.Login(
		ctx, req.Body.Username, req.Body.Password,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "login after setup")
	}

	return api.Setup200JSONResponse{
		Body:    auth.UserToAPI(user),
		Headers: api.Setup200ResponseHeaders{SetCookie: sessionCookie(token)},
	}, nil
}

func setupBadRequest(message string) api.Setup400JSONResponse {
	return api.Setup400JSONResponse{
		BadRequestJSONResponse: api.BadRequestJSONResponse(
			envelope(aichteeteapee.ErrorCodeBadRequest, message),
		),
	}
}

// Login authenticates a user and issues a session cookie. Public.
func (s *Server) Login(
	ctx context.Context,
	req api.LoginRequestObject,
) (api.LoginResponseObject, error) {
	if req.Body == nil {
		return loginUnauthorized(), nil
	}

	token, user, err := s.deps.Auth.Login(
		ctx, req.Body.Username, req.Body.Password,
	)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return loginUnauthorized(), nil
		}

		return nil, ctxerrors.Wrap(err, "login")
	}

	return api.Login200JSONResponse{
		Body:    auth.UserToAPI(user),
		Headers: api.Login200ResponseHeaders{SetCookie: sessionCookie(token)},
	}, nil
}

func loginUnauthorized() api.Login401JSONResponse {
	return api.Login401JSONResponse{
		UnauthorizedJSONResponse: api.UnauthorizedJSONResponse(
			envelope(
				aichteeteapee.ErrorCodeUnauthorized,
				"invalid credentials",
			),
		),
	}
}

// Logout revokes the current session. Idempotent.
func (s *Server) Logout(
	ctx context.Context,
	_ api.LogoutRequestObject,
) (api.LogoutResponseObject, error) {
	if token := tokenFromContext(ctx); token != "" {
		if err := s.deps.Auth.Logout(ctx, token); err != nil {
			return nil, ctxerrors.Wrap(err, "logout")
		}
	}

	return api.Logout204Response{}, nil
}
