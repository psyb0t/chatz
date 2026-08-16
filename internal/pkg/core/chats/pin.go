package chats

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/ctxerrors"
)

// SetPinned pins or unpins a chat for the caller. Pinned chats lead their
// active or archived list while retaining activity order within each group.
func (s *Service) SetPinned(
	ctx context.Context,
	chatID, userID uuid.UUID,
	pinned bool,
) (*models.Chat, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}

	var pinnedAt *time.Time

	if pinned {
		now := time.Now().UTC()
		pinnedAt = &now
	}

	repo := s.query.Chat
	if _, err := repo.WithContext(ctx).
		Where(repo.ID.Eq(chat.ID)).
		Update(repo.PinnedAt, pinnedAt); err != nil {
		return nil, ctxerrors.Wrap(err, "set chat pin state")
	}

	chat.PinnedAt = pinnedAt

	return chat, nil
}
