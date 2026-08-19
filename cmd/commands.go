package main

import (
	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
	"github.com/spf13/cobra"
)

// This file is yours - it never gets replaced by framework
// updates. Return your custom CLI commands here.
func commands() []*cobra.Command {
	return []*cobra.Command{
		migrateCommand(),
	}
}

// migrateCommand applies pending migrations and exits without serving. The
// HTTP service already migrates on boot, so this exists for the operator who
// needs the schema moved on its own — a deploy step that runs before the new
// binary takes traffic, or a recovery after a failed rollout. `make migrate`
// invokes it.
func migrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending database migrations and exit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMigrate(cmd)
		},
	}
}

func runMigrate(cmd *cobra.Command) error {
	ctx := cmd.Context()
	logger := ctxscope.GetLogger(ctx)

	cfg, err := config.Parse()
	if err != nil {
		return ctxerrors.Wrap(err, "parse config")
	}

	// Connect is what runs the migrations for the configured driver; the db
	// package exposes no migrate-only entry point.
	database, err := db.Connect(ctx, cfg.DBConfig())
	if err != nil {
		return ctxerrors.Wrap(err, "connect db")
	}

	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("close db failed", "err", closeErr)
		}
	}()

	status, err := database.MigrationStatus(ctx)
	if err != nil {
		return ctxerrors.Wrap(err, "read migration status")
	}

	logger.Info("migrations applied",
		"db_driver", cfg.DBDriver,
		"migration_version", status.Version,
		"migration_dirty", status.Dirty,
	)

	return nil
}
