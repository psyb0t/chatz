// Package server implements chatz's generated StrictServerInterface on echo:
// session-cookie auth, model discovery, MCP management, and streaming chat.
// Handlers are split per resource across handler_*.go; this file owns the echo
// bootstrap, middleware, and the request-scoped identity helpers.
package server

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	"github.com/psyb0t/chatz/internal/pkg/core/chats"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	chatzlogging "github.com/psyb0t/chatz/internal/pkg/logging"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/chatz/internal/pkg/operations"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
)

const (
	shutdownTimeout      = 10 * time.Second
	sessionCookieMaxAge  = 30 * 24 * time.Hour
	sessionCookieMaxSecs = int(sessionCookieMaxAge / time.Second)

	// maxRequestBody caps request bodies (JSON only — no uploads) so a huge
	// .mcp.json / message can't exhaust memory.
	maxRequestBody = "1M"

	// apiGroupPrefix mounts the generated API onto the wire. This is where the
	// /api/v1 URL comes from at runtime — the spec (api/api.yml) only knows
	// about /v1 via its own servers.url; /api is purely a server-code decision.
	apiGroupPrefix = "/api/v1"

	// healthzPath is the liveness probe, kept bare/unversioned at root per
	// convention (health/diagnostic endpoints never carry a version prefix).
	healthzPath = "/healthz"

	// The request-scoped identity attribute names live in internal/pkg/logging
	// (ScopeKeyUserID / ScopeKeyIsAdmin / ScopeKeyRequestID) because the usage
	// recorder reads the request id back and must not import this package. The
	// request / response HEADER carrying the id is
	// aichteeteapee.HeaderNameXRequestID.
	//
	// maxRequestIDLen bounds an incoming X-Request-Id before it's even
	// regex-matched — a malicious caller can't make us run a regex against an
	// unbounded string.
	maxRequestIDLen = 64
)

// requestIDShapeRE accepts a UUID (any version, incl. the v4 we mint) or a
// ULID. Anything else (newline injection, absurd length, garbage) is
// rejected and a fresh UUID is minted instead — an inbound header is
// attacker-controlled and must never be trusted blindly into a log field.
var requestIDShapeRE = regexp.MustCompile(
	`^(?:[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-` +
		`[0-9a-fA-F]{12}|[0-9A-HJKMNP-TV-Z]{26})$`,
)

// Deps are the collaborators the handlers drive.
type Deps struct {
	Auth       *auth.Service
	Chats      *chats.Service
	MCP        *mcp.Manager
	MCPServers *mcp.ServerStore
	Models     *upstreams.Registry
	Readiness  *operations.Service
	Secrets    *secrets.Box

	// WebFS, when non-nil, holds the built SPA (index.html + _app/...). Its
	// presence makes New register catch-all SPA serving AFTER the API routes so
	// one service serves API + UI. Nil disables SPA serving entirely (callers /
	// tests that don't set it still compile + run — API-only).
	WebFS fs.FS
}

// Server implements api.StrictServerInterface.
type Server struct {
	deps Deps
	echo *echo.Echo
}

