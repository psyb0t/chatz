package operations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	readinessTestVersion   = "test-version"
	readinessTestCommit    = "test-commit"
	readinessTestMaxAge    = time.Hour
	readinessTestMarker    = "backup-status.json"
	readinessTestTimestamp = "2026-08-12T16:00:00Z"
	//nolint:lll // compact JSON fixture
	readinessFreshPostgres = `{"completedAt":"2026-08-12T15:30:00Z","driver":"postgres"}`
	//nolint:lll // compact JSON fixture
	readinessStalePostgres = `{"completedAt":"2026-08-12T14:00:00Z","driver":"postgres"}`
	//nolint:lll // compact JSON fixture
	readinessFuturePostgres = `{"completedAt":"2026-08-12T16:00:01Z","driver":"postgres"}`
	//nolint:lll // compact JSON fixture
	readinessFreshSQLite = `{"completedAt":"2026-08-12T15:30:00Z","driver":"sqlite"}`
	//nolint:lll // compact JSON fixture
	readinessUnknownField = `{"completedAt":"2026-08-12T15:30:00Z","driver":"postgres","path":"secret"}`
)

type stubMigrationReader struct {
	status db.MigrationStatus
	err    error
}

func (s stubMigrationReader) MigrationStatus(
	_ context.Context,
) (db.MigrationStatus, error) {
	return s.status, s.err
}

func TestService_BackupStatus(t *testing.T) {
	t.Parallel()

	now, err := time.Parse(time.RFC3339, readinessTestTimestamp)
	require.NoError(t, err)

	testCases := []struct {
		name       string
		prepare    func(t *testing.T, markerPath string)
		wantState  BackupState
		wantDriver db.Driver
	}{
		{
			name:      "missing marker is not recorded",
			wantState: BackupStateNotRecorded,
		},
		{
			name: "fresh matching marker is current",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()
				writeMarker(t, markerPath, readinessFreshPostgres)
			},
			wantState:  BackupStateCurrent,
			wantDriver: db.DriverPostgres,
		},
		{
			name: "old marker is stale",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()
				writeMarker(t, markerPath, readinessStalePostgres)
			},
			wantState:  BackupStateStale,
			wantDriver: db.DriverPostgres,
		},
		{
			name: "unknown marker fields are invalid",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()
				writeMarker(t, markerPath, readinessUnknownField)
			},
			wantState: BackupStateInvalid,
		},
		{
			name: "future marker is invalid",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()
				writeMarker(t, markerPath, readinessFuturePostgres)
			},
			wantState: BackupStateInvalid,
		},
		{
			name: "other database marker is invalid",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()
				writeMarker(t, markerPath, readinessFreshSQLite)
			},
			wantState: BackupStateInvalid,
		},
		{
			name: "oversized marker is invalid",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()

				contents := make([]byte, maxBackupStatusBytes+1)
				require.NoError(t, os.WriteFile(markerPath, contents, 0o600))
			},
			wantState: BackupStateInvalid,
		},
		{
			name: "symlink marker is unavailable",
			prepare: func(t *testing.T, markerPath string) {
				t.Helper()

				target := filepath.Join(filepath.Dir(markerPath), "target.json")
				writeMarker(t, target, readinessFreshPostgres)
				require.NoError(t, os.Symlink(target, markerPath))
			},
			wantState: BackupStateUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()

			markerPath := filepath.Join(directory, readinessTestMarker)
			if tc.prepare != nil {
				tc.prepare(t, markerPath)
			}

			service := newTestService(t, markerPath, directory)
			service.now = func() time.Time { return now }

			status := service.backupStatus()
			assert.Equal(t, tc.wantState, status.State)
			assert.Equal(t, tc.wantDriver, status.Driver)
		})
	}
}

func TestNew_ValidatesConfiguration(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	validConfig := Config{
		AppVersion:       readinessTestVersion,
		DatabaseDriver:   db.DriverPostgres,
		BackupStatusPath: filepath.Join(directory, readinessTestMarker),
		BackupMaxAge:     readinessTestMaxAge,
		dataDirectory:    directory,
	}

	testCases := []struct {
		name     string
		database MigrationReader
		config   Config
		wantErr  error
	}{
		{
			name:    "missing database",
			config:  validConfig,
			wantErr: commerr.ErrRequiredFieldNotSet,
		},
		{
			name:     "missing app version",
			database: stubMigrationReader{},
			config: Config{
				BackupMaxAge:  readinessTestMaxAge,
				dataDirectory: directory,
			},
			wantErr: commerr.ErrRequiredFieldNotSet,
		},
		{
			name:     "nonpositive backup age",
			database: stubMigrationReader{},
			config: Config{
				AppVersion:       readinessTestVersion,
				BackupStatusPath: filepath.Join(directory, readinessTestMarker),
				dataDirectory:    directory,
			},
			wantErr: commerr.ErrInvalidArgument,
		},
		{
			name:     "marker outside data directory",
			database: stubMigrationReader{},
			config: Config{
				AppVersion: readinessTestVersion,
				BackupStatusPath: filepath.Join(
					directory,
					"nested",
					readinessTestMarker,
				),
				BackupMaxAge:  readinessTestMaxAge,
				dataDirectory: directory,
			},
			wantErr: commerr.ErrInvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(tc.database, nil, tc.config)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestService_Snapshot(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	service := newTestService(
		t,
		filepath.Join(directory, readinessTestMarker),
		directory,
	)

	snapshot, err := service.Snapshot(t.Context())
	require.NoError(t, err)
	assert.Equal(t, readinessTestVersion, snapshot.AppVersion)
	assert.Equal(t, readinessTestCommit, snapshot.Commit)
	assert.Equal(t, db.DriverPostgres, snapshot.DatabaseDriver)
	assert.Equal(t, db.MigrationStatus{Version: 42}, snapshot.Migration)
	assert.Equal(t, BackupStateNotRecorded, snapshot.Backup.State)
	assert.Empty(t, snapshot.UpstreamHealths)
}

func newTestService(
	t *testing.T,
	markerPath string,
	directory string,
) *Service {
	t.Helper()

	service, err := New(stubMigrationReader{
		status: db.MigrationStatus{Version: 42},
	}, nil, Config{
		AppVersion:       readinessTestVersion,
		Commit:           readinessTestCommit,
		DatabaseDriver:   db.DriverPostgres,
		BackupStatusPath: markerPath,
		BackupMaxAge:     readinessTestMaxAge,
		dataDirectory:    directory,
	})
	require.NoError(t, err)

	return service
}

func writeMarker(t *testing.T, path string, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}
