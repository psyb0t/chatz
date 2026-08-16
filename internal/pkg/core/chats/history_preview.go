package chats

import (
	"context"

	"github.com/google/uuid"
	"github.com/psyb0t/ctxerrors"
)

// PreviewContext returns the exact prompt selection a turn would use for the
// caller's unsent message. It shares the production history loader, sticky
// system prompt, tiktoken counting, and complete-tool-turn selection logic.
func (s *Service) PreviewContext(
	ctx context.Context,
	chatID, userID uuid.UUID,
	message string,
) (PromptContext, error) {
	chat, err := s.ownedChat(ctx, chatID, userID)
	if err != nil {
		return PromptContext{}, err
	}

	history, err := s.loadHistory(ctx, chat)
	if err != nil {
		return PromptContext{}, ctxerrors.Wrap(err, "load chat history")
	}

	preview, err := selectPromptHistory(
		chatSystemPrompt(),
		history,
		message,
		historyTokenBudget(chat),
	)
	if err != nil {
		return PromptContext{}, ctxerrors.Wrap(err, "select preview history")
	}

	return preview, nil
}
