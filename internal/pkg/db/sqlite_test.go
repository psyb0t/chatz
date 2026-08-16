package db

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sqliteTestBusyTimeout        = time.Second
	sqliteConcurrentMessageCount = 12
)

func TestValidateSQLiteConfig(t *testing.T) {
	t.Parallel()

	dataDirectory := t.TempDir()
	databasePath := filepath.Join(dataDirectory, "chatz.sqlite")
	missingDirectory := filepath.Join(dataDirectory, "missing")
	nestedDirectory := filepath.Join(dataDirectory, "nested")
	require.NoError(t, os.Mkdir(nestedDirectory, 0o700))

	badFilePath := filepath.Join(nestedDirectory, "chatz.sqlite")
	symlinkPath := filepath.Join(dataDirectory, "chatz-link.sqlite")
	require.NoError(t, os.Symlink(databasePath, symlinkPath))

	testCases := []struct {
		name    string
		config  Config
		wantErr error
	}{
		{
			name:   "valid new database",
			config: sqliteTestConfig(dataDirectory, databasePath),
		},
		{
			name: "nonpositive busy timeout",
			config: Config{
				SQLitePath:          databasePath,
				sqliteDataDirectory: dataDirectory,
			},
			wantErr: commerr.ErrInvalidArgument,
		},
		{
			name:    "missing data directory",
			config:  sqliteTestConfig(missingDirectory, databasePath),
			wantErr: os.ErrNotExist,
		},
		{
			name:    "database outside data directory",
			config:  sqliteTestConfig(dataDirectory, badFilePath),
			wantErr: commerr.ErrInvalidArgument,
		},
		{
			name:    "database symlink",
			config:  sqliteTestConfig(dataDirectory, symlinkPath),
			wantErr: commerr.ErrInvalidArgument,
		},
		{
			name: "dsn query delimiter",
			config: sqliteTestConfig(
				dataDirectory,
				databasePath+"?mode=memory",
			),
			wantErr: commerr.ErrInvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateSQLiteConfig(tc.config)
			if tc.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestConnectSQLite_PersistsSchemaAndEnforcesConstraints(t *testing.T) {
	ctx := context.Background()
	dataDirectory := t.TempDir()
	databasePath := filepath.Join(dataDirectory, "chatz.sqlite")
	config := sqliteTestConfig(dataDirectory, databasePath)

	first, err := Connect(ctx, config)
	require.NoError(t, err)

	assertSQLitePragmas(t, first)
	seedSQLiteConversation(ctx, t, first)
	require.NoError(t, first.Close())

	second, err := Connect(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, second.Close())
	})

	assertSQLitePragmas(t, second)
	assertSQLiteRestartState(ctx, t, second)
}

func TestConnectSQLite_MigrationStatus(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	handle, err := Connect(
		t.Context(),
		sqliteTestConfig(directory, filepath.Join(directory, "chatz.sqlite")),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, handle.Close())
	})

	status, err := handle.MigrationStatus(t.Context())
	require.NoError(t, err)
	assert.Positive(t, status.Version)
	assert.False(t, status.Dirty)
}

func TestConnectSQLite_SerializesConcurrentRepositoryWrites(t *testing.T) {
	ctx := context.Background()
	dataDirectory := t.TempDir()
	databasePath := filepath.Join(dataDirectory, "chatz.sqlite")
	handle, err := Connect(
		ctx,
		sqliteTestConfig(dataDirectory, databasePath),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, handle.Close())
	})

	user := &models.User{Username: "concurrent-user"}
	require.NoError(t, repositories.Q.User.WithContext(ctx).Create(user))

	chat := &models.Chat{UserID: user.ID, Title: "concurrent", ModelID: "model"}
	require.NoError(t, repositories.Q.Chat.WithContext(ctx).Create(chat))

	errorsByWrite := make(chan error, sqliteConcurrentMessageCount)

	var writes sync.WaitGroup
	writes.Add(sqliteConcurrentMessageCount)

	for range sqliteConcurrentMessageCount {
		go func() {
			defer writes.Done()

			turnComplete := true
			errorsByWrite <- repositories.Q.Message.WithContext(ctx).Create(
				&models.Message{
					TurnID:       uuid.New(),
					TurnComplete: &turnComplete,
					ChatID:       chat.ID,
					Role:         models.MessageRoleUser,
					Content:      "concurrent",
				},
			)
		}()
	}

	writes.Wait()
	close(errorsByWrite)

	for err := range errorsByWrite {
		require.NoError(t, err)
	}

	messages, err := repositories.Q.Message.WithContext(ctx).
		Order(repositories.Q.Message.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, messages, sqliteConcurrentMessageCount)

	for index, message := range messages {
		assert.EqualValues(t, index+1, message.Position)
	}
}

