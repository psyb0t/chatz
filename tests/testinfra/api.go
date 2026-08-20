//go:build api

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

// The api stack is orchestrated from Go via testcontainers: one bridge network
// carrying Postgres, the fake OpenAI upstream (so model discovery returns real
// ids in the browser), the optional MCP fixture, and the app built from the
// prod Dockerfile (SPA + API on one
// origin). The browser (browser.go) joins the same network and reaches the app
// same-origin by alias, exactly like the compose stack.
const (
	apiPostgresAlias  = "postgres"
	apiUpstreamAlias  = "fakeupstream"
	apiMCPServerAlias = "mcpserver"
	apiAppAlias       = "backend"

	apiAppPort       = "8080"
	apiUpstreamPort  = "8000"
	apiMCPServerPort = "8000"

	apiDBName = "chatz"
	apiDBUser = "chatz"
	apiDBPass = "chatz"

	// Real-upstream config for real_chat: the app reuses the runner's
	// CHATZ_UPSTREAMS plus every configured provider key from the same .env as
	// make test-real.
	apiUpstreamsEnv      = "CHATZ_UPSTREAMS"
	apiRealModelEnv      = "CHATZ_API_REAL_MODEL"
	apiDefaultRealModel  = "groq-gpt-oss-120b"
	apiDatabaseDriverEnv = "CHATZ_API_DB_DRIVER"
	// fakeUpstreamFailFirstStreamEnv makes the fake upstream exhaust the retry
	// stack for its first streamed turn. Test-fixture control only.
	fakeUpstreamFailFirstStreamEnv = "CHATZ_API_FAIL_FIRST_STREAM"
	// fakeUpstreamResponseTextEnv pins the fake upstream's assistant turn to an
	// exact string — see APIOptions.UpstreamResponseText.
	fakeUpstreamResponseTextEnv = "CHATZ_API_RESPONSE_TEXT"

	// The app image is a multi-stage prod build (SPA + Go binary); the first
	// run pays the full build, later runs hit the layer cache (KeepImage).
	apiImageBuildTimeout = 8 * time.Minute
	apiBootTimeout       = 3 * time.Minute
	apiImageTag          = "current"

	apiFakeUpstreamImageRepository = "chatz-api-fakeupstream"
	apiAppImageRepository          = "chatz-api-app"
	apiFakeUpstreamImage           = "chatz-api-fakeupstream:current"
	apiAppImage                    = "chatz-api-app:current"

	apiMCPUnauthorizedGet = 406
)

var (
	//nolint:gochecknoglobals // Coordinates parallel test stacks.
	apiImagesOnce sync.Once
	errAPIImages  error
)

type apiDatabaseDriver string

const (
	apiDatabaseDriverPostgres apiDatabaseDriver = "postgres"
	apiDatabaseDriverSQLite   apiDatabaseDriver = "sqlite"
)

// APIOptions tunes the stack for a specific driver.
type APIOptions struct {
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
	// UpstreamResponseText pins the fake upstream's assistant turn to this
	// exact text, delivered in one chunk. Empty keeps its default
	// partial/pause/completed script. Drives the browser with a payload the
	// canned showcase responses cannot express — a malformed spec fence, say.
	UpstreamResponseText string
}

// apiNetwork is the slice of the (deprecated) testcontainers network type we
// use — declaring it locally keeps the deprecated symbol out of the struct.
type apiNetwork interface {
	Remove(context.Context) error
}

