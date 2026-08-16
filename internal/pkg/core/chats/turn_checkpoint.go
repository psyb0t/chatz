package chats

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
	"gorm.io/gorm"
)

const assistantCheckpointInterval = 750 * time.Millisecond

// turnCheckpoint retains the text and reasoning received for an active turn.
// It persists at most once per interval; the final transcript replaces this
// incomplete row atomically when the turn completes.
type turnCheckpoint struct {
	service *Service
	chatID  uuid.UUID
	turnID  uuid.UUID
	modelID string

	content   string
	reasoning string
	dirty     bool
	lastTry   time.Time
}

func newTurnCheckpoint(
	service *Service,
	chatID, turnID uuid.UUID,
	modelID string,
) *turnCheckpoint {
	return &turnCheckpoint{
		service: service,
		chatID:  chatID,
		turnID:  turnID,
		modelID: modelID,
	}
}

// add records an assistant delta in memory and periodically writes the latest
// complete prefix. Checkpoint failures are deliberately non-fatal: failing to
// save recovery state must never stop an otherwise healthy streamed response.
func (checkpoint *turnCheckpoint) add(ctx context.Context, delta elelem.Delta) {
	if !checkpoint.append(delta) {
		return
	}

	if time.Since(checkpoint.lastTry) < assistantCheckpointInterval {
		return
	}

	checkpoint.save(ctx)
}

func (checkpoint *turnCheckpoint) append(delta elelem.Delta) bool {
	if delta.Text == "" && delta.Reasoning == "" {
		return false
	}

	checkpoint.content += delta.Text
	checkpoint.reasoning += delta.Reasoning
	checkpoint.dirty = true

	return true
}

// flush persists any prefix received since the most recent checkpoint. It is
// called before the final transcript transaction, so a process interruption in
// the small window after the final delta still leaves recoverable output.
func (checkpoint *turnCheckpoint) flush(ctx context.Context) {
	if !checkpoint.dirty {
		return
	}

	checkpoint.save(ctx)
}

func (checkpoint *turnCheckpoint) save(ctx context.Context) {
	checkpoint.lastTry = time.Now()

	persistCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		persistTimeout,
	)
	defer cancel()

	if err := checkpoint.upsert(persistCtx); err != nil {
		ctxscope.GetLogger(ctx).Warn(
			"persist assistant checkpoint failed",
			"chat_id", checkpoint.chatID,
			"turn_id", checkpoint.turnID,
			"reason", "assistant_checkpoint_persist_failed",
			"err", err,
		)

		return
	}

	checkpoint.dirty = false
}

func (checkpoint *turnCheckpoint) upsert(ctx context.Context) error {
	repo := checkpoint.service.query.Message
	turnComplete := false

	row, err := repo.WithContext(ctx).
		Where(
			repo.ChatID.Eq(checkpoint.chatID),
			repo.TurnID.Eq(checkpoint.turnID),
			repo.Role.Eq(models.MessageRoleAssistant),
			repo.TurnComplete.Is(false),
		).
		First()
	if err == nil {
		row.Content = checkpoint.content
		row.Reasoning = checkpoint.reasoning

		if err := repo.WithContext(ctx).Save(row); err != nil {
			return ctxerrors.Wrap(err, "update assistant checkpoint")
		}

		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctxerrors.Wrap(err, "find assistant checkpoint")
	}

	row = &models.Message{
		ChatID:       checkpoint.chatID,
		TurnID:       checkpoint.turnID,
		TurnComplete: &turnComplete,
		ModelID:      checkpoint.modelID,
		Role:         models.MessageRoleAssistant,
		Content:      checkpoint.content,
		Reasoning:    checkpoint.reasoning,
	}
	if err := repo.WithContext(ctx).Create(row); err != nil {
		return ctxerrors.Wrap(err, "create assistant checkpoint")
	}

	return nil
}
