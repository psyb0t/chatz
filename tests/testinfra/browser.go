//go:build api

package testinfra

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/ctxerrors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The browser is a psyb0t/stealthy-auto-browse container in server mode: it
// exposes a JSON HTTP API where each POST is one browser action. It joins the
// api network so it reaches the app same-origin by alias, and its API port is
// mapped to the host so the Go test process drives it.
const (
	browserImage = "psyb0t/stealthy-auto-browse"
	browserPort  = "8080"

	// ~20s on an idle host, but this starts Xvfb, Chrome and a Python API
	// server, and api runs several full stacks at once on a shared box. Under
	// load it has been measured past two minutes, and overshooting surfaces as
	// "wait until ready: context deadline exceeded" — which reads like a broken
	// image rather than a slow one. Generous costs nothing on a fast host: the
	// wait ends the moment /health answers.
	browserBootTimeout   = 5 * time.Minute
	browserActionTimeout = 150 * time.Second

	screenshotDirPerm  = 0o750
	screenshotFilePerm = 0o600

	// APIUser / APIPass are the admin credentials the setupAdmin flow creates
	// on first run (/setup), then logs in with.
	APIUser = "admin"
	APIPass = "hunter2hunter2"
)

// Browser owns the running browser container plus the client that drives it.
type Browser struct {
	container testcontainers.Container
	Client    *BrowserClient
}

// BrowserOptions tunes the browser container for a driver.
type BrowserOptions struct {
	// Viewport, when set (e.g. "414x896"), forces a fixed window size so the
	// app's mobile media queries engage. Empty = the default desktop viewport.
	Viewport string

	// BootTimeout overrides how long to wait for the browser's /health to come
	// up. Zero means browserBootTimeout. The leak regression test sets this
	// very low to force the failure path deterministically, which is the only
	// way to assert that a browser that never becomes ready is still torn down.
	BootTimeout time.Duration
}

// SetupBrowser starts the browser on the given network (so it can reach the app
// by alias) and returns a client bound to its host-mapped API port.
func SetupBrowser(
	ctx context.Context,
	networkName, appURL string,
	opts BrowserOptions,
) (*Browser, error) {
	env := map[string]string{
		"APP_URL":  appURL,
		"API_USER": APIUser,
		"API_PASS": APIPass,
	}
	if opts.Viewport != "" {
		env["USE_VIEWPORT"] = "true"
		env["XVFB_RESOLUTION"] = opts.Viewport
	}

	bootTimeout := opts.BootTimeout
	if bootTimeout == 0 {
		bootTimeout = browserBootTimeout
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        browserImage,
				Networks:     []string{networkName},
				ExposedPorts: []string{browserPort + "/tcp"},
				Env:          env,
				WaitingFor: wait.ForHTTP("/health").
					WithPort(browserPort + "/tcp").
					WithStartupTimeout(bootTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		// GenericContainer returns a NON-NIL container with this error when
		// the container was created but never became ready — upstream calls
		// it "an opportunity to call Destroy". Returning without terminating
		// leaked it, and since it stays attached to the per-test network,
		// Teardown then failed with "network has active endpoints".
		return nil, abandonBrowser(ctx, container, err, "start browser")
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, abandonBrowser(ctx, container, err, "browser host")
	}

	port, err := container.MappedPort(ctx, browserPort+"/tcp")
	if err != nil {
		return nil, abandonBrowser(ctx, container, err, "browser mapped port")
	}

	base := "http://" + net.JoinHostPort(host, port.Port())

	return &Browser{
		container: container,
		Client: &BrowserClient{
			base: base,
			http: &http.Client{Timeout: browserActionTimeout},
		},
	}, nil
}

// abandonBrowser wraps the failure that made setup give up, terminating the
// container first so a half-started browser is not left running. A teardown
// failure is joined rather than replacing the original — the reason setup
// failed is the one worth reading.
func abandonBrowser(
	ctx context.Context,
	container testcontainers.Container,
	err error,
	what string,
) error {
	// The bring-up path can hand this a nil container: GenericContainer returns
	// (nil, err) when it fails before creating anything. There is nothing to
	// terminate then, and dereferencing it would turn a failed start into a
	// panic.
	if container == nil {
		return ctxerrors.Wrap(err, what)
	}

	if terminateErr := container.Terminate(ctx); terminateErr != nil {
		return errors.Join(
			ctxerrors.Wrap(err, what),
			ctxerrors.Wrapf(terminateErr, "terminate browser after %s", what),
		)
	}

	return ctxerrors.Wrap(err, what)
}

// Close terminates the browser container.
func (b *Browser) Close(ctx context.Context) error {
	if b.container == nil {
		return nil
	}

	if err := b.container.Terminate(ctx); err != nil {
		return ctxerrors.Wrap(err, "terminate browser")
	}

	return nil
}

// BrowserClient POSTs one action at a time to the browser API and asserts
// success, so a failing step surfaces immediately at the exact action.
type BrowserClient struct {
	base string
	http *http.Client
}

type actEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// Act POSTs {"action": action, ...params} and returns the action's data.
func (c *BrowserClient) Act(
	ctx context.Context,
	action string,
	params map[string]any,
) (json.RawMessage, error) {
	payload := map[string]any{"action": action}
	for key, value := range params {
		payload[key] = value
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "marshal action")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.base+"/",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build action request")
	}

	req.Header.Set(
		aichteeteapee.HeaderNameContentType,
		aichteeteapee.ContentTypeJSON,
	)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, ctxerrors.Wrapf(err, "post action %s", action)
	}

	defer func() { _ = resp.Body.Close() }()

	var env actEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, ctxerrors.Wrapf(err, "decode action %s response", action)
	}

	if !env.Success {
		return nil, ctxerrors.New(
			fmt.Sprintf("action %s failed: %s", action, env.Error),
		)
	}

	return env.Data, nil
}

