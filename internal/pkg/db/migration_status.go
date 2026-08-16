package db

import (
	"context"
	"errors"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"gorm.io/gorm"
)

// MigrationStatus is the database's applied schema position.
type MigrationStatus struct {
	Version int
	Dirty   bool
}

const migrationTable = "schema_migrations"

// MigrationStatus reads the actual migration ledger after startup migrations
// have completed. It is diagnostics-only and does not mutate the database.
func (h *Handle) MigrationStatus(ctx context.Context) (MigrationStatus, error) {
	if h == nil || h.GormDB == nil {
		return MigrationStatus{}, ctxerrors.Wrap(
			commerr.ErrRequiredFieldNotSet,
			"read database migration status",
		)
	}

	switch h.Driver {
	case DriverPostgres, DriverSQLite:
		return migrationStatus(ctx, h.GormDB)
	default:
		return MigrationStatus{}, ctxerrors.Wrapf(
			commerr.ErrInvalidState,
			"read database migration status for unsupported driver %q",
			h.Driver,
		)
	}
}

func migrationStatus(
	ctx context.Context,
	database *gorm.DB,
) (MigrationStatus, error) {
	var row MigrationStatus

	err := database.WithContext(ctx).
		Table(migrationTable).
		Select("version", "dirty").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MigrationStatus{}, nil
	}

	if err != nil {
		return MigrationStatus{}, ctxerrors.Wrap(
			err,
			"query migration status",
		)
	}

	return row, nil
}
