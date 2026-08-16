package httpserver

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/drivers/anthropic"
	"github.com/psyb0t/elelem/drivers/openai"
	"github.com/psyb0t/elelem/elelemtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The whole point of the testing.Testing() switch is that a test can never
// reach a real provider by accident, so these assert the switch itself rather
// than trusting it. Not parallel: they mutate the process-wide registry.
func TestNewUpstreamDriver_UnderTestResolvesToTheDouble(t *testing.T) {
	t.Cleanup(elelemtest.ResetGlobalScriptedDriver)
	elelemtest.ResetGlobalScriptedDriver()

	upstream := config.Upstream{
		Name:     "openai",
		Provider: config.UpstreamProviderOpenAI,
	}

	// Nothing installed: still never a real driver. An empty script means the
	// first Stream fails offline instead of dialling the provider.
	driver, err := newUpstreamDriver(upstream, time.Second, false)
	require.NoError(t, err)
	require.IsType(t, &elelemtest.ScriptedDriver{}, driver)

	_, err = driver.Stream(
		t.Context(),
		elelem.DriverRequest{Model: elelem.Model{ID: "any"}},
		nil,
	)
	require.ErrorIs(t, err, elelemtest.ErrNoScriptedTurns)

	// Installed: the app's wiring hands back the exact driver the test set up.
	installed := elelemtest.NewScriptedDriver(elelemtest.Text("scripted"))
	elelemtest.SetGlobalScriptedDriver(installed)

	driver, err = newUpstreamDriver(upstream, time.Second, false)
	require.NoError(t, err)
	assert.Same(t, installed, driver)
}

func TestNewUpstreamDriver_ForceRealBypassesTheDouble(t *testing.T) {
	t.Cleanup(elelemtest.ResetGlobalScriptedDriver)
	elelemtest.SetGlobalScriptedDriver(
		elelemtest.NewScriptedDriver(elelemtest.Text("scripted")),
	)

	// Force-real must win even with a double installed — otherwise the
	// real-LLM suite would silently test the double.
	driver, err := newUpstreamDriver(config.Upstream{
		Name:     "openai",
		Provider: config.UpstreamProviderOpenAI,
	}, time.Second, true)
	require.NoError(t, err)
	assert.IsType(t, &openai.Driver{}, driver)
}

func TestNewUpstreamDriver_IsolatesEnvironmentCredentials(t *testing.T) {
	testCases := []struct {
		name              string
		provider          config.UpstreamProvider
		credentialEnvName string
		credentialHeaders []string
		response          string
		requestPath       string
	}{
		{
			name:              "openai",
			provider:          config.UpstreamProviderOpenAI,
			credentialEnvName: "OPENAI_API_KEY",
			credentialHeaders: []string{"Authorization", "X-Environment-Secret"},
			response:          `{"data":[]}`,
			requestPath:       "/models",
		},
		{
			name:              "anthropic",
			provider:          config.UpstreamProviderAnthropic,
			credentialEnvName: "ANTHROPIC_API_KEY",
			credentialHeaders: []string{"Authorization", "X-Api-Key"},
			response:          `{"data":[],"has_more":false}`,
			requestPath:       "/v1/models",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(testCase.credentialEnvName, "must-not-reach-keyless-upstream")
			if testCase.provider == config.UpstreamProviderOpenAI {
				t.Setenv(
					"OPENAI_CUSTOM_HEADERS",
					"X-Environment-Secret: must-not-reach-keyless-upstream",
				)
			}

			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				assert.Equal(t, testCase.requestPath, request.URL.Path)
				for _, headerName := range testCase.credentialHeaders {
					assert.Empty(t, request.Header.Get(headerName), headerName)
				}

				writer.Header().Set(
					aichteeteapee.HeaderNameContentType,
					aichteeteapee.ContentTypeJSON,
				)
				_, err := writer.Write([]byte(testCase.response))
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			driver, err := newUpstreamDriver(config.Upstream{
				Name:     testCase.name,
				Provider: testCase.provider,
				BaseURL:  server.URL,
			}, time.Second, true)
			require.NoError(t, err)

			_, err = driver.ListModels(t.Context())
			require.NoError(t, err)
		})
	}
}

// freeAddr returns a currently-free loopback address to bind an aux listener.
func freeAddr(t *testing.T) string {
	t.Helper()

	lc := net.ListenConfig{}

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return addr
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
}

// tryGet does a context-bound GET, returning the status (0 on transport error).
func tryGet(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode, nil
}

// TestServeAux_EmptyAddrDisabled: an empty address disables the listener — the
// returned stop func is a safe no-op and nothing is bound.
func TestServeAux_EmptyAddrDisabled(t *testing.T) {
	t.Parallel()

	stop := serveAux(t.Context(), "metrics", "", okHandler())
	require.NotNil(t, stop)

	assert.NotPanics(t, stop)
}

// TestServeAux_ServesAndShuts: a configured address serves the handler, and the
// returned stop func shuts the listener down (subsequent dials fail).
func TestServeAux_ServesAndShuts(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)
	url := "http://" + addr + "/x"

	stop := serveAux(t.Context(), "metrics", addr, okHandler())

	// The listener binds asynchronously — poll until it answers 200.
	require.Eventually(t, func() bool {
		code, err := tryGet(t.Context(), url)

		return err == nil && code == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)

	stop()

	// After shutdown the port stops answering.
	require.Eventually(t, func() bool {
		_, err := tryGet(t.Context(), url)

		return err != nil
	}, 5*time.Second, 20*time.Millisecond)
}

func TestNewUpstreamDriver_SelectsProvider(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		provider config.UpstreamProvider
		assert   func(*testing.T, any)
	}{
		{
			name:     "openai",
			provider: config.UpstreamProviderOpenAI,
			assert: func(t *testing.T, driver any) {
				t.Helper()
				assert.IsType(t, &openai.Driver{}, driver)
			},
		},
		{
			name:     "anthropic",
			provider: config.UpstreamProviderAnthropic,
			assert: func(t *testing.T, driver any) {
				t.Helper()
				assert.IsType(t, &anthropic.Driver{}, driver)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			driver, err := newUpstreamDriver(config.Upstream{
				Name:     testCase.name,
				Provider: testCase.provider,
			}, time.Second, true)
			require.NoError(t, err)
			testCase.assert(t, driver)
		})
	}
}

func TestNewUpstreamDriver_RejectsUnsupportedProvider(t *testing.T) {
	t.Parallel()

	_, err := newUpstreamDriver(config.Upstream{
		Name:     "invalid",
		Provider: "invalid",
	}, time.Second, true)
	require.ErrorIs(t, err, config.ErrInvalidUpstream)
}

func TestNewUpstreamHTTPClient_UsesConnectionDeadlineOnly(t *testing.T) {
	t.Parallel()

	connectTimeout := 7 * time.Second
	client, err := newUpstreamHTTPClient(connectTimeout)
	require.NoError(t, err)

	require.Zero(t, client.Timeout)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, transport.DialContext)
	assert.Equal(t, connectTimeout, transport.TLSHandshakeTimeout)

	dialer := newUpstreamDialer(connectTimeout)
	assert.Equal(t, connectTimeout, dialer.Timeout)
	assert.Equal(t, upstreamHTTPKeepAlive, dialer.KeepAlive)
}
