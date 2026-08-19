// Package config loads chatz's configuration from environment variables via
// gonfiguration. LLM upstreams are provided as a JSON array in CHATZ_UPSTREAMS;
// the app queries each provider for its models and merges the results. MCP
// servers are configured separately in the DB and UI.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/chatz/internal/pkg/operations"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/gonfiguration"
)

const (
	iso4217CurrencyCodeLength = 3
	tokenPriceDirectionInput  = "input"
	tokenPriceDirectionOutput = "output"
)

// UpstreamProvider identifies the Elelem driver used by an LLM upstream.
type UpstreamProvider string

// Supported upstream providers.
const (
	UpstreamProviderOpenAI    UpstreamProvider = "openai"
	UpstreamProviderAnthropic UpstreamProvider = "anthropic"
)

var (
	// ErrInvalidUpstream marks an invalid CHATZ_UPSTREAMS entry.
	ErrInvalidUpstream = errors.New("invalid upstream")

	// ErrInvalidUpstreamRuntime marks invalid LLM request bounds.
	ErrInvalidUpstreamRuntime = errors.New("invalid upstream runtime limits")
)

// Config is the full app configuration, parsed from the environment.
type Config struct {
	HTTPListenAddress      string `default:":8080" env:"CHATZ_HTTP_LISTENADDRESS"`      //nolint:lll // env tag
	MetricsListenAddress   string `default:":9091" env:"CHATZ_METRICS_LISTENADDRESS"`   //nolint:lll // env tag
	ProfilingListenAddress string `default:":6060" env:"CHATZ_PROFILING_LISTENADDRESS"` //nolint:lll // env tag

	DBHostname          string        `default:"localhost"                      env:"CHATZ_DB_HOSTNAME"`            //nolint:lll // env tag
	DBPort              int           `default:"5432"                           env:"CHATZ_DB_PORT"`                //nolint:lll // env tag
	DBUsername          string        `default:"chatz"                          env:"CHATZ_DB_USERNAME"`            //nolint:lll // env tag
	DBPassword          string        `default:"chatz"                          env:"CHATZ_DB_PASSWORD"`            //nolint:lll // env tag
	DBName              string        `default:"chatz"                          env:"CHATZ_DB_NAME"`                //nolint:lll // env tag
	DBIsSSL             bool          `default:"false"                          env:"CHATZ_DB_ISSSL"`               //nolint:lll // env tag
	DBDriver            db.Driver     `default:"postgres"                       env:"CHATZ_DB_DRIVER"`              //nolint:lll // env tag
	DBSQLitePath        string        `default:"/data/chatz.sqlite"             env:"CHATZ_DB_SQLITE_PATH"`         //nolint:lll // env tag
	DBSQLiteBusyTimeout time.Duration `default:"5s"                             env:"CHATZ_DB_SQLITE_BUSY_TIMEOUT"` //nolint:lll // env tag
	BackupStatusPath    string        `default:"/data/chatz-backup-status.json" env:"CHATZ_BACKUP_STATUS_PATH"`     //nolint:lll // env tag
	BackupMaxAge        time.Duration `default:"24h"                            env:"CHATZ_BACKUP_MAX_AGE"`         //nolint:lll // env tag

	SessionSecret    string `env:"CHATZ_SESSION_SECRET"`
	SecretsKey       string `env:"CHATZ_SECRETS_KEY"`
	AuthPasswordless bool   `default:"false"            env:"CHATZ_AUTH_PASSWORDLESS"` //nolint:lll // env tag
	ShowcaseMode     bool   `default:"false"            env:"CHATZ_SHOWCASE_MODE"`     //nolint:lll // env tag
	DefaultModel     string `env:"CHATZ_DEFAULT_MODEL"`

	UpstreamConnectTimeout    time.Duration `default:"10s" env:"CHATZ_UPSTREAM_CONNECT_TIMEOUT"`     //nolint:lll // env tag
	UpstreamFirstTokenTimeout time.Duration `default:"45s" env:"CHATZ_UPSTREAM_FIRST_TOKEN_TIMEOUT"` //nolint:lll // env tag
	UpstreamTurnTimeout       time.Duration `default:"5m"  env:"CHATZ_UPSTREAM_TURN_TIMEOUT"`        //nolint:lll // env tag
	UpstreamConcurrency       int           `default:"8"   env:"CHATZ_UPSTREAM_CONCURRENCY"`         //nolint:lll // env tag

	// UpstreamsJSON is a JSON array of Upstream. Empty leaves Chatz running
	// without model providers until the operator configures an Elelem driver.
	UpstreamsJSON string `env:"CHATZ_UPSTREAMS"`

	// ForceRealLLM makes upstream drivers talk to the real provider even under
	// `go test`, where they otherwise resolve to the scripted double. Nothing
	// in the repo sets it: the operator opts in from their own `.env`, which
	// `make test-real` forwards into the container. Leave it false elsewhere.
	ForceRealLLM bool `default:"false" env:"CHATZ_FORCE_REAL_LLM"`
}

