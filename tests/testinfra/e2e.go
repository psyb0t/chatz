//go:build e2e

package testinfra

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The e2e stack is orchestrated from Go via testcontainers: one bridge network
// carrying Postgres, the fake OpenAI upstream (so model discovery returns real
// ids in the browser), the optional MCP fixture, and the app built from the
// prod Dockerfile (SPA + API on one
// origin). The browser (browser.go) joins the same network and reaches the app
// same-origin by alias, exactly like the compose stack.
const (
	e2ePostgresAlias  = "postgres"
	e2eUpstreamAlias  = "fakeupstream"
	e2eMCPServerAlias = "mcpserver"
	e2eAppAlias       = "backend"

	e2eAppPort       = "8080"
	e2eUpstreamPort  = "8000"
	e2eMCPServerPort = "8000"

	e2eDBName = "chatz"
	e2eDBUser = "chatz"
	e2eDBPass = "chatz"

	// Real-upstream config for real_chat: the app reuses the runner's
	// CHATZ_UPSTREAMS plus every configured provider key from the same .env as
	// make test-real.
	e2eUpstreamsEnv      = "CHATZ_UPSTREAMS"
	e2eRealModelEnv      = "CHATZ_E2E_REAL_MODEL"
	e2eDefaultRealModel  = "groq-gpt-oss-120b"
	e2eDatabaseDriverEnv = "CHATZ_E2E_DB_DRIVER"
	// fakeUpstreamFailFirstStreamEnv makes the fake upstream exhaust the retry
	// stack for its first streamed turn. Test-fixture control only.
	fakeUpstreamFailFirstStreamEnv = "CHATZ_E2E_FAIL_FIRST_STREAM"

	// The app image is a multi-stage prod build (SPA + Go binary); the first
	// run pays the full build, later runs hit the layer cache (KeepImage).
	e2eImageBuildTimeout = 8 * time.Minute
	e2eBootTimeout       = 3 * time.Minute
	e2eImageTag          = "current"

	e2eFakeUpstreamImageRepository = "chatz-e2e-fakeupstream"
	e2eAppImageRepository          = "chatz-e2e-app"
	e2eFakeUpstreamImage           = "chatz-e2e-fakeupstream:current"
	e2eAppImage                    = "chatz-e2e-app:current"

	e2eMCPUnauthorizedGet = 406
)

var (
	//nolint:gochecknoglobals // Coordinates parallel test stacks.
	e2eImagesOnce sync.Once
	errE2EImages  error
)

type e2eDatabaseDriver string

const (
	e2eDatabaseDriverPostgres e2eDatabaseDriver = "postgres"
	e2eDatabaseDriverSQLite   e2eDatabaseDriver = "sqlite"
)

// E2EOptions tunes the stack for a specific driver.
type E2EOptions struct {
	// ShowcaseMode sets CHATZ_SHOWCASE_MODE on the app so the exact showcase
	// prompts stream a canned thinking + tool-card + dashboard turn — the
	// render surface the removed /demo commands used to provide.
	ShowcaseMode bool
	// WithMCPServer brings up the FastMCP fixture (alias "mcpserver") for the
	// per-chat + admin MCP drivers.
	WithMCPServer bool
	// RealUpstream points the app at the runner's CHATZ_UPSTREAMS + api key
	// instead of the fake upstream, for the real_chat driver. Requires a key
	// (see RealUpstreamConfigured); the fake upstream is skipped.
	RealUpstream bool
	// FailFirstUpstreamStream asks the fake upstream to exhaust its retry stack
	// for the first streamed turn, for browser recovery coverage.
	FailFirstUpstreamStream bool
}

// e2eNetwork is the slice of the (deprecated) testcontainers network type we
// use — declaring it locally keeps the deprecated symbol out of the struct.
type e2eNetwork interface {
	Remove(context.Context) error
}

// E2EStack holds the running e2e containers + the in-network URL the browser
// uses to reach the app. Call Teardown when done (from TestMain wrapping
// m.Run). Fields are exported so a test can inspect a container if needed.
type E2EStack struct {
	Network   e2eNetwork
	Postgres  *tcpostgres.PostgresContainer
	Upstream  testcontainers.Container
	MCPServer testcontainers.Container
	App       testcontainers.Container

	// AppURL is how the browser (same network) reaches the app: same-origin
	// API + SPA, e.g. http://backend:8080.
	AppURL string
	// MCPServerURL is the in-network streamable-HTTP endpoint an MCP driver
	// registers with the app, e.g. http://mcpserver:8000/mcp.
	MCPServerURL string

	networkName string
}

