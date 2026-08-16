// Package httpserver is the servicepack service that runs chatz's HTTP API.
// It self-bootstraps from the environment: parse config, open the DB (running
// migrations + wiring the generated repositories), build the collaborators
// (secrets box, model upstreams, MCP manager, auth), and serve the generated
// StrictServerInterface until the context is cancelled.
package httpserver

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/buildinfo"
	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	"github.com/psyb0t/chatz/internal/pkg/core/chats"
	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/chatz/internal/pkg/http/server"
	chatzlogging "github.com/psyb0t/chatz/internal/pkg/logging"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/chatz/internal/pkg/metrics"
	"github.com/psyb0t/chatz/internal/pkg/operations"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/chatz/internal/pkg/usage"
	"github.com/psyb0t/chatz/internal/pkg/webassets"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/drivers/anthropic"
	"github.com/psyb0t/elelem/drivers/openai"
	"github.com/psyb0t/elelem/elelemtest"
)

// ServiceName is the service-manager registration key.
const ServiceName = "http-server"

const (
	usageStage           = "chat"
	driverOptionCapacity = 4
	commitEnvVar         = "GIT_COMMIT"
)

// Timeouts for the internal observability (metrics/pprof) listeners.
const (
	auxReadHeaderTimeout  = 5 * time.Second
	auxShutdownTimeout    = 5 * time.Second
	upstreamHTTPKeepAlive = 30 * time.Second
)

// metricsPath is where the Prometheus exposition is served on the metrics port.
const metricsPath = "/metrics"

// Service wires the HTTP API together and owns the resources it opens (the DB
// handle + the MCP subprocesses/clients), releasing them on Stop. It also runs
// the internal-only observability listeners (/metrics + pprof) alongside the
// API, sharing the same metrics registry the usage recorder emits into.
type Service struct {
	server      *server.Server
	addr        string
	db          *db.Handle
	mcp         *mcp.Manager
	metrics     *metrics.Metrics
	metricsAddr string
	pprofAddr   string
}

// setGlobalScope stamps the facts that describe this BINARY onto every line the
// process logs, under any context. They go on the global tier because that tier
// is never serialized: a service name or commit sent across a hop would
// overwrite the receiving service's own, and its logs would then name the wrong
// deploy.
//
// The commit is whatever the build injected; unset in a dev run, so it is only
// set when non-empty rather than stamping an empty field on every line.
func setGlobalScope() {
	attrs := []ctxscope.Attribute{
		ctxscope.Attr(chatzlogging.ScopeKeyService, ServiceName),
	}

	if buildinfo.Version != "" {
		attrs = append(attrs,
			ctxscope.Attr(chatzlogging.ScopeKeyVersion, buildinfo.Version),
		)
	}

	if commit := buildCommit(); commit != "" {
		attrs = append(attrs,
			ctxscope.Attr(chatzlogging.ScopeKeyCommit, commit),
		)
	}

	ctxscope.SetGlobal(attrs...)
}

func buildCommit() string {
	if buildinfo.Commit != "" {
		return buildinfo.Commit
	}

	return os.Getenv(commitEnvVar)
}

// New parses config, connects the DB, discovers model upstreams, connects the
// enabled MCP servers (best-effort), and assembles the HTTP server. It runs
// when the service manager starts the service (the `run` command), not at
// registration time — so the DB is only touched when the app actually boots.
func New() (*Service, error) {
	ctx := context.Background()

	setGlobalScope()

	logger := ctxscope.GetLogger(ctx)

	cfg, database, err := newDatabase(ctx)
	if err != nil {
		return nil, err
	}

	box, err := cfg.SecretsBox()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build secrets box")
	}

	if box == nil {
		logger.Warn("secrets key not configured; MCP secret sealing disabled",
			"reason", "secrets_not_configured")
	}

	met, err := metrics.New()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build metrics")
	}

	reg, err := discoverUpstreams(ctx, cfg, met)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "discover upstreams")
	}

	readiness, err := newReadiness(database, reg, cfg)
	if err != nil {
		return nil, err
	}

	mgr := mcp.NewManager(box)
	connectMCPServers(ctx, mgr)

	webFS, err := loadWebFS(ctx)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "load web assets")
	}

	srv := buildServer(cfg, box, reg, mgr, readiness, webFS)

	logger.Info("http service initialized",
		"addr", cfg.HTTPListenAddress, "models", len(reg.Models()))

	return &Service{
		server:      srv,
		addr:        cfg.HTTPListenAddress,
		db:          database,
		mcp:         mgr,
		metrics:     met,
		metricsAddr: cfg.MetricsListenAddress,
		pprofAddr:   cfg.ProfilingListenAddress,
	}, nil
}

