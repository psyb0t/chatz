package chats

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/psyb0t/ctxerrors"
)

type chatTurnLock struct {
	token chan struct{}
	refs  int
}

type chatTurnLockRegistry struct {
	mutex sync.Mutex
	locks map[uuid.UUID]*chatTurnLock
}

//nolint:gochecknoglobals // Shared by all Service instances.
var processTurnLocks = chatTurnLockRegistry{
	locks: make(map[uuid.UUID]*chatTurnLock),
}

func lockChatTurn(
	ctx context.Context,
	chatID uuid.UUID,
) (func(), error) {
	processTurnLocks.mutex.Lock()

	lock := processTurnLocks.locks[chatID]
	if lock == nil {
		lock = &chatTurnLock{token: make(chan struct{}, 1)}
		lock.token <- struct{}{}

		processTurnLocks.locks[chatID] = lock
	}

	lock.refs++

	processTurnLocks.mutex.Unlock()

	select {
	case <-ctx.Done():
		releaseChatTurnRef(chatID, lock)

		return nil, ctxerrors.Wrap(ctx.Err(), "wait for chat turn lock")
	case <-lock.token:
	}

	if err := ctx.Err(); err != nil {
		lock.token <- struct{}{}

		releaseChatTurnRef(chatID, lock)

		return nil, ctxerrors.Wrap(err, "wait for chat turn lock")
	}

	var once sync.Once

	return func() {
		once.Do(func() {
			lock.token <- struct{}{}

			releaseChatTurnRef(chatID, lock)
		})
	}, nil
}

func releaseChatTurnRef(chatID uuid.UUID, lock *chatTurnLock) {
	processTurnLocks.mutex.Lock()
	defer processTurnLocks.mutex.Unlock()

	lock.refs--
	if lock.refs == 0 {
		delete(processTurnLocks.locks, chatID)
	}
}

func releaseUntransferredChatTurn(unlock *func()) {
	if *unlock != nil {
		(*unlock)()
	}
}