// New wires the echo server: request logging, session auth, and the generated
// strict handlers.
func New(deps Deps) *Server {
	// Collapse repo/driver not-found errors to commerr.ErrNotFound so the
	// handler layer has one not-found check (see errormap.go).
	registerErrorMappings()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = errorHandler

	s := &Server{deps: deps, echo: e}

	// BodyLimit caps request size; CSRF on state-changing routes is covered by
	// the SameSite=Lax session cookie (a cross-site POST/DELETE sends no cookie
	// -> 401). No separate CSRF token — single trusted admin, same-origin SPA.
	e.Use(echomiddleware.BodyLimit(maxRequestBody))
	// requestID runs first so every subsequent middleware/handler log line
	// during this request — including requestLogger's own completion line —
	// carries request_id.
	e.Use(requestID())
	e.Use(requestLogger())
	e.Use(s.sessionAuth())

	// apiGroupPrefix (/api/v1) is a server-code mount-point decision, separate
	// from the spec's own servers.url (/v1) — the spec's paths are relative to
	// its server URL, not to /api. Echo's root e.Use() middlewares apply to
	// every request regardless of which group matched the route (Echo runs
	// routing first, then wraps the resolved handler with e.middleware at the
	// Echo level, not per-group) — so BodyLimit/requestLogger/sessionAuth above
	// still cover every route registered on apiGroup with no extra Use() call.
	//
	// RegisterHandlers registers every path in the spec (including /healthz)
	// onto whichever single router it's given — it has no per-path filtering.
	// /healthz must stay bare at root (health checks are unversioned per
	// convention), so it's registered directly on e first, and skipRouter
	// drops the GET /healthz registration that RegisterHandlers would
	// otherwise also add under the group (which would make it reachable, and
	// wrongly, at /api/v1/healthz too).
	strictHandler := api.NewStrictHandler(s, nil)
	wrapper := &api.ServerInterfaceWrapper{Handler: strictHandler}
	e.GET(healthzPath, wrapper.GetHealth)

	apiGroup := e.Group(apiGroupPrefix)
	groupRouter := &skipRouter{EchoRouter: apiGroup, skipGET: healthzPath}
	api.RegisterHandlers(groupRouter, strictHandler)

	// SPA serving is registered LAST, as a catch-all, so every explicit API
	// route + /healthz above wins the match. Nil WebFS => API-only.
	if deps.WebFS != nil {
		s.registerSPA()
	}

	return s
}

// skipRouter wraps an api.EchoRouter and drops one GET registration — used to
// keep RegisterHandlers' monolithic, all-paths-in-the-spec registration from
// re-adding /healthz under the /api/v1 group after it's already been
// registered bare, at root.
type skipRouter struct {
	api.EchoRouter
	skipGET string
}

func (r *skipRouter) GET(
	path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc,
) *echo.Route {
	if path == r.skipGET {
		return &echo.Route{Method: http.MethodGet, Path: path}
	}

	return r.EchoRouter.GET(path, h, m...)
}

// Handler returns the underlying http.Handler (for embedding + tests).
func (s *Server) Handler() http.Handler {
	return s.echo
}

// Serve runs the echo server on addr until ctx is cancelled, then drains it.
func (s *Server) Serve(ctx context.Context, addr string) error {
	errCh := make(chan error, 1)

	go func() {
		if err := s.echo.Start(addr); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), shutdownTimeout,
		)
		defer cancel()

		if err := s.echo.Shutdown(shutCtx); err != nil {
			return ctxerrors.Wrap(err, "shutdown echo")
		}

		return nil
	case err := <-errCh:
		return ctxerrors.Wrap(err, "echo serve")
	}
}

// --- request-scoped identity -----------------------------------------------

type userCtxKey struct{}

type tokenCtxKey struct{}

// sessionAuth attaches the authenticated user + raw session token to the
// request context when a valid session cookie is present. It never rejects —
// per-handler checks enforce authz, so public endpoints stay reachable.
func (s *Server) sessionAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(auth.CookieName)
			if err != nil || cookie.Value == "" {
				return next(c)
			}

			ctx := context.WithValue(
				c.Request().Context(), tokenCtxKey{}, cookie.Value,
			)

			user, aerr := s.deps.Auth.Authenticate(ctx, cookie.Value)
			if aerr == nil {
				ctx = context.WithValue(ctx, userCtxKey{}, user)
				ctx = getCtxWithIdentity(ctx, user)
			}

			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

func userFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(userCtxKey{}).(*models.User)

	return user
}

func tokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenCtxKey{}).(string)

	return token
}

// getCtxWithIdentity puts the authenticated user's id + admin flag on the
// request scope, so every downstream log line (handlers + the request-completed
// line) carries who made the request. requestLogger runs outer of the auth
// middleware but calls scope.GetLogger after next(), so it sees this.
//
// These go on the scope rather than on a logger: scope.GetLogger applies them
// at the moment a line is written, and anything that needs the VALUE (not just
// the log field) can read it back — which a logger attribute cannot offer.
func getCtxWithIdentity(
	ctx context.Context,
	user *models.User,
) context.Context {
	return ctxscope.Set(ctx,
		ctxscope.Attr(chatzlogging.ScopeKeyUserID, user.ID.String()),
		ctxscope.Attr(chatzlogging.ScopeKeyIsAdmin, user.IsAdmin),
	)
}

