// Package testinfra brings up the real external dependencies chatz uses (a
// real Postgres via testcontainers-go) for integration tests. It runs the SAME
// connection + migration path as production (internal/pkg/db.Connect) so tests
// catch divergence between test setup and prod setup.
package testinfra

import (
	"context"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/common-go/db/postgresql"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Infra holds the running containers + connections for a test package. Call
// Teardown when done (typically from TestMain wrapping m.Run).
type Infra struct {
	PostgresContainer *tcpostgres.PostgresContainer
	Database          *db.Handle
	PG                *postgresql.Postgresql
}

// Setup brings up Postgres and runs chatz's migrations against it. Every
// error path tears down what was started so no container leaks.
func Setup(ctx context.Context) (*Infra, error) {
	infra := &Infra{}

	if err := infra.setupPostgres(ctx); err != nil {
		infra.Teardown(ctx)

		return nil, err
	}

	return infra, nil
}

// Teardown closes connections then terminates containers. Idempotent — safe to
// call from any failure path inside Setup. Cleanup errors are best-effort: the
// process is exiting, there is nothing to recover.
func (i *Infra) Teardown(ctx context.Context) {
	if i.Database != nil {
		_ = i.Database.Close()
	}

	if i.PostgresContainer != nil {
		_ = i.PostgresContainer.Terminate(ctx)
	}
}
