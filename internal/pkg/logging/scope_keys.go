package logging

import (
	"context"

	"github.com/psyb0t/ctxscope"
)

// The identity attributes chatz puts on the request scope. They are set once at
// the HTTP boundary and every log line under that context carries them, so they
// live here rather than in the HTTP package: the usage recorder reads the
// request id to attribute token spend, and it must not import the HTTP layer to
// do it.
//
// These belong on the scope tier, not the global one — they describe the WORK,
// and a process-wide value would survive into the next request.
const (
	ScopeKeyRequestID = "request_id"
	ScopeKeyUserID    = "user_id"
	ScopeKeyIsAdmin   = "is_admin"
)

// The facts that describe the BINARY rather than any one request. Set once at
// startup on the global tier, which ToJSON never serializes — sending a service
// name across a hop would overwrite the receiving service's own.
const (
	ScopeKeyService = "service"
	ScopeKeyVersion = "version"
	ScopeKeyCommit  = "commit"
)

// RequestIDFromScope returns the correlation id for the current request, or ""
// when the caller runs outside one (a background job, a test).
//
// It exists so the key is written down once: a caller that hand-typed
// "request_id" would keep compiling after a rename and silently read nothing.
func RequestIDFromScope(ctx context.Context) string {
	requestID, _ := ctxscope.Get(ctx)[ScopeKeyRequestID].(string)

	return requestID
}
