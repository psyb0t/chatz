package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/psyb0t/chatz/internal/pkg/db/migrations"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	commondb "github.com/psyb0t/common-go/db"
	commonsqlite "github.com/psyb0t/common-go/db/sqlite"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
	"gorm.io/gorm"
)

const (
	defaultSQLiteDataDirectory = "/data"
	sqliteMigrationsPath       = "sqlite"
)

func connectSQLite(ctx context.Context, cfg Config) (*Handle, error) {
	if err := validateSQLiteConfig(cfg); err != nil {
		return nil, ctxerrors.Wrap(err, "validate sqlite config")
	}

	database, err := gorm.Open(sqlite.Open(sqliteDSN(cfg)), &gorm.Config{
		Logger: commondb.NewGormSlogLogger(),
	})
	if err != nil {
		return nil, ctxerrors.Wrap(err, "open sqlite")
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, ctxerrors.Wrap(err, "get sqlite sql db")
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, ctxerrors.Wrap(err, "ping sqlite")
	}

	if err := commonsqlite.MigrateUp(
		sqlDB,
		sqliteMigrationsPath,
		&migrations.SQLiteFS,
	); err != nil {
		return nil, ctxerrors.Wrap(err, "migrate sqlite")
	}

	repositories.SetDefault(database)
	ctxscope.GetLogger(ctx).Info(
		"db ready",
		"driver", DriverSQLite,
		"database", cfg.SQLitePath,
	)

	return &Handle{
		GormDB: database,
		Driver: DriverSQLite,
		close:  sqlDB.Close,
	}, nil
}

func validateSQLiteConfig(cfg Config) error {
	if cfg.SQLiteBusyTimeout <= 0 {
		return ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"CHATZ_DB_SQLITE_BUSY_TIMEOUT must be positive, got %s",
			cfg.SQLiteBusyTimeout,
		)
	}

	dataDirectory, err := validateSQLiteDataDirectory(cfg)
	if err != nil {
		return err
	}

	return validateSQLiteDatabasePath(cfg, dataDirectory)
}

func validateSQLiteDataDirectory(cfg Config) (string, error) {
	dataDirectory := cfg.sqliteDataDirectoryPath()
	if !filepath.IsAbs(dataDirectory) {
		return "", ctxerrors.Wrapf(
			commerr.ErrInvalidState,
			"sqlite data directory must be absolute, got %q",
			dataDirectory,
		)
	}

	dataDirectoryInfo, err := os.Lstat(dataDirectory)
	if err != nil {
		return "", ctxerrors.Wrapf(
			err,
			"inspect sqlite data directory %q",
			dataDirectory,
		)
	}

	if !dataDirectoryInfo.IsDir() ||
		dataDirectoryInfo.Mode()&os.ModeSymlink != 0 {
		return "", ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"sqlite data directory must be a real directory, got %q",
			dataDirectory,
		)
	}

	if dataDirectoryInfo.Mode().Perm()&0o222 == 0 {
		return "", ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"sqlite data directory is not writable: %q",
			dataDirectory,
		)
	}

	return dataDirectory, nil
}

func validateSQLiteDatabasePath(cfg Config, dataDirectory string) error {
	if !filepath.IsAbs(cfg.SQLitePath) ||
		filepath.Clean(filepath.Dir(cfg.SQLitePath)) !=
			filepath.Clean(dataDirectory) ||
		strings.ContainsAny(cfg.SQLitePath, "?#") {
		return ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			//nolint:lll // one logical error message; splitting changes no semantics but harms readability
			"CHATZ_DB_SQLITE_PATH must name a regular file directly under %q, got %q",
			dataDirectory,
			cfg.SQLitePath,
		)
	}

	fileInfo, err := os.Lstat(cfg.SQLitePath)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return ctxerrors.Wrapf(
			err,
			"inspect sqlite database %q",
			cfg.SQLitePath,
		)
	}

	if !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 {
		return ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"CHATZ_DB_SQLITE_PATH must be a regular file, got %q",
			cfg.SQLitePath,
		)
	}

	return nil
}

func (cfg Config) sqliteDataDirectoryPath() string {
	if cfg.sqliteDataDirectory != "" {
		return cfg.sqliteDataDirectory
	}

	return defaultSQLiteDataDirectory
}

func sqliteDSN(cfg Config) string {
	return fmt.Sprintf(
		//nolint:lll // DSN query parameters are one indivisible connection string
		"%s?_pragma=busy_timeout(%d)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
		cfg.SQLitePath,
		cfg.SQLiteBusyTimeout.Milliseconds(),
	)
}
