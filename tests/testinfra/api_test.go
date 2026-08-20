//go:build api

package testinfra

import (
	"testing"

	"github.com/psyb0t/ctxerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const (
	testOpenAIKeyEnv    = "CHATZ_TEST_OPENAI_KEY"
	testAnthropicKeyEnv = "CHATZ_TEST_ANTHROPIC_KEY"
	testAbandonError    = "deliberate api setup failure"
)

func TestApplyUpstreamEnv_RealForwardsEveryConfiguredKey(t *testing.T) {
	t.Setenv(
		apiUpstreamsEnv,
		`[
  {"name":"openai","provider":"openai","apiKeyEnv":"CHATZ_TEST_OPENAI_KEY"},
  {
    "name":"anthropic",
    "provider":"anthropic",
    "apiKeyEnv":"CHATZ_TEST_ANTHROPIC_KEY"
  }
]`,
	)
	t.Setenv(testOpenAIKeyEnv, "openai-test-key")
	t.Setenv(testAnthropicKeyEnv, "anthropic-test-key")

	env := map[string]string{}

	require.NoError(t, applyUpstreamEnv(env, true))
	assert.NotEmpty(t, env[apiUpstreamsEnv])
	assert.Equal(t, "openai-test-key", env[testOpenAIKeyEnv])
	assert.Equal(t, "anthropic-test-key", env[testAnthropicKeyEnv])
}

func TestAbandonAPIContainer_TerminatesCreatedContainer(t *testing.T) {
	ctx := t.Context()
	root, err := repoRoot()
	require.NoError(t, err)
	require.NoError(t, ensureAPIImages(ctx, root))

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: apiFakeUpstreamImage,
			},
			Started: true,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, container)

	err = abandonAPIContainer(
		ctx,
		container,
		ctxerrors.New(testAbandonError),
		"deliberate setup failure",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, testAbandonError)

	_, err = container.State(ctx)
	assert.Error(t, err)
}