// NetworkName is the docker network the browser must join to reach the app.
func (s *E2EStack) NetworkName() string {
	return s.networkName
}

// RealUpstreamConfigured reports whether the runner supplied an upstream
// configuration. Setup validates and forwards every referenced provider key.
func RealUpstreamConfigured() bool {
	return os.Getenv(e2eUpstreamsEnv) != ""
}

// RealModel is the model id the real_chat driver selects, overridable via
// CHATZ_E2E_REAL_MODEL so the test tracks whatever provider .env points at.
func RealModel() string {
	if model := os.Getenv(e2eRealModelEnv); model != "" {
		return model
	}

	return e2eDefaultRealModel
}

// SetupE2E brings up network -> selected database -> upstream -> mcpserver ->
// app, each waited on before the next. Every error path tears down what was
// already started so no container or network leaks.
func SetupE2E(ctx context.Context, opts E2EOptions) (*E2EStack, error) {
	databaseDriver, err := e2eDatabaseDriverFromEnv()
	if err != nil {
		return nil, err
	}

	root, err := repoRoot()
	if err != nil {
		return nil, err
	}

	if err := ensureE2EImages(ctx, root); err != nil {
		return nil, err
	}

	mcpURL := "http://" +
		net.JoinHostPort(e2eMCPServerAlias, e2eMCPServerPort) + "/mcp"
	stack := &E2EStack{
		networkName:  fmt.Sprintf("chatz-e2e-%d", time.Now().UnixNano()),
		AppURL:       "http://" + net.JoinHostPort(e2eAppAlias, e2eAppPort),
		MCPServerURL: mcpURL,
	}

	steps := []func(context.Context, string) error{stack.setupNetwork}
	if databaseDriver == e2eDatabaseDriverPostgres {
		steps = append(steps, stack.setupPostgresOnNetwork)
	}

	if !opts.RealUpstream {
		steps = append(steps, func(ctx context.Context, root string) error {
			return stack.setupUpstream(ctx, root, opts)
		})
	}

	if opts.WithMCPServer {
		steps = append(steps, stack.setupMCPServer)
	}

	steps = append(steps, func(ctx context.Context, root string) error {
		return stack.setupApp(ctx, root, opts, databaseDriver)
	})

	for _, step := range steps {
		if err := step(ctx, root); err != nil {
			if teardownErr := stack.Teardown(ctx); teardownErr != nil {
				return nil, errors.Join(err, teardownErr)
			}

			return nil, err
		}
	}

	return stack, nil
}

// ensureE2EImages builds each fixture once per Go test process. Individual
// stacks consume tagged images, so one parallel stack cannot delete an image
// while another is starting from it.
func ensureE2EImages(ctx context.Context, root string) error {
	e2eImagesOnce.Do(func() {
		errE2EImages = buildE2EImage(
			ctx,
			filepath.Join(root, "tests", "fakeupstream"),
			e2eFakeUpstreamImageRepository,
		)
		if errE2EImages != nil {
			return
		}

		errE2EImages = buildE2EImage(ctx, root, e2eAppImageRepository)
	})

	return errE2EImages
}

func buildE2EImage(ctx context.Context, imageContext, repository string) error {
	buildCtx, cancel := context.WithTimeout(ctx, e2eImageBuildTimeout)
	defer cancel()

	container, err := testcontainers.GenericContainer(
		buildCtx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:   imageContext,
					Repo:      repository,
					Tag:       e2eImageTag,
					KeepImage: true,
				},
			},
		},
	)
	if err != nil {
		if container == nil {
			return ctxerrors.Wrapf(err, "build e2e image %s", repository)
		}

		if terminateErr := container.Terminate(ctx); terminateErr != nil {
			return errors.Join(
				ctxerrors.Wrapf(err, "build e2e image %s", repository),
				ctxerrors.Wrapf(
					terminateErr,
					"remove failed e2e image builder %s",
					repository,
				),
			)
		}

		return ctxerrors.Wrapf(err, "build e2e image %s", repository)
	}

	if err := container.Terminate(ctx); err != nil {
		return ctxerrors.Wrapf(err, "remove e2e image builder %s", repository)
	}

	return nil
}

func e2eDatabaseDriverFromEnv() (e2eDatabaseDriver, error) {
	driver := e2eDatabaseDriver(os.Getenv(e2eDatabaseDriverEnv))
	if driver == "" {
		return e2eDatabaseDriverPostgres, nil
	}

	switch driver {
	case e2eDatabaseDriverPostgres, e2eDatabaseDriverSQLite:
		return driver, nil
	default:
		return "", ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"%s must be postgres or sqlite, got %q",
			e2eDatabaseDriverEnv,
			driver,
		)
	}
}

