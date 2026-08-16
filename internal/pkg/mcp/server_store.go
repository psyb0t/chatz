package mcp

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"gorm.io/gorm"
)

// ServerStore is the persistence core for MCP server rows. The HTTP handlers
// drive it instead of reaching into the repositories directly, so the wire
// layer never knows about gorm — a missing row surfaces as
// commerr.ErrNotFound, which the handlers map to a 404. The live
// connection lifecycle stays in Manager; this type owns only the DB side.
type ServerStore struct {
	q *repositories.Query
}

// NewServerStore builds a ServerStore over the given query handle.
func NewServerStore(q *repositories.Query) *ServerStore {
	return &ServerStore{q: q}
}

// List returns all configured servers ordered by name.
func (s *ServerStore) List(ctx context.Context) ([]*models.MCPServer, error) {
	repo := s.q.MCPServer

	rows, err := repo.WithContext(ctx).Order(repo.Name).Find()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list mcp servers")
	}

	return rows, nil
}

// Get returns the server with id, or commerr.ErrNotFound if none exists.
func (s *ServerStore) Get(
	ctx context.Context,
	id uuid.UUID,
) (*models.MCPServer, error) {
	repo := s.q.MCPServer

	srv, err := repo.WithContext(ctx).Where(repo.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, commerr.ErrNotFound
		}

		return nil, ctxerrors.Wrap(err, "get mcp server")
	}

	return srv, nil
}

// Create persists a new server row.
func (s *ServerStore) Create(
	ctx context.Context,
	srv *models.MCPServer,
) error {
	if err := s.q.MCPServer.WithContext(ctx).Create(srv); err != nil {
		return ctxerrors.Wrap(err, "create mcp server")
	}

	return nil
}

// Save persists changes to an existing server row.
func (s *ServerStore) Save(
	ctx context.Context,
	srv *models.MCPServer,
) error {
	if err := s.q.MCPServer.WithContext(ctx).Save(srv); err != nil {
		return ctxerrors.Wrap(err, "save mcp server")
	}

	return nil
}

// Delete removes the server row with id. Deleting a row that isn't there is a
// no-op (callers Get first to produce a 404), so this never reports not-found.
func (s *ServerStore) Delete(ctx context.Context, id uuid.UUID) error {
	repo := s.q.MCPServer

	_, err := repo.WithContext(ctx).Where(repo.ID.Eq(id)).Delete()
	if err != nil {
		return ctxerrors.Wrap(err, "delete mcp server")
	}

	return nil
}