func newDatabase(ctx context.Context) (config.Config, *db.Handle, error) {
	cfg, err := config.Parse()
	if err != nil {
		return config.Config{}, nil, ctxerrors.Wrap(err, "parse config")
	}

	database, err := db.Connect(ctx, cfg.DBConfig())
	if err != nil {
		return config.Config{}, nil, ctxerrors.Wrap(err, "connect db")
	}

	return cfg, database, nil
}

func newReadiness(
	database *db.Handle,
	registry *upstreams.Registry,
	cfg config.Config,
) (*operations.Service, error) {
	readiness, err := operations.New(
		database,
		registry,
		cfg.ReadinessConfig(buildinfo.Version, buildCommit()),
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build readiness service")
	}

	return readiness, nil
}

// buildServer assembles the HTTP API server from its collaborators.
func buildServer(
	cfg config.Config,
	box *secrets.Box,
	reg *upstreams.Registry,
	mgr *mcp.Manager,
	readiness *operations.Service,
	webFS fs.FS,
) *server.Server {
	return server.New(server.Deps{
		Auth:       auth.New(repositories.Q, cfg.AuthPasswordless),
		Chats:      chats.New(repositories.Q, reg, mgr, cfg.ShowcaseMode),
		MCP:        mgr,
		MCPServers: mcp.NewServerStore(repositories.Q),
		Models:     reg,
		Readiness:  readiness,
		Secrets:    box,
		WebFS:      webFS,
	})
}

// discoverUpstreams resolves the configured model upstreams and probes each for
// its available models, returning the merged registry. Each upstream's client
// is wrapped with the usage recorder so every streamed chat round emits LLM
// metrics and persists one llm_usage row.
func discoverUpstreams(
	ctx context.Context,
	cfg config.Config,
	met *metrics.Metrics,
) (*upstreams.Registry, error) {
	ups, err := cfg.Upstreams()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "resolve upstreams")
	}

	runtime, err := cfg.UpstreamRuntimeConfig()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "resolve upstream runtime limits")
	}

	upstreamNames := make([]string, 0, len(ups))
	for _, upstream := range ups {
		upstreamNames = append(upstreamNames, upstream.Name)
	}

	health := upstreams.NewHealthTracker(upstreamNames)

	clients, err := buildUpstreamClients(
		ups,
		runtime,
		cfg.ForceRealLLM,
		met,
		health,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build upstream clients")
	}

	reg := upstreams.Discover(
		ctx, ups, func(u config.Upstream) *elelem.Client {
			return clients[u.Name]
		}, runtime.ConnectTimeout, health,
	)
	if err := reg.SetDefault(cfg.DefaultModel); err != nil {
		return nil, ctxerrors.Wrap(err, "validate configured default model")
	}

	if err := reg.SetFallbacks(ups); err != nil {
		return nil, ctxerrors.Wrap(err, "validate configured model fallbacks")
	}

	return reg, nil
}