func (s *E2EStack) setupNetwork(ctx context.Context, _ string) error {
	// network.New (the non-deprecated spelling) is not vendored and cannot be
	// fetched offline; GenericNetwork is the vendored equivalent.
	req := testcontainers.GenericNetworkRequest{ //nolint:staticcheck,lll // see above
		NetworkRequest: testcontainers.NetworkRequest{ //nolint:staticcheck,lll // see above
			Name:       s.networkName,
			Attachable: true,
		},
	}

	net, err := testcontainers.GenericNetwork(ctx, req) //nolint:staticcheck,lll // see above
	if err != nil {
		return ctxerrors.Wrap(err, "create e2e network")
	}

	s.Network = net

	return nil
}

func (s *E2EStack) setupPostgresOnNetwork(ctx context.Context, _ string) error {
	pg, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(e2eDBName),
		tcpostgres.WithUsername(e2eDBUser),
		tcpostgres.WithPassword(e2eDBPass),
		withNetworkAlias(s.networkName, e2ePostgresAlias),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").
				WithStartupTimeout(e2eBootTimeout),
		),
	)
	if err != nil {
		return ctxerrors.Wrap(err, "start e2e postgres")
	}

	s.Postgres = pg

	return nil
}

func (s *E2EStack) setupUpstream(
	ctx context.Context,
	_ string,
	opts E2EOptions,
) error {
	up, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:    e2eFakeUpstreamImage,
				Networks: []string{s.networkName},
				NetworkAliases: map[string][]string{
					s.networkName: {e2eUpstreamAlias},
				},
				Env: map[string]string{
					fakeUpstreamFailFirstStreamEnv: strconv.FormatBool(
						opts.FailFirstUpstreamStream,
					),
				},
				ExposedPorts: []string{e2eUpstreamPort + "/tcp"},
				WaitingFor: wait.ForHTTP("/v1/models").
					WithPort(e2eUpstreamPort + "/tcp").
					WithStartupTimeout(e2eImageBuildTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		return abandonE2EContainer(ctx, up, err, "start fake upstream")
	}

	s.Upstream = up

	return nil
}

func (s *E2EStack) setupMCPServer(ctx context.Context, root string) error {
	srv, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:   filepath.Join(root, "tests", "mcpserver"),
					KeepImage: true,
				},
				Env:      map[string]string{"CHATZ_MCP_MARKER": "e2e-marker"},
				Networks: []string{s.networkName},
				NetworkAliases: map[string][]string{
					s.networkName: {e2eMCPServerAlias},
				},
				ExposedPorts: []string{e2eMCPServerPort + "/tcp"},
				// The streamable-HTTP /mcp endpoint answers 406 to a plain GET
				// (it wants an SSE Accept header) — any HTTP response means up.
				WaitingFor: wait.ForHTTP("/mcp").
					WithPort(e2eMCPServerPort + "/tcp").
					WithStatusCodeMatcher(func(status int) bool {
						return status == e2eMCPUnauthorizedGet ||
							(status >= 200 && status < 400)
					}).
					WithStartupTimeout(e2eImageBuildTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		return abandonE2EContainer(ctx, srv, err, "start mcp server")
	}

	s.MCPServer = srv

	return nil
}

func (s *E2EStack) setupApp(
	ctx context.Context,
	_ string,
	opts E2EOptions,
	databaseDriver e2eDatabaseDriver,
) error {
	env := map[string]string{
		"LOG_LEVEL":                "info",
		"LOG_FORMAT":               "text",
		"LOG_ADD_SOURCE":           "true",
		"CHATZ_HTTP_LISTENADDRESS": ":" + e2eAppPort,
		"CHATZ_DB_HOSTNAME":        e2ePostgresAlias,
		"CHATZ_DB_PORT":            "5432",
		"CHATZ_DB_USERNAME":        e2eDBUser,
		"CHATZ_DB_PASSWORD":        e2eDBPass,
		"CHATZ_DB_NAME":            e2eDBName,
		"CHATZ_DB_ISSSL":           "false",
		"CHATZ_AUTH_PASSWORDLESS":  "false",
		"CHATZ_SHOWCASE_MODE":      strconv.FormatBool(opts.ShowcaseMode),
	}
	if databaseDriver == e2eDatabaseDriverSQLite {
		env["CHATZ_DB_DRIVER"] = string(e2eDatabaseDriverSQLite)
		env["CHATZ_DB_SQLITE_PATH"] = "/data/chatz.sqlite"
	}

	if err := applyUpstreamEnv(env, opts.RealUpstream); err != nil {
		return ctxerrors.Wrap(err, "apply e2e upstream environment")
	}

	app, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:    e2eAppImage,
				Cmd:      []string{"run"},
				Networks: []string{s.networkName},
				NetworkAliases: map[string][]string{
					s.networkName: {e2eAppAlias},
				},
				ExposedPorts: []string{e2eAppPort + "/tcp"},
				Env:          env,
				WaitingFor: wait.ForHTTP("/healthz").
					WithPort(e2eAppPort + "/tcp").
					WithStartupTimeout(e2eImageBuildTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		return abandonE2EContainer(ctx, app, err, "start app")
	}

	s.App = app

	return nil
}

