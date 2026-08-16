// Package secrets provides authenticated encryption (AES-256-GCM) for secret
// values stored at rest — MCP HTTP headers and stdio env vars. The key comes
// from CHATZ_SECRETS_KEY (base64, 32 bytes). Plaintext secrets never touch
// the DB; only sealed blobs (nonce-prefixed ciphertext) do.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"

	"github.com/psyb0t/ctxerrors"
)

const (
	// KeySize is the AES-256 key length in bytes.
	KeySize = 32

	// nonceSize is the standard GCM nonce length. Every Seal draws a fresh
	// random nonce and prepends it to the ciphertext.
	nonceSize = 12
)

// Box seals and opens secret values with a single AEAD key.
type Box struct {
	aead cipher.AEAD
}

// New builds a Box from a raw 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, ctxerrors.Wrapf(
			ErrInvalidKey, "got %d bytes", len(key),
		)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "new aes cipher")
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "new gcm")
	}

	return &Box{aead: aead}, nil
}

// NewFromBase64 builds a Box from a base64-encoded 32-byte key (the
// CHATZ_SECRETS_KEY form). Generate one with `openssl rand -base64 32`.
func NewFromBase64(encoded string) (*Box, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "decode base64 key")
	}

	return New(key)
}

// Seal encrypts plaintext, returning nonce||ciphertext. Every call uses a fresh
// random nonce, so sealing the same plaintext twice yields different blobs. A
// nil Box (no key configured) returns ErrNotConfigured — callers must not fall
// back to storing plaintext.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	if b == nil {
		return nil, ErrNotConfigured
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, ctxerrors.Wrap(err, "read nonce")
	}

	// Seal appends the ciphertext to nonce, so the nonce prefixes the result.
	return b.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a nonce||ciphertext blob produced by Seal. A tampered blob or
// wrong key fails authentication and returns an error. A nil Box (no key
// configured) returns ErrNotConfigured.
func (b *Box) Open(sealed []byte) ([]byte, error) {
	if b == nil {
		return nil, ErrNotConfigured
	}

	if len(sealed) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]

	plaintext, err := b.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "gcm open")
	}

	return plaintext, nil
}

// SealMap JSON-encodes a string map and seals it. An empty map returns nil so
// callers store NULL rather than a blob for "no secrets".
func (b *Box) SealMap(m map[string]string) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}

	raw, err := json.Marshal(m)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "marshal map")
	}

	return b.Seal(raw)
}

// OpenMap opens a blob produced by SealMap. A nil/empty blob returns a nil map
// (the "no secrets" case), not an error.
func (b *Box) OpenMap(sealed []byte) (map[string]string, error) {
	if len(sealed) == 0 {
		//nolint:nilnil // empty blob legitimately means "no secrets stored"
		return nil, nil
	}

	raw, err := b.Open(sealed)
	if err != nil {
		return nil, err
	}

	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, ctxerrors.Wrap(err, "unmarshal map")
	}

	return m, nil
}
