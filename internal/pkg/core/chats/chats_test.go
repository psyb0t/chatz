package chats

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A blank title is rejected before any DB access, so a zero-value Service (nil
// query) is enough to exercise the guard.
func TestRenameRejectsBlankTitle(t *testing.T) {
	t.Parallel()

	s := &Service{}

	for _, title := range []string{"", "   ", "\t\n"} {
		_, err := s.Rename(context.Background(), uuid.New(), uuid.New(), title)
		require.ErrorIs(t, err, ErrEmptyTitle)
	}
}
