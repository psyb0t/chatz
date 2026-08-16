// Package db opens the selected persistent store, runs its embedded migrations,
// and wires the generated gorm/gen repositories' default query. Business code
// uses the typed repositories (internal/pkg/db/repositories) — never raw SQL.
package db

import (
	"context"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db/migrations"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/common-go/db/postgresql"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
	"gorm.io/gorm"
)

// Driver identifies the persistence implementation selected for one Chatz
// process.
type Driver string

const (
	// DriverPostgres is the shared and scale-out persistence mode.
	DriverPostgres Driver = "postgres"

	// DriverSQLite is the single-process, local-volume persistence mode.
	DriverSQLite Driver = "sqlite"
)

// Config is the database connection config, mapped from the app config so the
// db package doesn't depend on the whole config struct.
type Config struct {
	Driver   Driver
	Hostname string
	Port     int
	Username string
	Password string
	Database string
	IsSSL    bool

	SQLitePath        string
	SQLiteBusyTimeout time.Duration

	// sqliteDataDirectory is a test seam. Production always uses /data, the
	// explicit writable mount in the otherwise read-only image.
	sqliteDataDirectory string
}

// Handle is an open database connection owned by its caller. The GORM handle
// backs the generated repositories; Close releases the underlying SQL pool.
type Handle struct {
	GormDB *gorm.DB
	Driver Driver

	close    func() error
	postgres *postgresql.Postgresql
}

// Close releases the database resources owned by the handle.
func (h *Handle) Close() error {
	if h == nil || h.close == nil {
		return nil
	}

	if err := h.close(); err != nil {
		return ctxerrors.Wrap(err, "close database")
	}

	return nil
}

// PostgreSQL returns the native connection for PostgreSQL-only maintenance
// operations, such as the migration reversibility integration test. It is nil
// for the SQLite driver; application code should use GormDB + repositories.
func (h *Handle) PostgreSQL() *postgresql.Postgresql {
	if h == nil {
		return nil
	}

	return h.postgres
}

// Connect opens the configured database, runs its migrations, and sets the
// generated repositories' default query. The returned handle is owned by the
// caller — Close it on shutdown.
func Connect(ctx context.Context, cfg Config) (*Handle, error) {
	switch cfg.Driver {
	case DriverPostgres:
		return connectPostgres(ctx, cfg)
	case DriverSQLite:
		return connectSQLite(ctx, cfg)
	default:
		return nil, ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"CHATZ_DB_DRIVER must be postgres or sqlite, got %q",
			cfg.Driver,
		)
	}
}

func connectPostgres(ctx context.Context, cfg Config) (*Handle, error) {
	logger := ctxscope.GetLogger(ctx)

	pg, err := postgresql.NewWithConfig(ctx, postgresql.Config{
		Hostname: cfg.Hostname,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		Database: cfg.Database,
		IsSSL:    cfg.IsSSL,
	}, true)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "connect postgres")
	}

	logger.Info("running db migrations")

	// "." = the embed FS root, where the *.sql files live (iofs rejects "").
	if err := pg.MigrateUp(".", &migrations.FS); err != nil {
		return nil, ctxerrors.Wrap(err, "migrate postgres up")
	}

	repositories.SetDefault(pg.GormDB)
	logger.Info("db ready", "database", cfg.Database)

	return &Handle{
		GormDB:   pg.GormDB,
		Driver:   DriverPostgres,
		close:    pg.Close,
		postgres: pg,
	}, nil
}
