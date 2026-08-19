package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync/atomic"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/ctxerrors"
)

const (
	// mcpClientName + mcpClientVersion identify chatz to MCP servers in the
	// initialize handshake.
	mcpClientName    = "chatz"
	mcpClientVersion = "0.3.0"

	// toolNameSep joins a server name and a tool name into the qualified name
	// the LLM sees, so a call routes back to the owning server. Server names
	// must not contain it (Cut splits on the first occurrence).
	toolNameSep = "__"
)

// Tool is a tool discovered on an MCP server.
type Tool struct {
	Server      string         `json:"server"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// QualifiedName is the server-namespaced name exposed to the LLM.
func (t Tool) QualifiedName() string {
	return t.Server + toolNameSep + t.Name
}

// ToolResult is the outcome of a tool call. IsError is the tool-level error
// flag (the call itself succeeded at the protocol level); Text is the joined
// text content.
type ToolResult struct {
	Text    string
	IsError bool
}

// Client is a live connection to one MCP server.
type Client struct {
	server  string
	session *mcpsdk.ClientSession

	// closing is set before Close tears down the session, so a caller
	// blocked in Wait can tell a deliberate shutdown (Remove, Disable, or
	// being replaced by a newer connect) apart from the session dying
	// unexpectedly on its own (e.g. its transport's background stream
	// exhausting reconnect retries) — only the latter should trigger the
	// Manager's auto-retry recovery. See WasIntentionalClose.
	closing atomic.Bool
}

// Connect dials the server described by srv, decrypting its env/header secrets
// with box, and completes the MCP initialize handshake. ctx bounds the connect;
// the session outlives it (a stdio subprocess is torn down by Close, not ctx).
func Connect(
	ctx context.Context,
	srv *models.MCPServer,
	box *secrets.Box,
) (*Client, error) {
	sdkClient := mcpsdk.NewClient(
		&mcpsdk.Implementation{Name: mcpClientName, Version: mcpClientVersion},
		nil,
	)

	var (
		session *mcpsdk.ClientSession
		err     error
	)

	switch srv.Transport {
	case models.MCPTransportStdio:
		transport, transportErr := stdioTransport(srv, box)
		if transportErr != nil {
			return nil, transportErr
		}

		session, err = sdkClient.Connect(ctx, transport, nil)
	case models.MCPTransportHTTP:
		transport, transportErr := httpTransport(srv, box)
		if transportErr != nil {
			return nil, transportErr
		}

		session, err = sdkClient.Connect(ctx, transport, nil)
	default:
		return nil, ctxerrors.Wrapf(
			ErrUnsupportedTransport,
			"transport %q",
			srv.Transport,
		)
	}

	if err != nil {
		return nil, ctxerrors.Wrapf(err, "connect to mcp server %q", srv.Name)
	}

	return &Client{server: srv.Name, session: session}, nil
}

// ListTools enumerates the server's tools (auto-paginated), namespaced to this
// server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var tools []Tool

	for tool, err := range c.session.Tools(ctx, nil) {
		if err != nil {
			return nil, ctxerrors.Wrap(err, "list mcp tools")
		}

		tools = append(tools, Tool{
			Server:      c.server,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaMap(tool.InputSchema),
		})
	}

	return tools, nil
}

// CallTool invokes tool (the raw server-side name) with args. A protocol
// failure returns an error; a tool-level failure returns a result with
// IsError set.
func (c *Client) CallTool(
	ctx context.Context,
	tool string,
	args map[string]any,
) (*ToolResult, error) {
	res, err := c.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      tool,
		Arguments: args,
	})
	if err != nil {
		return nil, ctxerrors.Wrapf(err, "call mcp tool %q", tool)
	}

	return &ToolResult{
		Text:    textOf(res.Content),
		IsError: res.IsError,
	}, nil
}

// Close disconnects the session (and tears down a stdio subprocess).
func (c *Client) Close() error {
	c.closing.Store(true)

	if err := c.session.Close(); err != nil {
		return ctxerrors.Wrap(err, "close mcp session")
	}

	return nil
}

// Wait blocks until the underlying session closes, deliberately or not, and
// returns the terminal error (nil on a clean close). A caller uses this to
// detect the connection dying in the background outside any specific RPC —
// see WasIntentionalClose to tell that apart from an expected shutdown.
func (c *Client) Wait() error {
	//nolint:wrapcheck // this IS the raw terminal signal; the caller decides
	// what it means (WasIntentionalClose) before wrapping/logging it
	return c.session.Wait()
}

// WasIntentionalClose reports whether Close has been called on this client —
// distinguishes a deliberate shutdown from an unexpected async death for a
// Wait caller.
func (c *Client) WasIntentionalClose() bool {
	return c.closing.Load()
}

func stdioTransport(
	srv *models.MCPServer,
	box *secrets.Box,
) (*mcpsdk.CommandTransport, error) {
	var args []string
	if len(srv.Args) > 0 {
		if err := json.Unmarshal(srv.Args, &args); err != nil {
			return nil, ctxerrors.Wrap(err, "unmarshal stdio args")
		}
	}

	env, err := box.OpenMap(srv.EnvEnc)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "open stdio env")
	}

	// The command is admin-configured (single trusted admin), and its lifetime
	// is bound to the session (Close), NOT the connect ctx — a short connect
	// timeout must not kill a long-lived server.
	cmd := exec.Command(srv.Command, args...) //nolint:gosec,noctx // see above

	cmd.Env = append(os.Environ(), envSlice(env)...)

	return &mcpsdk.CommandTransport{Command: cmd}, nil
}

func httpTransport(
	srv *models.MCPServer,
	box *secrets.Box,
) (*mcpsdk.StreamableClientTransport, error) {
	headers, err := box.OpenMap(srv.HeadersEnc)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "open http headers")
	}

	httpClient := http.DefaultClient
	if len(headers) > 0 {
		httpClient = &http.Client{
			Transport: &headerRoundTripper{
				headers: headers,
				base:    http.DefaultTransport,
			},
		}
	}

	return &mcpsdk.StreamableClientTransport{
		Endpoint:   srv.URL,
		HTTPClient: httpClient,
	}, nil
}

// headerRoundTripper injects static headers (e.g. Authorization) on every
// request — the SDK's StreamableClientTransport has no header field, so custom
// headers ride on a custom http.Client.
type headerRoundTripper struct {
	headers map[string]string
	base    http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h.headers {
		r.Header.Set(k, v)
	}

	//nolint:wrapcheck // transparent RoundTripper passthrough
	return h.base.RoundTrip(r)
}

func schemaMap(schema any) map[string]any {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}

	return m
}

func textOf(content []mcpsdk.Content) string {
	var parts []string

	for _, c := range content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}

	return strings.Join(parts, "\n")
}

func envSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	out := make([]string, 0, len(env))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}

	return out
}
