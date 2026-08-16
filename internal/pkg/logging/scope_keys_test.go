package logging

import (
	"context"
	"testing"

	"github.com/psyb0t/ctxscope"
	"github.com/stretchr/testify/assert"
)

// The llm_usage.request_id column sat empty for its whole existence because the
// id lived only as a logger attribute, which slog cannot read back. It is a
// scope value now, and this is the read the usage recorder performs — so if the
// key ever drifts from what the HTTP middleware sets, the column silently goes
// empty again rather than failing to compile.
func TestRequestIDFromScope(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		ctx  func() context.Context
		want string
	}{
		{
			name: "returns the id the middleware set",
			ctx: func() context.Context {
				return ctxscope.Set(
					context.Background(),
					ctxscope.Attr(ScopeKeyRequestID, "req-abc"),
				)
			},
			want: "req-abc",
		},
		{
			// A background job or a test has no request. Empty is the honest
			// answer; a placeholder would look like a real correlation id.
			name: "empty outside a request",
			ctx:  context.Background,
			want: "",
		},
		{
			// Other scope attributes must not be mistaken for the request id.
			name: "empty when only other attributes are set",
			ctx: func() context.Context {
				return ctxscope.Set(
					context.Background(),
					ctxscope.Attr(ScopeKeyUserID, "user-1"),
				)
			},
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, RequestIDFromScope(tc.ctx()))
		})
	}
}

// The two tiers must stay apart: SetGlobal describes the binary and never
// travels, Set describes the work and does. Putting a service name on the
// travelling tier would cross a hop and rename the receiving service's logs.
func TestScopeTiers_GlobalIsNotSerialized(t *testing.T) {
	ctxscope.SetGlobal(ctxscope.Attr(ScopeKeyService, "chatz"))
	t.Cleanup(func() { ctxscope.RemoveGlobal(ScopeKeyService) })

	ctx := ctxscope.Set(
		context.Background(),
		ctxscope.Attr(ScopeKeyRequestID, "req-abc"),
	)

	assert.Equal(t, "chatz", ctxscope.GetGlobal()[ScopeKeyService],
		"service belongs to the process tier")
	assert.NotContains(t, ctxscope.Get(ctx), ScopeKeyService,
		"the process tier must not leak into the travelling one")

	data, err := ctxscope.ToJSON(ctx)
	assert.NoError(t, err)
	assert.NotContains(t, string(data), ScopeKeyService,
		"a hop must not carry this binary's identity to the next service")
	assert.Contains(t, string(data), "req-abc",
		"the request id is exactly what SHOULD cross a hop")
}
