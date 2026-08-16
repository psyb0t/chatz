package chats

import (
	"context"

	"github.com/google/uuid"
	"github.com/psyb0t/ctxerrors"
)

// Delete removes a caller-owned chat from normal reads through the existing
// soft-delete model. The durable message rows remain outside normal history.
func (s *Service) Delete(
	ctx context.Context,
	chatID, userID uuid.UUID,
) error {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return err
	}

	if _, err := s.query.Chat.WithContext(ctx).Delete(chat); err != nil {
		return ctxerrors.Wrap(err, "delete chat")
	}

	return nil
}
