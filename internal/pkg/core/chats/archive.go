package chats

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/ctxerrors"
)

// SetArchived moves a chat into or out of the caller's archive. It retains the
// chat, messages, pin, and project assignment.
func (s *Service) SetArchived(
	ctx context.Context,
	chatID, userID uuid.UUID,
	archived bool,
) (*models.Chat, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	var archivedAt *time.Time

	if archived {
		now := time.Now().UTC()
		archivedAt = &now
	}

	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chat.ID)).
		Update(repo.ArchivedAt, archivedAt); err != nil {
		return nil, ctxerrors.Wrap(err, "set chat archive state")
	}

	chat.ArchivedAt = archivedAt

	return chat, nil
}
