package models

import "github.com/google/uuid"

// Project is a user-owned named chat grouping. It is deliberately not a
// workspace: a chat belongs to at most one project, and projects are never
// shared between users.
type Project struct {
	Base

	UserID uuid.UUID
	Name   string
}

// TableName pins the table name.
func (Project) TableName() string {
	return "projects"
}