// APIStack holds the running api containers + the in-network URL the browser
// uses to reach the app. Call Teardown when done (from TestMain wrapping
// m.Run). Fields are exported so a test can inspect a container if needed.
type APIStack struct {
	Network   apiNetwork
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
func (s *APIStack) NetworkName() string {
	return s.networkName
}

// RealUpstreamConfigured reports whether the runner supplied an upstream
// configuration. Setup validates and forwards every referenced provider key.
func RealUpstreamConfigured() bool {
	return os.Getenv(apiUpstreamsEnv) != ""
}

// RealModel is the model id the real_chat driver selects, overridable via
// CHATZ_API_REAL_MODEL so the test tracks whatever provider .env points at.
func RealModel() string {
	if model := os.Getenv(apiRealModelEnv); model != "" {
		return model
	}

	return apiDefaultRealModel
}

// SetupAPI brings up network -> selected database -> upstream -> mcpserver ->
// app, each waited on before the next. Every error path tears down what was
// already started so no container or network leaks.
func SetupAPI(ctx context.Context, opts APIOptions) (*APIStack, error) {
	databaseDriver, err := apiDatabaseDriverFromEnv()
	if err != nil {
		return nil, err
	}

	root, err := repoRoot()
	if err != nil {
		return nil, err
	}

	if err := ensureAPIImages(ctx, root); err != nil {
		return nil, err
	}

	mcpURL := "http://" +
		net.JoinHostPort(apiMCPServerAlias, apiMCPServerPort) + "/mcp"
	stack := &APIStack{
		networkName:  fmt.Sprintf("chatz-api-%d", time.Now().UnixNano()),
		AppURL:       "http://" + net.JoinHostPort(apiAppAlias, apiAppPort),
		MCPServerURL: mcpURL,
	}

	steps := []func(context.Context, string) error{stack.setupNetwork}
	if databaseDriver == apiDatabaseDriverPostgres {
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

// ensureAPIImages builds each fixture once per Go test process. Individual
// stacks consume tagged images, so one parallel stack cannot delete an image
// while another is starting from it.
func ensureAPIImages(ctx context.Context, root string) error {
	apiImagesOnce.Do(func() {
		errAPIImages = buildAPIImage(
			ctx,
			filepath.Join(root, "tests", "fakeupstream"),
			apiFakeUpstreamImageRepository,
		)
		if errAPIImages != nil {
			return
		}

		errAPIImages = buildAPIImage(ctx, root, apiAppImageRepository)
	})

	return errAPIImages
}

func buildAPIImage(ctx context.Context, imageContext, repository string) error {
	buildCtx, cancel := context.WithTimeout(ctx, apiImageBuildTimeout)
	defer cancel()

	container, err := testcontainers.GenericContainer(
		buildCtx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:   imageContext,
					Repo:      repository,
					Tag:       apiImageTag,
					KeepImage: true,
				},
			},
		},
	)
	if err != nil {
		if container == nil {
			return ctxerrors.Wrapf(err, "build api image %s", repository)
		}

		if terminateErr := container.Terminate(ctx); terminateErr != nil {
			return errors.Join(
				ctxerrors.Wrapf(err, "build api image %s", repository),
				ctxerrors.Wrapf(
					terminateErr,
					"remove failed api image builder %s",
					repository,
				),
			)
		}

		return ctxerrors.Wrapf(err, "build api image %s", repository)
	}

	if err := container.Terminate(ctx); err != nil {
		return ctxerrors.Wrapf(err, "remove api image builder %s", repository)
	}

	return nil
}

func apiDatabaseDriverFromEnv() (apiDatabaseDriver, error) {
	driver := apiDatabaseDriver(os.Getenv(apiDatabaseDriverEnv))
	if driver == "" {
		return apiDatabaseDriverPostgres, nil
	}

	switch driver {
	case apiDatabaseDriverPostgres, apiDatabaseDriverSQLite:
		return driver, nil
	default:
		return "", ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"%s must be postgres or sqlite, got %q",
			apiDatabaseDriverEnv,
			driver,
		)
	}
}

func (s *APIStack) setupNetwork(ctx context.Context, _ string) error {
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
		return ctxerrors.Wrap(err, "create api network")
	}

	s.Network = net

	return nil
}

func (s *APIStack) setupPostgresOnNetwork(ctx context.Context, _ string) error {
	pg, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(apiDBName),
		tcpostgres.WithUsername(apiDBUser),
		tcpostgres.WithPassword(apiDBPass),
		withNetworkAlias(s.networkName, apiPostgresAlias),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").
				WithStartupTimeout(apiBootTimeout),
		),
	)
	if err != nil {
		return ctxerrors.Wrap(err, "start api postgres")
	}

	s.Postgres = pg

	return nil
}

func (s *APIStack) setupUpstream(
	ctx context.Context,
	_ string,
	opts APIOptions,
) error {
	up, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:    apiFakeUpstreamImage,
				Networks: []string{s.networkName},
				NetworkAliases: map[string][]string{
					s.networkName: {apiUpstreamAlias},
				},
				Env: map[string]string{
					fakeUpstreamFailFirstStreamEnv: strconv.FormatBool(
						opts.FailFirstUpstreamStream,
					),
					fakeUpstreamResponseTextEnv: opts.UpstreamResponseText,
				},
				ExposedPorts: []string{apiUpstreamPort + "/tcp"},
				WaitingFor: wait.ForHTTP("/v1/models").
					WithPort(apiUpstreamPort + "/tcp").
					WithStartupTimeout(apiImageBuildTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		return abandonAPIContainer(ctx, up, err, "start fake upstream")
	}

	s.Upstream = up

	return nil
}

