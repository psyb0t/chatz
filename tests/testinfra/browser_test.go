//go:build api

package testinfra

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// probeBootTimeout is short enough that the browser cannot possibly report
// healthy, which is the point: it forces SetupBrowser down its start-failure
// path deterministically instead of waiting for real resource pressure.
const probeBootTimeout = time.Second

// A browser that is created but never becomes ready MUST still be terminated.
//
// testcontainers' GenericContainer returns a NON-NIL container alongside its
// error in exactly that case — upstream documents it as "give the caller an
// opportunity to call Destroy". Dropping it leaks a container that stays
// attached to the per-test network, so the stack's Teardown then fails with
// "network ... has active endpoints", and every later run starts with less
// headroom and leaks more. Two full api suites died to that before it was
// found, and it looked like flake because the failing assertion was in
// teardown, nowhere near the cause.
//
// This reproduces it in about a second. Reverting SetupBrowser's error path to
// a bare wrap makes this test fail with the same "active endpoints" error the
// suite was dying on.
func TestSetupBrowser_TerminatesContainerThatNeverBecomesReady(t *testing.T) {
	ctx := context.Background()

	const probeNetwork = "chatz-browser-leak-regression"

	// network.New (the non-deprecated spelling) is not vendored and cannot be
	// fetched offline; GenericNetwork is the vendored equivalent.
	req := testcontainers.GenericNetworkRequest{ //nolint:staticcheck,lll // see above
		NetworkRequest: testcontainers.NetworkRequest{ //nolint:staticcheck,lll // see above
			Name:       probeNetwork,
			Attachable: true,
		},
	}

	net, err := testcontainers.GenericNetwork(ctx, req) //nolint:staticcheck,lll // see above
	require.NoError(t, err, "create regression network")

	// Best-effort: the assertion below removes it on the happy path, so this
	// only fires when the test failed and something is still attached.
	t.Cleanup(func() {
		_ = net.Remove(ctx)
	})

	browser, err := SetupBrowser(
		ctx,
		probeNetwork,
		"http://127.0.0.1:1",
		BrowserOptions{BootTimeout: probeBootTimeout},
	)
	require.Error(t, err, "browser cannot be ready within the probe timeout")
	assert.Nil(t, browser, "no browser handle on the failure path")

	// The real assertion. Nothing is attached to the network only if the failed
	// container was terminated, so a successful Remove IS the no-leak proof.
	assert.NoError(
		t, net.Remove(ctx),
		"network must be removable — a leaked browser container pins it",
	)
}
