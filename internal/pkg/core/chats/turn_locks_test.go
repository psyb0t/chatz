package chats

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const lockTestTimeout = time.Second

type testLockResult struct {
	unlock func()
	err    error
}

// TestReleaseUntransferredChatTurn covers both branches: a non-nil unlock is
// invoked, and a nil unlock is a safe no-op (the turn was already transferred
// to the streamer, which owns the release).
func TestReleaseUntransferredChatTurn(t *testing.T) {
	t.Parallel()

	called := false
	unlock := func() { called = true }
	releaseUntransferredChatTurn(&unlock)
	require.True(t, called)

	var transferred func()

	require.NotPanics(t, func() { releaseUntransferredChatTurn(&transferred) })
}

func TestLockChatTurnSerializesSameChat(t *testing.T) {
	t.Parallel()

	chatID := uuid.New()
	unlockFirst, err := lockChatTurn(t.Context(), chatID)
	require.NoError(t, err)
	t.Cleanup(unlockFirst)

	secondAcquired := make(chan testLockResult, 1)

	go func() {
		unlock, lockErr := lockChatTurn(t.Context(), chatID)
		secondAcquired <- testLockResult{unlock: unlock, err: lockErr}
	}()

	waitCtx, cancelWait := context.WithTimeout(
		t.Context(),
		lockTestTimeout,
	)
	t.Cleanup(cancelWait)
	require.True(t, waitForChatTurnLockRefs(waitCtx, chatID, 2))

	select {
	case result := <-secondAcquired:
		if result.unlock != nil {
			result.unlock()
		}

		require.Fail(t, "second same-chat lock acquired before first unlock")
	default:
	}

	unlockFirst()

	var secondResult testLockResult
	select {
	case secondResult = <-secondAcquired:
	case <-waitCtx.Done():
		require.NoError(t, waitCtx.Err())
	}

	require.NoError(t, secondResult.err)
	require.NotNil(t, secondResult.unlock)
	secondResult.unlock()
	secondResult.unlock()
	require.True(t, waitForChatTurnLockRefs(waitCtx, chatID, 0))
}

func TestLockChatTurnCanceledWaiterCleansRegistryRef(t *testing.T) {
	t.Parallel()

	chatID := uuid.New()
	unlockFirst, err := lockChatTurn(t.Context(), chatID)
	require.NoError(t, err)
	t.Cleanup(unlockFirst)

	waiterCtx, cancelWaiter := context.WithCancel(t.Context())
	waiterResult := make(chan testLockResult, 1)

	go func() {
		unlock, lockErr := lockChatTurn(waiterCtx, chatID)
		waiterResult <- testLockResult{unlock: unlock, err: lockErr}
	}()

	waitCtx, cancelWait := context.WithTimeout(
		t.Context(),
		lockTestTimeout,
	)
	t.Cleanup(cancelWait)
	require.True(t, waitForChatTurnLockRefs(waitCtx, chatID, 2))

	cancelWaiter()

	var result testLockResult
	select {
	case result = <-waiterResult:
	case <-waitCtx.Done():
		require.NoError(t, waitCtx.Err())
	}

	require.ErrorIs(t, result.err, context.Canceled)
	require.Nil(t, result.unlock)
	require.True(t, waitForChatTurnLockRefs(waitCtx, chatID, 1))

	unlockFirst()
	require.True(t, waitForChatTurnLockRefs(waitCtx, chatID, 0))
}

func TestLockChatTurnDoesNotBlockDifferentChats(t *testing.T) {
	t.Parallel()

	unlockFirst, err := lockChatTurn(t.Context(), uuid.New())
	require.NoError(t, err)
	t.Cleanup(unlockFirst)

	secondCtx, cancelSecond := context.WithTimeout(
		t.Context(),
		lockTestTimeout,
	)
	t.Cleanup(cancelSecond)

	unlockSecond, err := lockChatTurn(secondCtx, uuid.New())
	require.NoError(t, err)
	require.NotNil(t, unlockSecond)
	unlockSecond()
}

func waitForChatTurnLockRefs(
	ctx context.Context,
	chatID uuid.UUID,
	want int,
) bool {
	for {
		processTurnLocks.mutex.Lock()
		lock, exists := processTurnLocks.locks[chatID]

		refs := 0
		if exists {
			refs = lock.refs
		}

		processTurnLocks.mutex.Unlock()

		if refs == want {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		default:
			runtime.Gosched()
		}
	}
}
