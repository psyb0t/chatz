package operations

import (
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToAPI covers both projection shapes: a fully populated snapshot fills
// every optional field (commit, backup completion + driver, per-upstream
// latency/success/failure timestamps), and a bare snapshot leaves them nil.
func TestToAPI(t *testing.T) {
	t.Parallel()

	t.Run("full snapshot fills every optional field", func(t *testing.T) {
		t.Parallel()

		completedAt := time.Date(2026, time.August, 1, 3, 0, 0, 0, time.UTC)
		successAt := time.Date(2026, time.August, 1, 3, 1, 0, 0, time.UTC)
		failureAt := time.Date(2026, time.August, 1, 3, 2, 0, 0, time.UTC)

		got := ToAPI(Snapshot{
			AppVersion:     "1.2.3",
			Commit:         "abc123",
			DatabaseDriver: db.DriverPostgres,
			Migration:      db.MigrationStatus{Version: 7, Dirty: false},
			Backup: BackupStatus{
				State:       BackupStateCurrent,
				CompletedAt: completedAt,
				Driver:      db.DriverPostgres,
			},
			UpstreamHealths: []UpstreamHealth{{
				Upstream:           "openai",
				State:              UpstreamStateDegraded,
				LastOperation:      "stream",
				LastLatency:        250 * time.Millisecond,
				LastSuccessAt:      successAt,
				LastFailureAt:      failureAt,
				LastFailureClass:   "timeout",
				ConsecutiveFailure: 2,
			}},
		})

		assert.Equal(t, "1.2.3", got.AppVersion)
		require.NotNil(t, got.Commit)
		assert.Equal(t, "abc123", *got.Commit)
		require.NotNil(t, got.Backup.CompletedAt)
		require.NotNil(t, got.Backup.Driver)

		require.Len(t, got.Upstreams, 1)
		up := got.Upstreams[0]
		require.NotNil(t, up.LastLatencyMs)
		assert.Equal(t, int64(250), *up.LastLatencyMs)
		require.NotNil(t, up.LastSuccessAt)
		require.NotNil(t, up.LastFailureAt)
		require.NotNil(t, up.LastFailureClass)
	})

	t.Run("bare snapshot leaves optionals nil", func(t *testing.T) {
		t.Parallel()

		got := ToAPI(Snapshot{
			AppVersion:     "1.2.3",
			DatabaseDriver: db.DriverPostgres,
			Backup:         BackupStatus{State: BackupStateNotRecorded},
		})

		assert.Nil(t, got.Commit)
		assert.Nil(t, got.Backup.CompletedAt)
		assert.Nil(t, got.Backup.Driver)
		assert.Empty(t, got.Upstreams)
	})
}
