package chats

import (
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
)

func TestTurnCheckpoint_Append(t *testing.T) {
	t.Parallel()

	checkpoint := newTurnCheckpoint(
		nil,
		uuid.New(),
		uuid.New(),
		"model",
	)

	assert.False(t, checkpoint.append(elelem.Delta{}))
	assert.False(t, checkpoint.dirty)
	assert.Empty(t, checkpoint.content)
	assert.Empty(t, checkpoint.reasoning)

	assert.True(t, checkpoint.append(elelem.Delta{Text: "first "}))
	assert.True(t, checkpoint.append(elelem.Delta{Reasoning: "considering"}))
	assert.True(t, checkpoint.append(elelem.Delta{Text: "answer"}))
	assert.True(t, checkpoint.dirty)
	assert.Equal(t, "first answer", checkpoint.content)
	assert.Equal(t, "considering", checkpoint.reasoning)
}