func buildUpstreamClients(
	configured []config.Upstream,
	runtime config.UpstreamRuntime,
	forceRealLLM bool,
	met *metrics.Metrics,
	health *upstreams.HealthTracker,
) (map[string]*elelem.Client, error) {
	clients := make(map[string]*elelem.Client, len(configured))
	limits := upstreams.RuntimeLimits{
		MaxConcurrent:     runtime.Concurrency,
		FirstTokenTimeout: runtime.FirstTokenTimeout,
		TurnTimeout:       runtime.TurnTimeout,
	}

	for _, upstream := range configured {
		driver, err := newUpstreamDriver(
			upstream,
			runtime.ConnectTimeout,
			forceRealLLM,
		)
		if err != nil {
			return nil, ctxerrors.Wrapf(err, "build upstream %q", upstream.Name)
		}

		driver, err = upstreams.WrapDriver(
			driver,
			upstream.Name,
			limits,
			health,
		)
		if err != nil {
			return nil, ctxerrors.Wrapf(
				err,
				"apply runtime limits to upstream %q",
				upstream.Name,
			)
		}

		driver = elelem.WithRetry(driver, elelem.RetryConfig{})
		driver = usage.Wrap(driver, met, usageStage, true)
		clients[upstream.Name] = elelem.New(driver)
	}

	return clients, nil
}

// testDoubleDriver returns the driver every upstream resolves to under
// `go test`, or nil when the caller should build a real one.
func testDoubleDriver(forceRealLLM bool) elelem.Driver {
	if !testing.Testing() || forceRealLLM {
		return nil
	}

	if driver := elelemtest.GlobalScriptedDriver(); driver != nil {
		return driver
	}

	return elelemtest.NewScriptedDriver()
}

// newUpstreamDriver builds the Driver for one upstream.
//
// Under `go test` it resolves to the scripted double instead of a real
// provider, so tests drive the app through THIS path rather than hand-building
// a driver alongside it and quietly diverging. A test programs the double via
// elelemtest.SetGlobalScriptedDriver; with none installed the double has no
// turns, so the first Stream fails loudly offline rather than dialling out.
// The real-LLM suites opt back in with CHATZ_FORCE_REAL_LLM.
func newUpstreamDriver(
	upstream config.Upstream,
	connectTimeout time.Duration,
	forceRealLLM bool,
) (elelem.Driver, error) {
	if double := testDoubleDriver(forceRealLLM); double != nil {
		return double, nil
	}

	apiKey := upstream.APIKey()

	httpClient, err := newUpstreamHTTPClient(connectTimeout)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "create upstream HTTP client")
	}

	switch upstream.Provider {
	case config.UpstreamProviderOpenAI:
		options := make([]openai.DriverOption, 0, driverOptionCapacity)
		options = append(options, openai.WithoutEnvironmentDefaults())
		if upstream.BaseURL != "" {
			options = append(options, openai.WithBaseURL(upstream.BaseURL))
		}

		if apiKey != "" {
			options = append(options, openai.WithAPIKey(apiKey))
		}

		options = append(options, openai.WithHTTPClient(httpClient))

		return openai.NewDriver(options...), nil
	case config.UpstreamProviderAnthropic:
		options := make([]anthropic.DriverOption, 0, driverOptionCapacity)
		options = append(options, anthropic.WithoutEnvironmentDefaults())
		if upstream.BaseURL != "" {
			options = append(options, anthropic.WithBaseURL(upstream.BaseURL))
		}

		if apiKey != "" {
			options = append(options, anthropic.WithAPIKey(apiKey))
		}

		options = append(options, anthropic.WithHTTPClient(httpClient))

		return anthropic.NewDriver(options...), nil
	default:
		return nil, ctxerrors.Wrapf(
			config.ErrInvalidUpstream,
			"upstream %q has unsupported provider %q",
			upstream.Name,
			upstream.Provider,
		)
	}
}

// newUpstreamHTTPClient creates a transport exclusively for one provider
// client. Client.Timeout remains unset because a streamed turn has separate
// first-token and total-turn deadlines; only connection establishment is
// bounded here.
func newUpstreamHTTPClient(connectTimeout time.Duration) (*http.Client, error) {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ctxerrors.Wrap(
			commerr.ErrInvalidState,
			"default HTTP transport is not an *http.Transport",
		)
	}

	transport := baseTransport.Clone()
	dialer := newUpstreamDialer(connectTimeout)

	transport.DialContext = dialer.DialContext
	transport.TLSHandshakeTimeout = connectTimeout

	return &http.Client{Transport: transport}, nil
}

func newUpstreamDialer(connectTimeout time.Duration) net.Dialer {
	return net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: upstreamHTTPKeepAlive,
	}
}

