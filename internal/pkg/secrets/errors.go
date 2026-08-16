package secrets

import "errors"

// Secrets errors. Declared with errors.New so they stay comparable across
// ctxerrors.Wrap layers via errors.Is.
var (
	// ErrInvalidKey is returned when the AEAD key is not exactly KeySize bytes.
	ErrInvalidKey = errors.New("secrets: key must be 32 bytes")

	// ErrCiphertextTooShort is returned when a value to Open is shorter than
	// the nonce prefix — it can't be authentic ciphertext.
	ErrCiphertextTooShort = errors.New("secrets: ciphertext too short")

	// ErrNotConfigured is returned when a nil Box (no CHATZ_SECRETS_KEY set)
	// is asked to seal or open a non-empty secret. Boot still succeeds without
	// a key; only storing/reading a secret needs one — so a plaintext secret is
	// never written when encryption is unavailable.
	ErrNotConfigured = errors.New("secrets: no encryption key configured")
)