// abandonE2EContainer keeps a partially started test container from outliving
// the stack when a readiness check fails. GenericContainer can return both a
// container and an error, so the caller must terminate that container before
// propagating the original failure.
func abandonE2EContainer(
	ctx context.Context,
	container testcontainers.Container,
	err error,
	what string,
) error {
	if container == nil {
		return ctxerrors.Wrap(err, what)
	}

	if terminateErr := container.Terminate(ctx); terminateErr != nil {
		return errors.Join(
			ctxerrors.Wrap(err, what),
			ctxerrors.Wrapf(
				terminateErr,
				"terminate e2e container after %s",
				what,
			),
		)
	}

	return ctxerrors.Wrap(err, what)
}

// applyUpstreamEnv sets the app's model-discovery upstream. A real stack gets
// only the provider keys referenced by its validated upstream configuration;
// the fake stack stays keyless.
func applyUpstreamEnv(env map[string]string, realUpstream bool) error {
	if realUpstream {
		upstreamsJSON := os.Getenv(e2eUpstreamsEnv)

		upstreamConfig := config.Config{
			UpstreamsJSON: upstreamsJSON,
		}

		upstreams, err := upstreamConfig.Upstreams()
		if err != nil {
			return ctxerrors.Wrap(err, "validate real e2e upstreams")
		}

		env[e2eUpstreamsEnv] = upstreamsJSON

		for _, upstream := range upstreams {
			if upstream.APIKeyEnv == "" {
				continue
			}

			env[upstream.APIKeyEnv] = os.Getenv(upstream.APIKeyEnv)
		}

		return nil
	}

	env[e2eUpstreamsEnv] = fmt.Sprintf(
		`[{"name":"e2e-fake","baseUrl":"http://%s:%s/v1"}]`,
		e2eUpstreamAlias,
		e2eUpstreamPort,
	)

	return nil
}

// Teardown terminates containers newest-first, then removes the network.
// It returns every cleanup error so the test cannot falsely pass while leaving
// project-owned resources behind.
func (s *E2EStack) Teardown(ctx context.Context) error {
	containers := []struct {
		name      string
		container testcontainers.Container
	}{
		{name: "app", container: s.App},
		{name: "mcp server", container: s.MCPServer},
		{name: "upstream", container: s.Upstream},
	}

	var teardownErrs []error

	for _, resource := range containers {
		if resource.container == nil {
			continue
		}

		if err := resource.container.Terminate(ctx); err != nil {
			teardownErrs = append(teardownErrs,
				ctxerrors.Wrapf(err, "terminate e2e %s", resource.name))
		}
	}

	if s.Postgres != nil {
		if err := s.Postgres.Terminate(ctx); err != nil {
			teardownErrs = append(teardownErrs,
				ctxerrors.Wrap(err, "terminate e2e postgres"))
		}
	}

	if s.Network != nil {
		if err := s.Network.Remove(ctx); err != nil {
			teardownErrs = append(teardownErrs,
				ctxerrors.Wrap(err, "remove e2e network"))
		}
	}

	return errors.Join(teardownErrs...)
}

// withNetworkAlias attaches a module container (driven via its Run helper,
// not a raw request) to the given network under an alias.
func withNetworkAlias(
	name, alias string,
) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Networks = append(req.Networks, name)

		if req.NetworkAliases == nil {
			req.NetworkAliases = map[string][]string{}
		}

		req.NetworkAliases[name] = append(req.NetworkAliases[name], alias)

		return nil
	}
}

// repoRoot resolves the repository root from this file's compile-time path
// (<root>/tests/testinfra/e2e.go) so the Dockerfile build contexts resolve no
// matter the test's working directory.
func repoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", ctxerrors.New("cannot resolve repo root from runtime.Caller")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile))), nil
}