// loadWebFS returns the embedded SPA filesystem and warns if only the committed
// placeholder is present (i.e. `make web-embed` was never run for this binary).
func loadWebFS(ctx context.Context) (fs.FS, error) {
	webFS, err := webassets.FS()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "load embedded web assets")
	}

	if webassets.IsPlaceholder(webFS) {
		ctxscope.GetLogger(ctx).Warn(
			"serving placeholder SPA; run `make web-embed` to embed the real "+
				"build", "reason", "spa_not_built",
		)
	}

	return webFS, nil
}

// Name identifies the service to the service manager.
func (s *Service) Name() string {
	return ServiceName
}

// Run serves the HTTP API until ctx is cancelled, at which point Serve drains
// and shuts down echo and returns nil. The internal observability listeners
// (/metrics + pprof) run alongside on their own ports and are torn down after
// the API drains. A failure on an aux port only warns — observability never
// takes down the API.
func (s *Service) Run(ctx context.Context) error {
	ctxscope.GetLogger(ctx).Info("starting service",
		"service", ServiceName, "addr", s.addr)

	stopMetrics := serveAux(ctx, "metrics", s.metricsAddr, s.metricsMux())
	defer stopMetrics()

	stopPprof := serveAux(ctx, "pprof", s.pprofAddr, s.metrics.PprofHandler())
	defer stopPprof()

	if err := s.server.Serve(ctx, s.addr); err != nil {
		return ctxerrors.Wrap(err, "serve http")
	}

	return nil
}

// metricsMux serves the Prometheus exposition at metricsPath.
func (s *Service) metricsMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(metricsPath, s.metrics.Handler())

	return mux
}

// serveAux starts an internal observability listener in a goroutine and returns
// a shutdown func. An empty addr is a no-op (the listener is disabled). A
// bind/serve failure only warns — these ports are best-effort and must not take
// the API down.
func serveAux(
	ctx context.Context,
	name, addr string,
	handler http.Handler,
) func() {
	logger := ctxscope.GetLogger(ctx)

	if addr == "" {
		logger.Debug("observability listener disabled",
			"listener", name, "reason", "addr_empty")

		return func() {}
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: auxReadHeaderTimeout,
	}

	go func() {
		logger.Info("observability listener started",
			"listener", name, "addr", addr)

		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Warn("observability listener failed",
				"listener", name, "addr", addr, "err", err,
				"reason", "aux_listener_failed")
		}
	}()

	return func() {
		shutCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), auxShutdownTimeout,
		)
		defer cancel()

		if err := srv.Shutdown(shutCtx); err != nil {
			logger.Warn("observability listener shutdown failed",
				"listener", name, "err", err, "reason", "aux_shutdown_failed")
		}
	}
}

// Stop releases the MCP clients (killing stdio subprocesses) and closes the DB
// handle. MCP close failures only warn — a broken subprocess mustn't block a
// clean DB shutdown.
func (s *Service) Stop(ctx context.Context) error {
	logger := ctxscope.GetLogger(ctx)
	logger.Info("stopping service", "service", ServiceName)

	if err := s.mcp.Close(); err != nil {
		logger.Warn("close mcp manager failed",
			"err", err, "reason", "mcp_close_failed")
	}

	if err := s.db.Close(); err != nil {
		return ctxerrors.Wrap(err, "close db")
	}

	return nil
}

// connectMCPServers loads the enabled MCP servers from the DB and connects
// each. Best-effort: a server that fails to connect logs a warning but never
// blocks startup — its tools are simply absent until it comes back.
func connectMCPServers(ctx context.Context, mgr *mcp.Manager) {
	logger := ctxscope.GetLogger(ctx)

	repo := repositories.MCPServer

	rows, err := repo.WithContext(ctx).Where(repo.Enabled.Is(true)).Find()
	if err != nil {
		logger.Warn("load mcp servers failed",
			"err", err, "reason", "mcp_load_failed")

		return
	}

	// Connect in the background so a dead/slow server can't delay startup; the
	// manager records each server's connecting → connected/failed status.
	for _, srv := range rows {
		mgr.ConnectAsync(ctx, srv)
	}

	logger.Info("mcp servers connecting", "count", len(rows))
}
