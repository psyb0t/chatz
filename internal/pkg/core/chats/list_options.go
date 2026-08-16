package chats

import "github.com/google/uuid"

// ListOptions narrows one caller's chat history without changing ownership.
// Archived selects archived rather than active chats; ProjectID limits the
// result to one project when non-nil.
type ListOptions struct {
	Archived  bool
	Search    string
	ProjectID *uuid.UUID
}