func (s *APIStack) setupMCPServer(ctx context.Context, root string) error {
	srv, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:   filepath.Join(root, "tests", "mcpserver"),
					KeepImage: true,
				},
				Env:      map[string]string{"CHATZ_MCP_MARKER": "api-marker"},
				Networks: []string{s.networkName},
				NetworkAliases: map[string][]string{
					s.networkName: {apiMCPServerAlias},
				},
				ExposedPorts: []string{apiMCPServerPort + "/tcp"},
				// The streamable-HTTP /mcp endpoint answers 406 to a plain GET
				// (it wants an SSE Accept header) — any HTTP response means up.
				WaitingFor: wait.ForHTTP("/mcp").
					WithPort(apiMCPServerPort + "/tcp").
					WithStatusCodeMatcher(func(status int) bool {
						return status == apiMCPUnauthorizedGet ||
							(status >= 200 && status < 400)
					}).
					WithStartupTimeout(apiImageBuildTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		return abandonAPIContainer(ctx, srv, err, "start mcp server")
	}

	s.MCPServer = srv

	return nil
}

func (s *APIStack) setupApp(
	ctx context.Context,
	_ string,
	opts APIOptions,
	databaseDriver apiDatabaseDriver,
) error {
	env := map[string]string{
		"LOG_LEVEL":                "info",
		"LOG_FORMAT":               "text",
		"LOG_ADD_SOURCE":           "true",
		"CHATZ_HTTP_LISTENADDRESS": ":" + apiAppPort,
		"CHATZ_DB_HOSTNAME":        apiPostgresAlias,
		"CHATZ_DB_PORT":            "5432",
		"CHATZ_DB_USERNAME":        apiDBUser,
		"CHATZ_DB_PASSWORD":        apiDBPass,
		"CHATZ_DB_NAME":            apiDBName,
		"CHATZ_DB_ISSSL":           "false",
		"CHATZ_AUTH_PASSWORDLESS":  "false",
		"CHATZ_SHOWCASE_MODE":      strconv.FormatBool(opts.ShowcaseMode),
	}
	if databaseDriver == apiDatabaseDriverSQLite {
		env["CHATZ_DB_DRIVER"] = string(apiDatabaseDriverSQLite)
		env["CHATZ_DB_SQLITE_PATH"] = "/data/chatz.sqlite"
	}

	if err := applyUpstreamEnv(env, opts.RealUpstream); err != nil {
		return ctxerrors.Wrap(err, "apply api upstream environment")
	}

	app, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:    apiAppImage,
				Cmd:      []string{"run"},
				Networks: []string{s.networkName},
				NetworkAliases: map[string][]string{
					s.networkName: {apiAppAlias},
				},
				ExposedPorts: []string{apiAppPort + "/tcp"},
				Env:          env,
				WaitingFor: wait.ForHTTP("/healthz").
					WithPort(apiAppPort + "/tcp").
					WithStartupTimeout(apiImageBuildTimeout),
			},
			Started: true,
		},
	)
	if err != nil {
		return abandonAPIContainer(ctx, app, err, "start app")
	}

	s.App = app

	return nil
}

// abandonAPIContainer keeps a partially started test container from outliving
// the stack when a readiness check fails. GenericContainer can return both a
// container and an error, so the caller must terminate that container before
// propagating the original failure.
func abandonAPIContainer(
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
				"terminate api container after %s",
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
		upstreamsJSON := os.Getenv(apiUpstreamsEnv)

		upstreamConfig := config.Config{
			UpstreamsJSON: upstreamsJSON,
		}

		upstreams, err := upstreamConfig.Upstreams()
		if err != nil {
			return ctxerrors.Wrap(err, "validate real api upstreams")
		}

		env[apiUpstreamsEnv] = upstreamsJSON

		for _, upstream := range upstreams {
			if upstream.APIKeyEnv == "" {
				continue
			}

			env[upstream.APIKeyEnv] = os.Getenv(upstream.APIKeyEnv)
		}

		return nil
	}

	env[apiUpstreamsEnv] = fmt.Sprintf(
		`[{"name":"api-fake","provider":"openai","baseUrl":"http://%s:%s/v1"}]`,
		apiUpstreamAlias,
		apiUpstreamPort,
	)

	return nil
}

// Teardown terminates containers newest-first, then removes the network.
// It returns every cleanup error so the test cannot falsely pass while leaving
// project-owned resources behind.
func (s *APIStack) Teardown(ctx context.Context) error {
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
				ctxerrors.Wrapf(err, "terminate api %s", resource.name))
		}
	}

	if s.Postgres != nil {
		if err := s.Postgres.Terminate(ctx); err != nil {
			teardownErrs = append(teardownErrs,
				ctxerrors.Wrap(err, "terminate api postgres"))
		}
	}

	if s.Network != nil {
		if err := s.Network.Remove(ctx); err != nil {
			teardownErrs = append(teardownErrs,
				ctxerrors.Wrap(err, "remove api network"))
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
// (<root>/tests/testinfra/api.go) so the Dockerfile build contexts resolve no
// matter the test's working directory.
func repoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", ctxerrors.New("cannot resolve repo root from runtime.Caller")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile))), nil
}