// requireUser returns the authenticated user or an echo 401.
func requireUser(ctx context.Context) (*models.User, error) {
	user := userFromContext(ctx)
	if user == nil {
		return nil, echo.NewHTTPError(
			http.StatusUnauthorized, "authentication required",
		)
	}

	return user, nil
}

// requireAdmin returns the authenticated admin or an echo 401/403.
func requireAdmin(ctx context.Context) (*models.User, error) {
	user, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	if !user.IsAdmin {
		return nil, echo.NewHTTPError(http.StatusForbidden, "admin required")
	}

	return user, nil
}

// errorHandler renders every echo-level error as the aichteeteapee-shaped
// {code, message, details} envelope. Internal (non-HTTPError) failures never
// leak their message to the client — the real error is on the request log.
func errorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	message := "internal server error"

	var he *echo.HTTPError
	if errors.As(err, &he) {
		status = he.Code
		if m, ok := he.Message.(string); ok {
			message = m
		}
	}

	_ = c.JSON(status, envelope(
		aichteeteapee.ErrorCodeFromHTTPStatus(status), message,
	))
}

// sessionCookie renders the Set-Cookie header value for a new session. Returns
// a pointer because the generated Set-Cookie response header is optional (only
// present when a session is actually minted). Secure is omitted so it works
// over plain HTTP in dev; terminate TLS at the proxy in prod and the cookie
// still rides on the encrypted hop.
func sessionCookie(token string) *string {
	c := &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionCookieMaxSecs,
	}

	v := c.String()

	return &v
}

// requestID reads X-Request-Id from the incoming request, falling back to a
// freshly minted UUID v4 when absent or when the value doesn't look like a
// UUID/ULID (never trust an attacker-controlled header blindly into a log
// field). The id goes on the request scope (so every downstream
// scope.GetLogger(ctx) call during this request carries request_id for free)
// and is echoed back on the response.
//
// Scope, not a logger attribute: the llm_usage row needs the id as a VALUE to
// attribute token spend to a request, and an attribute baked into a logger
// cannot be read back out.
func requestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Request().Header.Get(aichteeteapee.HeaderNameXRequestID)
			if !isValidRequestID(id) {
				id = uuid.NewString()
			}

			ctx := ctxscope.Set(
				c.Request().Context(),
				ctxscope.Attr(chatzlogging.ScopeKeyRequestID, id),
			)
			c.SetRequest(c.Request().WithContext(ctx))

			c.Response().Header().Set(aichteeteapee.HeaderNameXRequestID, id)

			return next(c)
		}
	}
}

// isValidRequestID rejects anything that isn't a well-formed UUID or ULID —
// newline injection, absurd lengths, and other garbage never make it onto a
// log line as request_id.
func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}

	return requestIDShapeRE.MatchString(id)
}

// requestLogger emits one line per request via the ctx logger. /healthz
// completions log at DEBUG (the docker healthcheck polls it every ~10s and
// would otherwise drown real traffic at INFO); every other route logs its
// completion at INFO as before.
func requestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			status := c.Response().Status

			if err != nil {
				var he *echo.HTTPError
				if errors.As(err, &he) {
					status = he.Code
				} else {
					status = http.StatusInternalServerError
				}
			}

			fields := []any{
				"method", c.Request().Method,
				"route", c.Path(),
				"status", status,
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if err != nil {
				fields = append(fields, "err", err)
			}

			// Re-fetch AFTER next — requestID + sessionAuth enrich the ctx
			// logger and re-set the request ctx, so only a post-next fetch
			// sees request_id/user_id on the completion line.
			logger := ctxscope.GetLogger(c.Request().Context())

			if c.Path() == healthzPath {
				logger.Debug("request completed", fields...)
			} else {
				logger.Info("request completed", fields...)
			}

			return err
		}
	}
}
