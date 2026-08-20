package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateConfiguredUpstream walks the upstream-entry rejection branches:
// a missing name, a missing or unsupported provider, and (via validateModels)
// a duplicate model id and an empty model id. A well-formed entry passes.
func TestValidateConfiguredUpstream(t *testing.T) {
	t.Parallel()

	valid := Upstream{
		Name:     "openai",
		Provider: UpstreamProviderOpenAI,
		Models:   []Model{{ID: "gpt"}},
	}

	testCases := []struct {
		name     string
		upstream Upstream
		wantErr  bool
	}{
		{name: "valid", upstream: valid},
		{
			name:     "missing name",
			upstream: Upstream{Provider: UpstreamProviderOpenAI},
			wantErr:  true,
		},
		{
			name:     "missing provider",
			upstream: Upstream{Name: "x"},
			wantErr:  true,
		},
		{
			name: "unsupported provider",
			upstream: Upstream{
				Name:     "x",
				Provider: UpstreamProvider("cohere"),
			},
			wantErr: true,
		},
		{
			name: "duplicate model",
			upstream: Upstream{
				Name:     "x",
				Provider: UpstreamProviderOpenAI,
				Models:   []Model{{ID: "m"}, {ID: "m"}},
			},
			wantErr: true,
		},
		{
			name: "empty model id",
			upstream: Upstream{
				Name:     "x",
				Provider: UpstreamProviderOpenAI,
				Models:   []Model{{ID: ""}},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := validateConfiguredUpstream(tc.upstream, 0)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidUpstream)

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestValidateFallbackModels covers every rejection branch: an empty fallback
// id, a model that falls back to itself, and a duplicate fallback, plus the
// valid case with distinct non-empty ids.
func TestValidateFallbackModels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		model   Model
		wantErr bool
	}{
		{
			name:  "valid distinct fallbacks",
			model: Model{ID: "a", FallbackModels: []string{"b", "c"}},
		},
		{
			name:    "empty fallback entry",
			model:   Model{ID: "a", FallbackModels: []string{""}},
			wantErr: true,
		},
		{
			name:    "self reference",
			model:   Model{ID: "a", FallbackModels: []string{"a"}},
			wantErr: true,
		},
		{
			name:    "duplicate fallback",
			model:   Model{ID: "a", FallbackModels: []string{"b", "b"}},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateFallbackModels("up", tc.model)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidUpstream)

				return
			}

			assert.NoError(t, err)
		})
	}
}
