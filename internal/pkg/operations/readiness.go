// Package operations provides small operator-facing snapshots assembled from
// Chatz's already-running collaborators.
package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

const (
	dataDirectory              = "/data"
	maxBackupStatusBytes       = 4 * 1024
	backupStatusDriverPostgres = db.DriverPostgres
	backupStatusDriverSQLite   = db.DriverSQLite
)

// BackupState explains whether an operator-provided backup marker proves a
// recent successful backup. It intentionally says nothing about archive
// contents: creation and integrity verification belong to the backup job.
type BackupState string

const (
	BackupStateCurrent     BackupState = "current"
	BackupStateNotRecorded BackupState = "not_recorded"
	BackupStateStale       BackupState = "stale"
	BackupStateInvalid     BackupState = "invalid"
	BackupStateUnavailable BackupState = "unavailable"
)

// BackupStatus is the safe subset of a backup marker shown to administrators.
type BackupStatus struct {
	State       BackupState
	CompletedAt time.Time
	Driver      db.Driver
}

// Snapshot is the complete operational state rendered by the admin readiness
// page. It contains no credentials, file paths, prompt content, or provider
// response text.
type Snapshot struct {
	AppVersion      string
	Commit          string
	DatabaseDriver  db.Driver
	Migration       db.MigrationStatus
	Backup          BackupStatus
	UpstreamHealths []UpstreamHealth
}

// MigrationReader lets readiness use the live DB handle while retaining a
// narrow, deterministic test seam.
type MigrationReader interface {
	MigrationStatus(context.Context) (db.MigrationStatus, error)
}

// UpstreamHealthReader supplies a redacted, point-in-time upstream health
// snapshot without coupling readiness to the upstream configuration package.
type UpstreamHealthReader interface {
	ReadinessHealth() []UpstreamHealth
}

// UpstreamHealth is the operational subset of an upstream's last outcome.
type UpstreamHealth struct {
	Upstream           string
	State              UpstreamState
	LastOperation      string
	LastLatency        time.Duration
	LastSuccessAt      time.Time
	LastFailureAt      time.Time
	LastFailureClass   string
	ConsecutiveFailure int
}

// UpstreamState is the current client-visible condition of an upstream.
type UpstreamState string

const (
	UpstreamStateUnknown  UpstreamState = "unknown"
	UpstreamStateHealthy  UpstreamState = "healthy"
	UpstreamStateDegraded UpstreamState = "degraded"
)

// Config selects the identity and local backup-marker policy for one process.
type Config struct {
	AppVersion       string
	Commit           string
	DatabaseDriver   db.Driver
	BackupStatusPath string
	BackupMaxAge     time.Duration

	dataDirectory string
}

// Service reads non-secret operational facts from the live collaborators.
type Service struct {
	database MigrationReader
	registry UpstreamHealthReader
	config   Config
	now      func() time.Time
}

// New validates the static readiness configuration before serving requests.
func New(
	database MigrationReader,
	registry UpstreamHealthReader,
	config Config,
) (*Service, error) {
	if database == nil {
		return nil, ctxerrors.Wrap(
			commerr.ErrRequiredFieldNotSet,
			"create readiness service without database",
		)
	}

	if config.AppVersion == "" {
		return nil, ctxerrors.Wrap(
			commerr.ErrRequiredFieldNotSet,
			"create readiness service without app version",
		)
	}

	if config.BackupMaxAge <= 0 {
		return nil, ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"CHATZ_BACKUP_MAX_AGE must be positive, got %s",
			config.BackupMaxAge,
		)
	}

	if err := validateBackupStatusPath(config); err != nil {
		return nil, ctxerrors.Wrap(err, "validate backup status path")
	}

	return &Service{
		database: database,
		registry: registry,
		config:   config,
		now:      time.Now,
	}, nil
}

