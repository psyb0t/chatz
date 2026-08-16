package chats

import "errors"

var (
	// ErrUnknownModel is returned when a real (non-demo) turn names a model no
	// configured upstream serves. Callers map it to a 400.
	ErrUnknownModel = errors.New("chats: unknown model")

	// ErrEmptyTitle is returned when a rename is given a blank title. Callers
	// map it to a 400.
	ErrEmptyTitle = errors.New("chats: title must not be empty")
)