// UpstreamRuntime is the validated set of request bounds shared by every
// configured model upstream. Per-upstream customization belongs in a future
// explicit config surface rather than unvalidated JSON fields.
type UpstreamRuntime struct {
	ConnectTimeout    time.Duration
	FirstTokenTimeout time.Duration
	TurnTimeout       time.Duration
	Concurrency       int
}

// Upstream configures one provider endpoint. OpenAI-compatible services such
// as Ollama and vLLM use the openai provider.
type Upstream struct {
	Name      string           `json:"name"`
	Provider  UpstreamProvider `json:"provider"`
	BaseURL   string           `json:"baseUrl"`
	APIKeyEnv string           `json:"apiKeyEnv"`
	Models    []Model          `json:"models"`
}

// Model supplies optional public metadata for one model that an upstream may
// advertise. Discovery remains authoritative: an entry never creates a model
// that the provider did not list.
type Model struct {
	ID                  string      `json:"id"`
	Alias               string      `json:"alias"`
	FallbackModels      []string    `json:"fallbackModels"`
	ContextWindow       int         `json:"contextWindow"`
	MaxOutputTokens     int         `json:"maxOutputTokens"`
	SupportsTools       *bool       `json:"supportsTools"`
	SupportsReasoning   *bool       `json:"supportsReasoning"`
	SupportsVision      *bool       `json:"supportsVision"`
	SupportsFiles       *bool       `json:"supportsFiles"`
	FirstTokenLatencyMs int64       `json:"expectedFirstTokenLatencyMs"`
	InputTokenPrice     *TokenPrice `json:"inputPricePerMillionTokens"`
	OutputTokenPrice    *TokenPrice `json:"outputPricePerMillionTokens"`
}

// TokenPrice is the configured price for one million input or output tokens.
// AmountSmallestUnit uses the currency's smallest unit; it is never a float.
type TokenPrice struct {
	AmountSmallestUnit int64  `json:"amountSmallestUnit"`
	Currency           string `json:"currency"`
}

// APIKey resolves the key from the env var the config named in APIKeyEnv. Empty
// when there is no ref or the var is unset — fine for keyless local endpoints
// like Ollama. Keys are referenced by env name, never stored inline (secrets
// rule); os.Getenv here dereferences a runtime-chosen name, which gonfiguration
// (fixed field bindings) can't express.
func (u Upstream) APIKey() string {
	if u.APIKeyEnv == "" {
		return ""
	}

	return os.Getenv(u.APIKeyEnv)
}

// Parse reads the configuration from the environment.
func Parse() (Config, error) {
	cfg := Config{}
	if err := gonfiguration.Parse(&cfg); err != nil {
		return Config{}, ctxerrors.Wrap(err, "parse config")
	}

	return cfg, nil
}

// SecretsBox builds the AEAD box used to seal/open at-rest secrets (MCP HTTP
// headers + stdio env) from CHATZ_SECRETS_KEY (base64, 32 bytes). An unset
// key returns a nil box: the app boots without encryption, and any attempt to
// store an actual secret later fails with secrets.ErrNotConfigured rather than
// writing plaintext.
func (c Config) SecretsBox() (*secrets.Box, error) {
	if c.SecretsKey == "" {
		//nolint:nilnil // nil box legitimately means "secrets not configured"
		return nil, nil
	}

	box, err := secrets.NewFromBase64(c.SecretsKey)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "build secrets box")
	}

	return box, nil
}

