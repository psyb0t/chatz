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

	// ListNonEmpty returns a filtered page of the user's chats that have at
	// least one user message. Pinned chats sort first, then newest activity.
	//
	/*
		SELECT c.*
		FROM chats c
		WHERE c.user_id = @userID
		  AND c.deleted_at IS NULL
		  {{if archived}}
		  AND c.archived_at IS NOT NULL
		  {{else}}
		  AND c.archived_at IS NULL
		  {{end}}
		  {{if search != ""}}
		  AND LOWER(c.title) LIKE LOWER(@search) ESCAPE '!'
		  {{end}}
		  {{if projectID != nil}} AND c.project_id = @projectID {{end}}
		  AND EXISTS (
		      SELECT 1 FROM messages m
		      WHERE m.chat_id = c.id AND m.role = 'user'
		  )
		ORDER BY CASE WHEN c.pinned_at IS NULL THEN 1 ELSE 0 END,
		         c.pinned_at DESC,
		         c.updated_at DESC
		{{if limit > 0}} LIMIT @limit {{end}}
		{{if offset > 0}} OFFSET @offset {{end}}
	*/
	ListNonEmpty(
		userID uuid.UUID,
		archived bool,
		search string,
		projectID *uuid.UUID,
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
		  {{if archived}}
		  AND c.archived_at IS NOT NULL
		  {{else}}
		  AND c.archived_at IS NULL
		  {{end}}
		  {{if search != ""}}
		  AND LOWER(c.title) LIKE LOWER(@search) ESCAPE '!'
		  {{end}}
		  {{if projectID != nil}} AND c.project_id = @projectID {{end}}
		  AND EXISTS (
		      SELECT 1 FROM messages m
		      WHERE m.chat_id = c.id AND m.role = 'user'
		  )
	*/
	CountNonEmpty(
		userID uuid.UUID,
		archived bool,
		search string,
		projectID *uuid.UUID,
	) (int64, error)
}
