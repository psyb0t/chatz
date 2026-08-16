package models

import (
	"time"

	"github.com/google/uuid"
)

// Session is a server-side login session. Only the hash of the opaque token is
// stored; the raw token lives in the client's HttpOnly cookie.
type Session struct {
	Base

	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
}

// TableName pins the table name.
func (Session) TableName() string {
	return "sessions"
}
