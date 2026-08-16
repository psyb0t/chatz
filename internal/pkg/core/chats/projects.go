package chats

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

// ListProjects returns the caller's projects in stable name order.
func (s *Service) ListProjects(
	ctx context.Context,
	userID uuid.UUID,
) ([]*models.Project, error) {
	repo := s.query.Project

	projects, err := repo.WithContext(ctx).
		Where(repo.UserID.Eq(userID)).
		Order(repo.Name.Asc()).
		Find()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list projects")
	}

	return projects, nil
}

// CreateProject creates a named grouping for one caller's chats.
func (s *Service) CreateProject(
	ctx context.Context,
	userID uuid.UUID,
	name string,
) (*models.Project, error) {
	name, err := validatedProjectName(name)
	if err != nil {
		return nil, err
	}

	repo := s.query.Project

	existing, err := repo.WithContext(ctx).
		Where(repo.UserID.Eq(userID), repo.Name.Eq(name)).
		First()
	if err == nil && existing != nil {
		return nil, ctxerrors.Wrap(
			commerr.ErrConflict,
			"project name already exists",
		)
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ctxerrors.Wrap(err, "find project by name")
	}

	project := &models.Project{UserID: userID, Name: name}
	if err := repo.WithContext(ctx).Create(project); err != nil {
		return nil, ctxerrors.Wrap(err, "create project")
	}

	return project, nil
}

// RenameProject changes a caller-owned project's display name.
func (s *Service) RenameProject(
	ctx context.Context,
	projectID, userID uuid.UUID,
	name string,
) (*models.Project, error) {
	name, err := validatedProjectName(name)
	if err != nil {
		return nil, err
	}

	project, err := s.ownedProject(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	repo := s.query.Project

	existing, err := repo.WithContext(ctx).
		Where(repo.UserID.Eq(userID), repo.Name.Eq(name)).
		First()
	if err == nil && existing.ID != project.ID {
		return nil, ctxerrors.Wrap(
			commerr.ErrConflict,
			"project name already exists",
		)
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ctxerrors.Wrap(err, "find project by name")
	}

	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(project.ID)).
		Update(repo.Name, name); err != nil {
		return nil, ctxerrors.Wrap(err, "rename project")
	}

	project.Name = name

	return project, nil
}

// DeleteProject removes a project but deliberately preserves every chat by
// unassigning its members in the same transaction.
func (s *Service) DeleteProject(
	ctx context.Context,
	projectID, userID uuid.UUID,
) error {
	if err := s.query.Transaction(func(tx *repositories.Query) error {
		project, err := ownedProject(ctx, tx, projectID, userID)
		if err != nil {
			return err
		}

		chatRepo := tx.Chat
		if _, err := chatRepo.WithContext(ctx).
			Where(
				chatRepo.UserID.Eq(userID),
				chatRepo.ProjectID.Eq(projectID),
			).
			Update(chatRepo.ProjectID, nil); err != nil {
			return ctxerrors.Wrap(err, "clear project chats")
		}

		if _, err := tx.Project.WithContext(ctx).Delete(project); err != nil {
			return ctxerrors.Wrap(err, "delete project")
		}

		return nil
	}); err != nil {
		return ctxerrors.Wrap(err, "delete project transaction")
	}

	return nil
}

// AssignProject attaches a chat to a project owned by the same caller.
func (s *Service) AssignProject(
	ctx context.Context,
	chatID, projectID, userID uuid.UUID,
) (*models.Chat, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	if _, err := s.ownedProject(ctx, projectID, userID); err != nil {
		return nil, err
	}

	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chat.ID)).
		Update(repo.ProjectID, projectID); err != nil {
		return nil, ctxerrors.Wrap(err, "assign chat project")
	}

	chat.ProjectID = &projectID

	return chat, nil
}

// ClearProject removes a chat's project assignment without changing the chat.
func (s *Service) ClearProject(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (*models.Chat, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chat.ID)).
		Update(repo.ProjectID, nil); err != nil {
		return nil, ctxerrors.Wrap(err, "clear chat project")
	}

	chat.ProjectID = nil

	return chat, nil
}

func (s *Service) ownedProject(
	ctx context.Context,
	projectID, userID uuid.UUID,
) (*models.Project, error) {
	return ownedProject(ctx, s.query, projectID, userID)
}

func ownedProject(
	ctx context.Context,
	query *repositories.Query,
	projectID, userID uuid.UUID,
) (*models.Project, error) {
	repo := query.Project

	project, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(projectID), repo.UserID.Eq(userID)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, commerr.ErrNotFound
		}

		return nil, ctxerrors.Wrap(err, "get project")
	}

	return project, nil
}