func sqliteTestConfig(dataDirectory, databasePath string) Config {
	return Config{
		Driver:              DriverSQLite,
		SQLitePath:          databasePath,
		SQLiteBusyTimeout:   sqliteTestBusyTimeout,
		sqliteDataDirectory: dataDirectory,
	}
}

func assertSQLitePragmas(t *testing.T, handle *Handle) {
	t.Helper()

	var journalMode string
	require.NoError(t, handle.GormDB.Raw(
		"PRAGMA journal_mode",
	).Scan(&journalMode).Error)
	assert.Equal(t, "wal", journalMode)

	var foreignKeys int
	require.NoError(t, handle.GormDB.Raw(
		"PRAGMA foreign_keys",
	).Scan(&foreignKeys).Error)
	assert.Equal(t, 1, foreignKeys)
}

func seedSQLiteConversation(ctx context.Context, t *testing.T, handle *Handle) {
	t.Helper()

	user := &models.User{Username: "sqlite-user"}
	require.NoError(t, repositories.Q.User.WithContext(ctx).Create(user))

	chat := &models.Chat{UserID: user.ID, Title: "SQLite", ModelID: "model"}
	require.NoError(t, repositories.Q.Chat.WithContext(ctx).Create(chat))

	turnComplete := true
	firstMessage := &models.Message{
		TurnID:       uuid.New(),
		TurnComplete: &turnComplete,
		ChatID:       chat.ID,
		Role:         models.MessageRoleUser,
		Content:      "first",
	}
	secondMessage := &models.Message{
		TurnID:       uuid.New(),
		TurnComplete: &turnComplete,
		ChatID:       chat.ID,
		Role:         models.MessageRoleAssistant,
		Content:      "second",
	}

	require.NoError(t, repositories.Q.Message.WithContext(ctx).
		Create(firstMessage))
	require.NoError(t, repositories.Q.Message.WithContext(ctx).
		Create(secondMessage))

	messages, err := repositories.Q.Message.WithContext(ctx).
		Order(repositories.Q.Message.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.EqualValues(t, 1, messages[0].Position)
	assert.EqualValues(t, 2, messages[1].Position)

	toolExecution := &models.MCPToolExecution{
		MessageID: firstMessage.ID,
		Server:    "operations",
		Tool:      "inspect",
		Result:    "ok",
	}
	require.NoError(t, repositories.Q.MCPToolExecution.WithContext(ctx).
		Create(toolExecution))

	foreignKeyError := handle.GormDB.WithContext(ctx).
		Create(&models.MCPToolExecution{
			MessageID: uuid.New(),
			Server:    "operations",
			Tool:      "inspect",
			Result:    "orphan",
		}).Error
	require.Error(t, foreignKeyError)
}

func assertSQLiteRestartState(
	ctx context.Context,
	t *testing.T,
	handle *Handle,
) {
	t.Helper()

	var migrationCount int
	require.NoError(t, handle.GormDB.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM schema_migrations",
	).Scan(&migrationCount).Error)
	assert.Equal(t, 1, migrationCount)

	users, err := repositories.Q.User.WithContext(ctx).Find()
	require.NoError(t, err)
	require.Len(t, users, 1)

	messages, err := repositories.Q.Message.WithContext(ctx).
		Order(repositories.Q.Message.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.EqualValues(t, 1, messages[0].Position)
	assert.EqualValues(t, 2, messages[1].Position)

	var executionCount int
	require.NoError(t, handle.GormDB.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM mcp_tool_executions",
	).Scan(&executionCount).Error)
	assert.Equal(t, 1, executionCount)

	turnComplete := true
	thirdMessage := &models.Message{
		TurnID:       uuid.New(),
		TurnComplete: &turnComplete,
		ChatID:       messages[0].ChatID,
		Role:         models.MessageRoleUser,
		Content:      "after restart",
	}
	require.NoError(t, repositories.Q.Message.WithContext(ctx).
		Create(thirdMessage))

	messages, err = repositories.Q.Message.WithContext(ctx).
		Order(repositories.Q.Message.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.EqualValues(t, 3, messages[2].Position)
}

func TestConnect_RejectsUnsupportedDriver(t *testing.T) {
	t.Parallel()

	_, err := Connect(context.Background(), Config{Driver: "unknown"})
	require.ErrorIs(t, err, commerr.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "CHATZ_DB_DRIVER")
}
