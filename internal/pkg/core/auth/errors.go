package auth

import "errors"

// Auth domain errors. The HTTP layer maps these to status codes (invalid
// credentials + invalid session -> 401, user-exists + setup-closed -> 409,
// invalid input -> 400). Declared with errors.New so they stay comparable
// across ctxerrors.Wrap layers via errors.Is.
var (
	// ErrInvalidCredentials is returned for an unknown user or a bad password.
	// Deliberately indistinguishable between the two (no user enumeration).
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrInvalidInput is returned for an empty username or password where one
	// is required.
	ErrInvalidInput = errors.New("auth: invalid input")

	// ErrUserExists is returned when the username is already taken.
	ErrUserExists = errors.New("auth: username already exists")

	// ErrSetupClosed is returned by Bootstrap once at least one user exists.
	ErrSetupClosed = errors.New("auth: setup already completed")

	// ErrSessionInvalid is returned for a missing, revoked, or expired session.
	ErrSessionInvalid = errors.New("auth: session invalid or expired")

	// ErrPasswordlessUnavailable is returned when passwordless auto-login is
	// disabled or the install is not a single-user one.
	ErrPasswordlessUnavailable = errors.New("auth: passwordless not available")
)
