//go:build integration

package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	_ "github.com/psyb0t/slogging/slogconf"
)

//nolint:gochecknoglobals // shared across the package's integration tests
var testInfra *testinfra.Infra

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error

	testInfra, err = testinfra.Setup(ctx)
	if err != nil {
		slog.Error("test infra setup failed", "err", err)
		os.Exit(1)
	}

	code := m.Run()

	testInfra.Teardown(ctx)
	os.Exit(code)
}
