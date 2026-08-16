package config

import (
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Upstreams(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		json          string
		wantLen       int
		wantErr       bool
		wantFirstName string
		wantProvider  UpstreamProvider
	}{
		{
			name:    "empty leaves no upstreams configured",
			json:    "",
			wantLen: 0,
		},
		{
			name:          "single upstream",
			json:          `[{"name":"ollama","provider":"openai","baseUrl":"http://x:11434/v1"}]`,
			wantLen:       1,
			wantFirstName: "ollama",
			wantProvider:  UpstreamProviderOpenAI,
		},
		{
			name:          "multiple upstreams merge",
			json:          `[{"name":"ollama","provider":"openai"},{"name":"openai","provider":"openai"}]`,
			wantLen:       2,
			wantFirstName: "ollama",
			wantProvider:  UpstreamProviderOpenAI,
		},
		{
			name:          "anthropic provider",
			json:          `[{"name":"claude","provider":"anthropic"}]`,
			wantLen:       1,
			wantFirstName: "claude",
			wantProvider:  UpstreamProviderAnthropic,
		},
		{
			name: "model metadata",
			json: `[
  {"name":"gateway","provider":"openai","models":[
    {"id":"analysis","alias":"Deep","contextWindow":128000,
     "maxOutputTokens":8192,"supportsTools":true,"supportsFiles":true,
     "expectedFirstTokenLatencyMs":900,
     "inputPricePerMillionTokens":{"amountSmallestUnit":15,"currency":"USD"},
     "outputPricePerMillionTokens":{"amountSmallestUnit":60,"currency":"USD"}}
  ]}
]`,
			wantLen:       1,
			wantFirstName: "gateway",
			wantProvider:  UpstreamProviderOpenAI,
		},
		{
			name:    "invalid json errors",
			json:    `{bad`,
			wantErr: true,
		},
		{
			name:    "missing name errors",
			json:    `[{"provider":"openai"}]`,
			wantErr: true,
		},
		{
			name:    "missing provider errors",
			json:    `[{"name":"gateway"}]`,
			wantErr: true,
		},
		{
			name:    "unsupported provider errors",
			json:    `[{"name":"bad","provider":"other"}]`,
			wantErr: true,
		},
		{
			name:    "duplicate names error",
			json:    `[{"name":"same","provider":"openai"},{"name":"same","provider":"openai"}]`,
			wantErr: true,
		},
		{
			name:    "model metadata without id errors",
			json:    `[{"name":"gateway","provider":"openai","models":[{}]}]`,
			wantErr: true,
		},
		{
			name:    "duplicate model metadata errors",
			json:    `[{"name":"gateway","provider":"openai","models":[{"id":"a"},{"id":"a"}]}]`,
			wantErr: true,
		},
		{
			name: "negative expected latency errors",
			json: `[
  {"name":"gateway","provider":"openai","models":[
    {"id":"a","expectedFirstTokenLatencyMs":-1}
  ]}
]`,
			wantErr: true,
		},
		{
			name: "negative token price errors",
			json: `[
  {"name":"gateway","provider":"openai","models":[
    {"id":"a","inputPricePerMillionTokens":{
      "amountSmallestUnit":-1,"currency":"USD"}}
  ]}
]`,
			wantErr: true,
		},
		{
			name: "invalid token price currency errors",
			json: `[
  {"name":"gateway","provider":"openai","models":[
    {"id":"a","inputPricePerMillionTokens":{
      "amountSmallestUnit":1,"currency":"usd"}}
  ]}
]`,
			wantErr: true,
		},
		{
			name: "token price without currency errors",
			json: `[
  {"name":"gateway","provider":"openai","models":[
    {"id":"a","outputPricePerMillionTokens":{
      "amountSmallestUnit":1,"currency":""}}
  ]}
]`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{UpstreamsJSON: tc.json}

			got, err := cfg.Upstreams()
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Len(t, got, tc.wantLen)
			if tc.wantLen == 0 {
				return
			}

			assert.Equal(t, tc.wantFirstName, got[0].Name)
			assert.Equal(t, tc.wantProvider, got[0].Provider)

			if tc.name == "model metadata" {
				require.Len(t, got[0].Models, 1)
				assert.Equal(t, "Deep", got[0].Models[0].Alias)
				assert.Equal(t, 128000, got[0].Models[0].ContextWindow)
				require.NotNil(t, got[0].Models[0].SupportsTools)
				assert.True(t, *got[0].Models[0].SupportsTools)
				require.NotNil(t, got[0].Models[0].SupportsFiles)
				assert.True(t, *got[0].Models[0].SupportsFiles)
				assert.Equal(
					t,
					int64(900),
					got[0].Models[0].FirstTokenLatencyMs,
				)
				inputPrice := got[0].Models[0].InputTokenPrice
				require.NotNil(t, inputPrice)
				assert.Equal(
					t,
					int64(15),
					inputPrice.AmountSmallestUnit,
				)
				assert.Equal(
					t,
					"USD",
					inputPrice.Currency,
				)

				outputPrice := got[0].Models[0].OutputTokenPrice
				require.NotNil(t, outputPrice)
				assert.Equal(
					t,
					int64(60),
					outputPrice.AmountSmallestUnit,
				)
			}
		})
	}
}

