// Package models holds the plain Go structs that gorm/gen generates type-safe
// repositories from. One file per model; shared fields live in Base. The SQL
// migrations (internal/pkg/db/migrations, run by the DB connection layer) are
// the source of truth for the schema — these struct tags are only gorm
// decoration, not a second schema definition. Never hand-edit the generated
// repositories; change a model or migration and re-run `make generate`.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base is embedded by every model: a UUID id (primary key by gorm convention)
// plus timestamps and the soft-delete column.
type Base struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

// BeforeCreate mints a UUID when the id is unset, so inserts don't send the
// zero UUID (which collides on the primary key). Tag-free — the DB column keeps
// its gen_random_uuid() default as a fallback for non-gorm writers.
func (b *Base) BeforeCreate(*gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}

	return nil
}