// DBConfig maps the DB fields to the db package's connection config.
func (c Config) DBConfig() db.Config {
	return db.Config{
		Driver:            c.DBDriver,
		Hostname:          c.DBHostname,
		Port:              c.DBPort,
		Username:          c.DBUsername,
		Password:          c.DBPassword,
		Database:          c.DBName,
		IsSSL:             c.DBIsSSL,
		SQLitePath:        c.DBSQLitePath,
		SQLiteBusyTimeout: c.DBSQLiteBusyTimeout,
	}
}

// ReadinessConfig maps the local backup-marker policy to the operations
// service. New validates it during startup, before the HTTP server is exposed.
func (c Config) ReadinessConfig(appVersion, commit string) operations.Config {
	return operations.Config{
		AppVersion:       appVersion,
		Commit:           commit,
		DatabaseDriver:   c.DBDriver,
		BackupStatusPath: c.BackupStatusPath,
		BackupMaxAge:     c.BackupMaxAge,
	}
}

// UpstreamRuntimeConfig converts configured LLM runtime bounds into a typed,
// validated value before startup creates any outbound provider clients.
func (c Config) UpstreamRuntimeConfig() (UpstreamRuntime, error) {
	runtime := UpstreamRuntime{
		ConnectTimeout:    c.UpstreamConnectTimeout,
		FirstTokenTimeout: c.UpstreamFirstTokenTimeout,
		TurnTimeout:       c.UpstreamTurnTimeout,
		Concurrency:       c.UpstreamConcurrency,
	}

	if runtime.ConnectTimeout <= 0 ||
		runtime.FirstTokenTimeout <= 0 ||
		runtime.TurnTimeout <= 0 ||
		runtime.Concurrency <= 0 ||
		runtime.FirstTokenTimeout > runtime.TurnTimeout {
		return UpstreamRuntime{}, ctxerrors.Wrap(
			ErrInvalidUpstreamRuntime,
			"validate upstream runtime configuration",
		)
	}

	return runtime, nil
}

// Upstreams returns the explicitly configured Elelem driver upstreams.
func (c Config) Upstreams() ([]Upstream, error) {
	if c.UpstreamsJSON == "" {
		return nil, nil
	}

	var upstreams []Upstream
	if err := json.Unmarshal([]byte(c.UpstreamsJSON), &upstreams); err != nil {
		return nil, ctxerrors.Wrap(err, "parse CHATZ_UPSTREAMS json")
	}

	names := make(map[string]struct{}, len(upstreams))
	for index := range upstreams {
		upstream, err := validateConfiguredUpstream(upstreams[index], index)
		if err != nil {
			return nil, err
		}

		if _, exists := names[upstream.Name]; exists {
			return nil, ctxerrors.Wrapf(
				ErrInvalidUpstream,
				"CHATZ_UPSTREAMS contains duplicate name %q",
				upstream.Name,
			)
		}

		names[upstream.Name] = struct{}{}
		upstreams[index] = upstream
	}

	return upstreams, nil
}

func validateConfiguredUpstream(
	upstream Upstream,
	index int,
) (Upstream, error) {
	if upstream.Name == "" {
		return Upstream{}, ctxerrors.Wrapf(
			ErrInvalidUpstream,
			"CHATZ_UPSTREAMS entry %d has no name",
			index,
		)
	}

	if err := validateModels(upstream); err != nil {
		return Upstream{}, err
	}

	if upstream.Provider == "" {
		return Upstream{}, ctxerrors.Wrapf(
			ErrInvalidUpstream,
			"CHATZ_UPSTREAMS entry %q has no provider",
			upstream.Name,
		)
	}

	switch upstream.Provider {
	case UpstreamProviderOpenAI, UpstreamProviderAnthropic:
	default:
		return Upstream{}, ctxerrors.Wrapf(
			ErrInvalidUpstream,
			"CHATZ_UPSTREAMS entry %q has unsupported provider %q",
			upstream.Name,
			upstream.Provider,
		)
	}

	if err := validateAPIKeyEnvironment(upstream); err != nil {
		return Upstream{}, err
	}

	return upstream, nil
}