// Snapshot reads the actual database ledger, current upstream tracker, and
// backup marker. A bad or missing marker is represented in BackupState rather
// than failing the whole operational page.
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	migration, err := s.database.MigrationStatus(ctx)
	if err != nil {
		return Snapshot{}, ctxerrors.Wrap(err, "read migration status")
	}

	snapshot := Snapshot{
		AppVersion:     s.config.AppVersion,
		Commit:         s.config.Commit,
		DatabaseDriver: s.config.DatabaseDriver,
		Migration:      migration,
		Backup:         s.backupStatus(),
	}
	if s.registry != nil {
		snapshot.UpstreamHealths = s.registry.ReadinessHealth()
	}

	return snapshot, nil
}

func (s *Service) backupStatus() BackupStatus {
	contents, state := s.readBackupMarker()
	if state != "" {
		return BackupStatus{State: state}
	}

	marker, err := parseBackupMarker(contents)
	if err != nil || !marker.valid(s.now().UTC(), s.config.DatabaseDriver) {
		return BackupStatus{State: BackupStateInvalid}
	}

	status := BackupStatus{
		State:       BackupStateCurrent,
		CompletedAt: marker.CompletedAt,
		Driver:      marker.Driver,
	}
	if s.now().UTC().Sub(marker.CompletedAt) > s.config.BackupMaxAge {
		status.State = BackupStateStale
	}

	return status
}

func (s *Service) readBackupMarker() ([]byte, BackupState) {
	fileInfo, err := os.Lstat(s.config.BackupStatusPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, BackupStateNotRecorded
	}

	if err != nil ||
		!fileInfo.Mode().IsRegular() ||
		fileInfo.Mode()&os.ModeSymlink != 0 {
		return nil, BackupStateUnavailable
	}

	file, err := os.Open(s.config.BackupStatusPath)
	if err != nil {
		return nil, BackupStateUnavailable
	}

	defer func() { _ = file.Close() }()

	openedFileInfo, err := file.Stat()
	if err != nil || !openedFileInfo.Mode().IsRegular() {
		return nil, BackupStateUnavailable
	}

	contents, err := io.ReadAll(io.LimitReader(file, maxBackupStatusBytes+1))
	if err != nil || len(contents) > maxBackupStatusBytes {
		return nil, BackupStateInvalid
	}

	return contents, ""
}

func validateBackupStatusPath(config Config) error {
	dataDirectoryPath := config.dataDirectory
	if dataDirectoryPath == "" {
		dataDirectoryPath = dataDirectory
	}

	if !filepath.IsAbs(dataDirectoryPath) ||
		!filepath.IsAbs(config.BackupStatusPath) ||
		filepath.Clean(filepath.Dir(config.BackupStatusPath)) !=
			filepath.Clean(dataDirectoryPath) {
		return ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			//nolint:lll // one logical configuration error message
			"CHATZ_BACKUP_STATUS_PATH must name a file directly under %q, got %q",
			dataDirectoryPath,
			config.BackupStatusPath,
		)
	}

	return nil
}

type backupMarker struct {
	CompletedAt time.Time `json:"completedAt"`
	Driver      db.Driver `json:"driver"`
}

func parseBackupMarker(contents []byte) (backupMarker, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()

	var marker backupMarker
	if err := decoder.Decode(&marker); err != nil {
		return backupMarker{}, ctxerrors.Wrap(
			err,
			"decode backup status marker",
		)
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return backupMarker{}, ctxerrors.Wrap(
			commerr.ErrInvalidArgument,
			"backup status marker contains more than one JSON value",
		)
	}

	return marker, nil
}

func (m backupMarker) valid(now time.Time, databaseDriver db.Driver) bool {
	if m.CompletedAt.IsZero() || m.CompletedAt.After(now) {
		return false
	}

	return (m.Driver == backupStatusDriverPostgres ||
		m.Driver == backupStatusDriverSQLite) &&
		m.Driver == databaseDriver
}