func TestUpstream_APIKey(t *testing.T) {
	t.Setenv("MY_TEST_KEY", "sekret")

	assert.Empty(t, Upstream{APIKeyEnv: ""}.APIKey())
	assert.Equal(t, "sekret", Upstream{APIKeyEnv: "MY_TEST_KEY"}.APIKey())
	assert.Empty(t, Upstream{APIKeyEnv: "UNSET_VAR_XYZ"}.APIKey())
}

func TestConfig_Upstreams_APIKeyEnvironment(t *testing.T) {
	t.Setenv("CHATZ_TEST_UPSTREAM_KEY", "configured-key")

	testCases := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name: "configured key is present",
			json: `[
  {"name":"gateway","provider":"openai","apiKeyEnv":"CHATZ_TEST_UPSTREAM_KEY"}
]`,
		},
		{
			name: "referenced key is missing",
			json: `[
  {"name":"gateway","provider":"openai","apiKeyEnv":"CHATZ_TEST_MISSING_UPSTREAM_KEY"}
]`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{UpstreamsJSON: tc.json}

			_, err := cfg.Upstreams()
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidUpstream)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestConfig_DBConfig(t *testing.T) {
	t.Parallel()

	cfg := Config{
		DBDriver:            db.DriverPostgres,
		DBHostname:          "h",
		DBPort:              5433,
		DBUsername:          "u",
		DBPassword:          "p",
		DBName:              "d",
		DBIsSSL:             true,
		DBSQLitePath:        "/data/chatz.sqlite",
		DBSQLiteBusyTimeout: time.Second,
	}

	got := cfg.DBConfig()

	assert.Equal(t, "h", got.Hostname)
	assert.Equal(t, 5433, got.Port)
	assert.Equal(t, "u", got.Username)
	assert.Equal(t, "p", got.Password)
	assert.Equal(t, "d", got.Database)
	assert.True(t, got.IsSSL)
	assert.Equal(t, db.DriverPostgres, got.Driver)
	assert.Equal(t, "/data/chatz.sqlite", got.SQLitePath)
	assert.Equal(t, time.Second, got.SQLiteBusyTimeout)
}

func TestConfig_ParseShowcaseMode(t *testing.T) {
	t.Setenv("CHATZ_SHOWCASE_MODE", "true")

	cfg, err := Parse()
	require.NoError(t, err)
	assert.True(t, cfg.ShowcaseMode)
}

func TestConfig_UpstreamRuntimeConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name: "valid",
			config: Config{
				UpstreamConnectTimeout:    time.Second,
				UpstreamFirstTokenTimeout: 2 * time.Second,
				UpstreamTurnTimeout:       3 * time.Second,
				UpstreamConcurrency:       1,
			},
		},
		{
			name: "nonpositive connection timeout",
			config: Config{
				UpstreamFirstTokenTimeout: time.Second,
				UpstreamTurnTimeout:       time.Second,
				UpstreamConcurrency:       1,
			},
			wantErr: ErrInvalidUpstreamRuntime,
		},
		{
			name: "first token exceeds turn timeout",
			config: Config{
				UpstreamConnectTimeout:    time.Second,
				UpstreamFirstTokenTimeout: 2 * time.Second,
				UpstreamTurnTimeout:       time.Second,
				UpstreamConcurrency:       1,
			},
			wantErr: ErrInvalidUpstreamRuntime,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.config.UpstreamRuntimeConfig()
			if tc.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