// EvalJSON runs a JS expression that returns a JSON string (via
// JSON.stringify) and unmarshals it into out. The browser wraps the return
// value as {"result": "<json-string>"}, so this decodes the wrapper then the
// inner JSON.
func (c *BrowserClient) EvalJSON(
	ctx context.Context,
	expression string,
	out any,
) error {
	data, err := c.Act(ctx, "eval", map[string]any{"expression": expression})
	if err != nil {
		return err
	}

	var wrap struct {
		Result *string `json:"result"`
	}

	if err := json.Unmarshal(data, &wrap); err == nil && wrap.Result != nil {
		if err := json.Unmarshal([]byte(*wrap.Result), out); err != nil {
			return ctxerrors.Wrap(err, "decode eval result")
		}

		return nil
	}

	if err := json.Unmarshal(data, out); err != nil {
		return ctxerrors.Wrap(err, "decode eval data")
	}

	return nil
}

// Goto navigates to url and waits for the given load state (e.g. networkidle).
func (c *BrowserClient) Goto(
	ctx context.Context,
	url, waitUntil string,
) error {
	_, err := c.Act(ctx, "goto", map[string]any{
		"url":        url,
		"wait_until": waitUntil,
	})

	return err
}

// Fill types value into the element matched by selector.
func (c *BrowserClient) Fill(
	ctx context.Context,
	selector, value string,
) error {
	_, err := c.Act(ctx, "fill", map[string]any{
		"selector": selector,
		"value":    value,
	})

	return err
}

// Eval runs a JS expression for its side effect (e.g. requestSubmit, click).
func (c *BrowserClient) Eval(ctx context.Context, expression string) error {
	_, err := c.Act(ctx, "eval", map[string]any{"expression": expression})

	return err
}

// WaitForElement blocks until selector reaches state (visible / hidden) or the
// timeout (seconds) elapses.
func (c *BrowserClient) WaitForElement(
	ctx context.Context,
	selector, state string,
	timeoutSeconds int,
) error {
	_, err := c.Act(ctx, "wait_for_element", map[string]any{
		"selector": selector,
		"state":    state,
		"timeout":  timeoutSeconds,
	})

	return err
}

// EnableConsoleLog starts capturing the page's console lines.
func (c *BrowserClient) EnableConsoleLog(ctx context.Context) error {
	_, err := c.Act(ctx, "enable_console_log", nil)

	return err
}

// EnableNetworkLog starts capturing the page's network activity.
func (c *BrowserClient) EnableNetworkLog(ctx context.Context) error {
	_, err := c.Act(ctx, "enable_network_log", nil)

	return err
}

// ConsoleEntry is one captured console line.
type ConsoleEntry struct {
	Text string `json:"text"`
}

// ConsoleLog returns the console lines captured since EnableConsoleLog. The
// browser has returned the list under a few keys over versions, so all three
// are tried (matching the Python driver's tolerance).
func (c *BrowserClient) ConsoleLog(
	ctx context.Context,
) ([]ConsoleEntry, error) {
	data, err := c.Act(ctx, "get_console_log", nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Log     []ConsoleEntry `json:"log"`
		Logs    []ConsoleEntry `json:"logs"`
		Entries []ConsoleEntry `json:"entries"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, ctxerrors.Wrap(err, "decode console log")
	}

	switch {
	case len(payload.Log) > 0:
		return payload.Log, nil
	case len(payload.Logs) > 0:
		return payload.Logs, nil
	default:
		return payload.Entries, nil
	}
}

// Screenshot saves a full-page PNG to path. Best-effort like the Python driver:
// a screenshot is a diagnostic nicety, never a reason to fail a run, so callers
// typically ignore the error.
func (c *BrowserClient) Screenshot(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.base+"/screenshot/browser",
		nil,
	)
	if err != nil {
		return ctxerrors.Wrap(err, "build screenshot request")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ctxerrors.Wrap(err, "get screenshot")
	}

	defer func() { _ = resp.Body.Close() }()

	png, err := io.ReadAll(resp.Body)
	if err != nil {
		return ctxerrors.Wrap(err, "read screenshot")
	}

	if err := os.MkdirAll(filepath.Dir(path), screenshotDirPerm); err != nil {
		return ctxerrors.Wrap(err, "mkdir screenshot dir")
	}

	if err := os.WriteFile(path, png, screenshotFilePerm); err != nil {
		return ctxerrors.Wrap(err, "write screenshot")
	}

	return nil
}
