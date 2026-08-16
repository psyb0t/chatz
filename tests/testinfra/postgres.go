package testinfra

import (
	"context"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/ctxerrors"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	pgImage     = "postgres:16-alpine"
	pgUser      = "test"
	pgPassword  = "test"
	pgDatabase  = "chatz"
	pgPort      = "5432"
	pgReadyLog  = "database system is ready to accept connections"
	pgStartWait = 60 * time.Second
)

// setupPostgres runs a real Postgres container, then connects + migrates
// through the production db.Connect path so tests exercise the same wiring.
func (i *Infra) setupPostgres(ctx context.Context) error {
	container, err := tcpostgres.Run(ctx, pgImage,
		tcpostgres.WithDatabase(pgDatabase),
		tcpostgres.WithUsername(pgUser),
		tcpostgres.WithPassword(pgPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog(pgReadyLog).
				WithOccurrence(2). //nolint:mnd // pg logs the ready line twice
				WithStartupTimeout(pgStartWait),
		),
	)
	if err != nil {
		return ctxerrors.Wrap(err, "run postgres container")
	}

	i.PostgresContainer = container

	host, err := container.Host(ctx)
	if err != nil {
		return ctxerrors.Wrap(err, "container host")
	}

	mapped, err := container.MappedPort(ctx, pgPort)
	if err != nil {
		return ctxerrors.Wrap(err, "container port")
	}

	database, err := db.Connect(ctx, db.Config{
		Driver:   db.DriverPostgres,
		Hostname: host,
		Port:     int(mapped.Num()),
		Username: pgUser,
		Password: pgPassword,
		Database: pgDatabase,
		IsSSL:    false,
	})
	if err != nil {
		return ctxerrors.Wrap(err, "connect + migrate")
	}

	i.Database = database
	i.PG = database.PostgreSQL()

	return nil
}