func validateAPIKeyEnvironment(upstream Upstream) error {
	if upstream.APIKeyEnv == "" {
		return nil
	}

	apiKey, found := os.LookupEnv(upstream.APIKeyEnv)
	if found && apiKey != "" {
		return nil
	}

	return ctxerrors.Wrapf(
		ErrInvalidUpstream,
		"upstream %q references missing required environment variable %q",
		upstream.Name,
		upstream.APIKeyEnv,
	)
}

func validateModels(upstream Upstream) error {
	modelIDs := make(map[string]struct{}, len(upstream.Models))
	for modelIndex, model := range upstream.Models {
		if _, exists := modelIDs[model.ID]; exists {
			return ctxerrors.Wrapf(
				ErrInvalidUpstream,
				"upstream %q has duplicate model metadata for %q",
				upstream.Name,
				model.ID,
			)
		}

		if err := validateModel(upstream.Name, model, modelIndex); err != nil {
			return err
		}

		modelIDs[model.ID] = struct{}{}
	}

	return nil
}

func validateModel(upstreamName string, model Model, modelIndex int) error {
	if model.ID == "" {
		return ctxerrors.Wrapf(
			ErrInvalidUpstream,
			"upstream %q model entry %d has no id",
			upstreamName,
			modelIndex,
		)
	}

	if model.ContextWindow < 0 ||
		model.MaxOutputTokens < 0 ||
		model.FirstTokenLatencyMs < 0 {
		return ctxerrors.Wrapf(
			ErrInvalidUpstream,
			//nolint:lll // one error message; a line break changes its API text
			"upstream %q model %q limits and expected latency must not be negative",
			upstreamName,
			model.ID,
		)
	}

	if err := validateTokenPrice(
		upstreamName,
		model.ID,
		tokenPriceDirectionInput,
		model.InputTokenPrice,
	); err != nil {
		return err
	}

	if err := validateTokenPrice(
		upstreamName,
		model.ID,
		tokenPriceDirectionOutput,
		model.OutputTokenPrice,
	); err != nil {
		return err
	}

	return validateFallbackModels(upstreamName, model)
}

func validateTokenPrice(
	upstreamName string,
	modelID string,
	direction string,
	price *TokenPrice,
) error {
	if price == nil {
		return nil
	}

	if price.AmountSmallestUnit < 0 {
		return ctxerrors.Wrapf(
			ErrInvalidUpstream,
			"upstream %q model %q %s token price must not be negative",
			upstreamName,
			modelID,
			direction,
		)
	}

	if !isISO4217Code(price.Currency) {
		return ctxerrors.Wrapf(
			ErrInvalidUpstream,
			//nolint:lll // one error message; a line break changes its API text
			"upstream %q model %q %s token price currency must be a three-letter uppercase code",
			upstreamName,
			modelID,
			direction,
		)
	}

	return nil
}

func isISO4217Code(currency string) bool {
	if len(currency) != iso4217CurrencyCodeLength {
		return false
	}

	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}

	return true
}

func validateFallbackModels(upstreamName string, model Model) error {
	fallbackIDs := make(map[string]struct{}, len(model.FallbackModels))
	for fallbackIndex, fallbackID := range model.FallbackModels {
		if fallbackID == "" {
			return ctxerrors.Wrapf(
				ErrInvalidUpstream,
				"upstream %q model %q fallback entry %d is empty",
				upstreamName,
				model.ID,
				fallbackIndex,
			)
		}

		if fallbackID == model.ID {
			return ctxerrors.Wrapf(
				ErrInvalidUpstream,
				"upstream %q model %q cannot fall back to itself",
				upstreamName,
				model.ID,
			)
		}

		if _, exists := fallbackIDs[fallbackID]; exists {
			return ctxerrors.Wrapf(
				ErrInvalidUpstream,
				"upstream %q model %q has duplicate fallback %q",
				upstreamName,
				model.ID,
				fallbackID,
			)
		}

		fallbackIDs[fallbackID] = struct{}{}
	}

	return nil
}
