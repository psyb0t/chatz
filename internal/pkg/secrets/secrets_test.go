package secrets

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) []byte {
	t.Helper()

	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	return key
}

// TestBox_OpenErrorBranches covers Open + OpenMap failure paths: a blob shorter
// than the nonce, a correctly sized but unauthenticated blob, a sealed blob
// that decrypts to non-JSON, and garbage passed to OpenMap.
func TestBox_OpenErrorBranches(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	_, err = box.Open([]byte{1, 2, 3})
	require.ErrorIs(t, err, ErrCiphertextTooShort)

	_, err = box.Open(make([]byte, nonceSize+16))
	require.Error(t, err)

	sealed, err := box.Seal([]byte("not-json"))
	require.NoError(t, err)

	_, err = box.OpenMap(sealed)
	require.Error(t, err)

	_, err = box.OpenMap(make([]byte, nonceSize+16))
	require.Error(t, err)
}

func TestNew_RejectsBadKeyLength(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		size int
	}{
		{"empty", 0},
		{"short", 16},
		{"long", 48},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(make([]byte, tc.size))
			require.ErrorIs(t, err, ErrInvalidKey)
		})
	}
}

func TestNewFromBase64(t *testing.T) {
	t.Parallel()

	encoded := base64.StdEncoding.EncodeToString(testKey(t))

	box, err := NewFromBase64(encoded)
	require.NoError(t, err)
	require.NotNil(t, box)

	_, err = NewFromBase64("not!base64!")
	require.Error(t, err)
}

func TestSealOpen_RoundTrip(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	plaintext := []byte("bearer sk-super-secret-value")

	sealed, err := box.Seal(plaintext)
	require.NoError(t, err)
	assert.NotContains(t, string(sealed), "secret") // stored blob is opaque

	opened, err := box.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, plaintext, opened)
}

func TestSeal_FreshNoncePerCall(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	a, err := box.Seal([]byte("same"))
	require.NoError(t, err)

	b, err := box.Seal([]byte("same"))
	require.NoError(t, err)

	// Same plaintext must not produce identical blobs (random nonce).
	assert.False(t, bytes.Equal(a, b))
}

func TestOpen_RejectsTamperedBlob(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	sealed, err := box.Seal([]byte("authentic"))
	require.NoError(t, err)

	sealed[len(sealed)-1] ^= 0xff // flip a ciphertext byte

	_, err = box.Open(sealed)
	require.Error(t, err)
}

func TestOpen_RejectsWrongKey(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	sealed, err := box.Seal([]byte("authentic"))
	require.NoError(t, err)

	other, err := New(make([]byte, KeySize)) // all-zero key
	require.NoError(t, err)

	_, err = other.Open(sealed)
	require.Error(t, err)
}

func TestOpen_RejectsTooShort(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	_, err = box.Open([]byte("tiny"))
	require.ErrorIs(t, err, ErrCiphertextTooShort)
}

func TestSealOpenMap_RoundTrip(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	headers := map[string]string{
		"Authorization": "Bearer xyz",
		"X-Api-Key":     "k-123",
	}

	sealed, err := box.SealMap(headers)
	require.NoError(t, err)

	opened, err := box.OpenMap(sealed)
	require.NoError(t, err)
	assert.Equal(t, headers, opened)
}

func TestSealMap_EmptyIsNil(t *testing.T) {
	t.Parallel()

	box, err := New(testKey(t))
	require.NoError(t, err)

	sealed, err := box.SealMap(nil)
	require.NoError(t, err)
	assert.Nil(t, sealed)

	opened, err := box.OpenMap(nil)
	require.NoError(t, err)
	assert.Nil(t, opened)
}
