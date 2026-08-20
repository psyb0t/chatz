package tiktoken_test

import (
	"strings"
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/tiktoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenizeRoundTrip is the invariant the demo stream relies on:
// concatenating the returned tokens reproduces the input byte-for-byte, so a
// canned response streams one delta per real token without corrupting text.
func TestTokenizeRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		text string
	}{
		{name: "empty", text: ""},
		{name: "ascii words", text: "hello world, this is a test."},
		{name: "unicode", text: "café, naïve résumé 🚀"},
		{name: "whitespace and newlines", text: "  leading\n\ttabbed\nlines  "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tokens, err := tiktoken.Tokenize(tc.text)
			require.NoError(t, err)
			assert.Equal(t, tc.text, strings.Join(tokens, ""))
		})
	}
}

// TestTokenizeNonEmptyProducesTokens guards against a silent regression where a
// non-empty input tokenizes to nothing.
func TestTokenizeNonEmptyProducesTokens(t *testing.T) {
	t.Parallel()

	tokens, err := tiktoken.Tokenize("tokens please")
	require.NoError(t, err)
	assert.NotEmpty(t, tokens)
}

// TestCountMatchesTokenize pins Count to the count Tokenize produces, since
// Count exists only to skip allocating the decoded strings.
func TestCountMatchesTokenize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		text string
	}{
		{name: "empty is zero", text: ""},
		{name: "sentence", text: "The quick brown fox."},
		{name: "unicode", text: "emoji 🚀 and accents é"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tokens, err := tiktoken.Tokenize(tc.text)
			require.NoError(t, err)

			count, err := tiktoken.Count(tc.text)
			require.NoError(t, err)
			assert.Equal(t, len(tokens), count)
		})
	}
}
