package main

import (
	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
)

// ChatQuerier holds chat queries the generated basic CRUD can't express:
// distinguishing chats that have no user message (the single
// reusable "empty chat" a user is pointed at by "New chat" — see
// core/chats.Service.GetOrCreateEmpty) from chats with real history.
type ChatQuerier interface {
	// FindEmptyChat returns the caller's oldest chat with zero user messages,
	// at most one row. An empty result slice means the user currently
	// has no empty chat.
	//
	// SELECT c.*
	// FROM chats c
	// WHERE c.user_id = @userID
	// AND c.deleted_at IS NULL
	// AND NOT EXISTS (
	//     SELECT 1 FROM messages m
	//     WHERE m.chat_id = c.id AND m.role = 'user'
	// )
	// ORDER BY c.created_at ASC
	// LIMIT 1
	FindEmptyChat(userID uuid.UUID) ([]*models.Chat, error)

	// ListNonEmpty returns a page of the user's chats that have at least one
	// user message, optionally narrowed by a literal title search, newest
	// activity first.
	//
	/*
		SELECT c.*
		FROM chats c
		WHERE c.user_id = @userID
		  AND c.deleted_at IS NULL
		  {{if search != ""}}
		  AND LOWER(c.title) LIKE LOWER(@search) ESCAPE '!'
		  {{end}}
		  AND EXISTS (
		      SELECT 1 FROM messages m
		      WHERE m.chat_id = c.id AND m.role = 'user'
		  )
		ORDER BY c.updated_at DESC
		{{if limit > 0}} LIMIT @limit {{end}}
		{{if offset > 0}} OFFSET @offset {{end}}
	*/
	ListNonEmpty(
		userID uuid.UUID,
		search string,
		limit, offset int,
	) ([]*models.Chat, error)

	// CountNonEmpty returns the total count matching ListNonEmpty's filter,
	// for the list endpoint's pagination envelope.
	//
	/*
		SELECT COUNT(*)
		FROM chats c
		WHERE c.user_id = @userID
		  AND c.deleted_at IS NULL
		  {{if search != ""}}
		  AND LOWER(c.title) LIKE LOWER(@search) ESCAPE '!'
		  {{end}}
		  AND EXISTS (
		      SELECT 1 FROM messages m
		      WHERE m.chat_id = c.id AND m.role = 'user'
		  )
	*/
	CountNonEmpty(
		userID uuid.UUID,
		search string,
	) (int64, error)
}
