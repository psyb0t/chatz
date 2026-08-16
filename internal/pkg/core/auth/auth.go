// Package auth is chatz's authentication domain: password hashing (bcrypt),
// server-side opaque session tokens (only the SHA-256 hash is stored; the raw
// token lives in the client's HttpOnly cookie), first-run admin bootstrap, and
// optional single-user passwordless auto-login. Handlers call this layer; it
// owns the users + sessions tables.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// CookieName is the session cookie holding the raw opaque token.
	CookieName = "chatz_session"

	// sessionTokenBytes is the entropy of an opaque session token.
	sessionTokenBytes = 32

	// sessionTTL bounds how long a session stays valid after issue.
	sessionTTL = 30 * 24 * time.Hour
)

// Service is the auth domain service. Construct with New and share it — it is
// stateless beyond its query handle and config.
type Service struct {
	q            *repositories.Query
	passwordless bool
}

// New builds the Service over the given query handle. passwordless enables
// single-user auto-login (see PasswordlessLogin).
func New(q *repositories.Query, passwordless bool) *Service {
	return &Service{
		q:            q,
		passwordless: passwordless,
	}
}

// NeedsSetup reports whether the install has no users yet (first-run).
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.q.User.WithContext(ctx).Count()
	if err != nil {
		return false, ctxerrors.Wrap(err, "count users")
	}

	return count == 0, nil
}

// Bootstrap creates the first user as admin. It fails with ErrSetupClosed once
// any user exists, and ErrInvalidInput without a password (the admin must have
// one — passwordless is opt-in per-config, not for the bootstrap account).
func (s *Service) Bootstrap(
	ctx context.Context,
	username, password string,
) (*models.User, error) {
	if strings.TrimSpace(password) == "" {
		return nil, ErrInvalidInput
	}

	need, err := s.NeedsSetup(ctx)
	if err != nil {
		return nil, err
	}

	if !need {
		return nil, ErrSetupClosed
	}

	return s.CreateUser(ctx, username, password, true)
}

// CreateUser provisions a user. An empty password creates a passwordless
// account (login only via PasswordlessLogin). Returns ErrUserExists if the
// username is taken, ErrInvalidInput if it's empty.
func (s *Service) CreateUser(
	ctx context.Context,
	username, password string,
	isAdmin bool,
) (*models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrInvalidInput
	}

	taken, err := s.usernameTaken(ctx, username)
	if err != nil {
		return nil, err
	}

	if taken {
		return nil, ErrUserExists
	}

	var passwordHash *string

	if password != "" {
		hashed, hErr := hashPassword(password)
		if hErr != nil {
			return nil, hErr
		}

		passwordHash = &hashed
	}

	user := &models.User{
		Username:     username,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
	}
	if err := s.q.User.WithContext(ctx).Create(user); err != nil {
		return nil, ctxerrors.Wrap(err, "create user")
	}

	return user, nil
}

// ListUsers returns every user ordered by creation time (admin view).
func (s *Service) ListUsers(ctx context.Context) ([]*models.User, error) {
	repo := s.q.User

	users, err := repo.WithContext(ctx).Order(repo.CreatedAt).Find()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list users")
	}

	return users, nil
}

// DeleteUser removes the user with id; the DB cascades their sessions + chats.
// Returns commerr.ErrNotFound when no user has that id.
func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	repo := s.q.User

	result, err := repo.WithContext(ctx).Where(repo.ID.Eq(id)).Delete()
	if err != nil {
		return ctxerrors.Wrap(err, "delete user")
	}

	if result.RowsAffected == 0 {
		return commerr.ErrNotFound
	}

	return nil
}

// Login verifies the password and issues a session, returning the raw token to
// set as a cookie. Unknown user and bad password both return
// ErrInvalidCredentials (no user enumeration).
func (s *Service) Login(
	ctx context.Context,
	username, password string,
) (string, *models.User, error) {
	user, err := s.userByName(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, ErrInvalidCredentials
		}

		return "", nil, err
	}

	if user.PasswordHash == nil ||
		!checkPassword(*user.PasswordHash, password) {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

// PasswordlessLogin issues a session without a password, but ONLY when
// passwordless is enabled AND the install has exactly one user. Otherwise
// ErrPasswordlessUnavailable — it must never silently pick an account on a
// multi-user install.
func (s *Service) PasswordlessLogin(
	ctx context.Context,
) (string, *models.User, error) {
	if !s.passwordless {
		return "", nil, ErrPasswordlessUnavailable
	}

	count, err := s.q.User.WithContext(ctx).Count()
	if err != nil {
		return "", nil, ctxerrors.Wrap(err, "count users")
	}

	if count != 1 {
		return "", nil, ErrPasswordlessUnavailable
	}

	user, err := s.q.User.WithContext(ctx).First()
	if err != nil {
		return "", nil, ctxerrors.Wrap(err, "load single user")
	}

	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

// Authenticate resolves a raw session token to its user, or ErrSessionInvalid
// if the session is missing, expired, or its user is gone.
func (s *Service) Authenticate(
	ctx context.Context,
	token string,
) (*models.User, error) {
	sessionRepo := s.q.Session

	session, err := sessionRepo.WithContext(ctx).
		Where(sessionRepo.TokenHash.Eq(tokenHash(token))).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionInvalid
		}

		return nil, ctxerrors.Wrap(err, "find session")
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionInvalid
	}

	userRepo := s.q.User

	user, err := userRepo.WithContext(ctx).
		Where(userRepo.ID.Eq(session.UserID)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionInvalid
		}

		return nil, ctxerrors.Wrap(err, "load session user")
	}

	return user, nil
}

// Logout revokes the session for the given raw token. A missing session is not
// an error — logout is idempotent.
func (s *Service) Logout(ctx context.Context, token string) error {
	sessionRepo := s.q.Session

	_, err := sessionRepo.WithContext(ctx).
		Where(sessionRepo.TokenHash.Eq(tokenHash(token))).
		Delete()
	if err != nil {
		return ctxerrors.Wrap(err, "delete session")
	}

	return nil
}

func (s *Service) usernameTaken(
	ctx context.Context,
	username string,
) (bool, error) {
	userRepo := s.q.User

	count, err := userRepo.WithContext(ctx).
		Where(userRepo.Username.Eq(username)).
		Count()
	if err != nil {
		return false, ctxerrors.Wrap(err, "count users by name")
	}

	return count > 0, nil
}

func (s *Service) userByName(
	ctx context.Context,
	username string,
) (*models.User, error) {
	userRepo := s.q.User

	user, err := userRepo.WithContext(ctx).
		Where(userRepo.Username.Eq(username)).
		First()
	if err != nil {
		return nil, err //nolint:wrapcheck // caller maps gorm.ErrRecordNotFound
	}

	return user, nil
}

func (s *Service) createSession(
	ctx context.Context,
	userID uuid.UUID,
) (string, error) {
	raw, hash, err := newSessionToken()
	if err != nil {
		return "", err
	}

	session := &models.Session{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	if err := s.q.Session.WithContext(ctx).Create(session); err != nil {
		return "", ctxerrors.Wrap(err, "create session")
	}

	return raw, nil
}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", ctxerrors.Wrap(err, "hash password")
	}

	return string(hashed), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	) == nil
}

// newSessionToken returns a raw opaque token and its SHA-256 hash. The raw
// token goes to the client; only the hash is persisted.
func newSessionToken() (string, string, error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", ctxerrors.Wrap(err, "read random")
	}

	raw := base64.RawURLEncoding.EncodeToString(buf)

	return raw, tokenHash(raw), nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}
