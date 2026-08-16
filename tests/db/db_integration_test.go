//go:build integration

package dbtest

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/migrations"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// legacyMessageShapeMigration is the sequence prefix of the migration that
// introduced turn_id / turn_complete. Everything from here on has to roll back
// for the messages table to be in its pre-turn shape again.
const legacyMessageShapeMigration = "0000012"

// migrationsAfterLegacyMessageShape counts how many migrations must roll DOWN
// to reach the pre-turn messages shape, derived rather than hardcoded.
//
// It used to be the literal 2, which the comment on migrationCount already
// warned against — and it went stale the moment a 14th migration landed,
// failing this package with a UUID mismatch that says nothing about the actual
// cause. Deriving it means adding a migration cannot break a test that has no
// relationship to what the migration does.
func migrationsAfterLegacyMessageShape(t *testing.T) int {
	t.Helper()

	entries, err := migrations.FS.ReadDir(".")
	require.NoError(t, err)

	count := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}

		if e.Name() >= legacyMessageShapeMigration {
			count++
		}
	}

	require.Positive(t, count, "no migrations at or after the legacy shape")

	return count
}

// migrationCount counts the *.up.sql files embedded in migrations.FS. The
// vendored postgresql.MigrateDown takes an explicit step count (no "down to
// zero" primitive is exposed), so a hardcoded number would silently go stale
// every time a migration is added — deriving it from the embedded FS means
// TestDB_MigrationsReversible always rolls back everything, no matter how
// many migrations exist.
func migrationCount(t *testing.T) int {
	t.Helper()

	entries, err := migrations.FS.ReadDir(".")
	require.NoError(t, err)

	count := 0

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			count++
		}
	}

	require.Positive(t, count, "no *.up.sql migrations found in embedded FS")

	return count
}

// TestDB_CRUDRoundTrip drives a user -> chat -> message chain through the
// generated repositories against the real migrated schema. It proves the
// migrations applied, ids auto-generate (gen_random_uuid default), and gorm's
// column inference matches the SQL columns — including the jsonb fields
// (ui_spec, tool_calls) which are the riskiest to infer.
func TestDB_CRUDRoundTrip(t *testing.T) {
	ctx := t.Context()

	userRepo := repositories.User
	user := &models.User{Username: "alice", IsAdmin: true}
	require.NoError(t, userRepo.WithContext(ctx).Create(user))
	require.NotEqual(t, uuid.Nil, user.ID)

	gotUser, err := userRepo.WithContext(ctx).
		Where(userRepo.Username.Eq("alice")).
		First()
	require.NoError(t, err)
	assert.Equal(t, "alice", gotUser.Username)
	assert.True(t, gotUser.IsAdmin)

	chatRepo := repositories.Chat
	chat := &models.Chat{UserID: user.ID, Title: "hi", ModelID: "gpt"}
	require.NoError(t, chatRepo.WithContext(ctx).Create(chat))
	require.NotEqual(t, uuid.Nil, chat.ID)

	uiSpec := datatypes.JSON([]byte(`{"type":"card","props":{"title":"x"}}`))
	toolCalls := datatypes.JSON([]byte(`[{"name":"search"}]`))
	msgRepo := repositories.Message
	msg := &models.Message{
		ChatID:    chat.ID,
		Role:      models.MessageRoleAssistant,
		Content:   "hello",
		Reasoning: "thinking it through",
		UISpec:    uiSpec,
		ToolCalls: toolCalls,
		IsError:   true,
	}
	require.NoError(t, msgRepo.WithContext(ctx).Create(msg))

	gotMsg, err := msgRepo.WithContext(ctx).
		Where(msgRepo.ID.Eq(msg.ID)).
		First()
	require.NoError(t, err)
	assert.Equal(t, models.MessageRoleAssistant, gotMsg.Role)
	assert.Equal(t, "hello", gotMsg.Content)
	assert.Equal(t, "thinking it through", gotMsg.Reasoning)
	assert.True(t, gotMsg.IsError)
	assert.JSONEq(t, string(uiSpec), string(gotMsg.UISpec))
	assert.JSONEq(t, string(toolCalls), string(gotMsg.ToolCalls))
	assert.Positive(t, gotMsg.Position)
	assert.NotEqual(t, uuid.Nil, gotMsg.TurnID)
	require.NotNil(t, gotMsg.TurnComplete)
	assert.True(t, *gotMsg.TurnComplete)

	incomplete := false
	pendingTurnID := uuid.New()
	pending := &models.Message{
		ChatID:       chat.ID,
		TurnID:       pendingTurnID,
		TurnComplete: &incomplete,
		Role:         models.MessageRoleUser,
		Content:      "pending",
	}
	require.NoError(t, msgRepo.WithContext(ctx).Create(pending))

	gotPending, err := msgRepo.WithContext(ctx).
		Where(msgRepo.ID.Eq(pending.ID)).
		First()
	require.NoError(t, err)
	assert.Equal(t, pendingTurnID, gotPending.TurnID)
	require.NotNil(t, gotPending.TurnComplete)
	assert.False(t, *gotPending.TurnComplete)
}

// TestDB_SoftDeleteHidesRow confirms the gorm soft-delete column wired by the
// migrations works: a deleted chat is excluded from normal queries.
func TestDB_SoftDeleteHidesRow(t *testing.T) {
	ctx := t.Context()

	userRepo := repositories.User
	user := &models.User{Username: "bob"}
	require.NoError(t, userRepo.WithContext(ctx).Create(user))

	chatRepo := repositories.Chat
	chat := &models.Chat{UserID: user.ID, Title: "gone"}
	require.NoError(t, chatRepo.WithContext(ctx).Create(chat))

	_, err := chatRepo.WithContext(ctx).
		Where(chatRepo.ID.Eq(chat.ID)).
		Delete()
	require.NoError(t, err)

	_, err = chatRepo.WithContext(ctx).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.Error(t, err)
}

// TestDB_TurnMigrationBackfillsLegacyRows proves the turn metadata migration
// assigns one ID to each inferred legacy turn and does not complete a trailing
// user attempt that has no assistant response.
func TestDB_TurnMigrationBackfillsLegacyRows(t *testing.T) {
	ctx := t.Context()

	user := &models.User{Username: "migration-backfill"}
	require.NoError(t, repositories.User.WithContext(ctx).Create(user))

	chat := &models.Chat{UserID: user.ID, Title: "legacy"}
	require.NoError(t, repositories.Chat.WithContext(ctx).Create(chat))

	require.NoError(
		t,
		testInfra.PG.MigrateDown(
			".",
			migrationsAfterLegacyMessageShape(t),
			&migrations.FS,
		),
	)
	t.Cleanup(func() {
		require.NoError(t, testInfra.PG.MigrateUp(".", &migrations.FS))
	})

	insertLegacyMessage := `
INSERT INTO messages (id, chat_id, role, content, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
`
	legacyRows := []struct {
		id      uuid.UUID
		role    models.MessageRole
		content string
		at      time.Time
	}{
		{uuid.New(), models.MessageRoleUser, "first", time.Unix(100, 0)},
		{uuid.New(), models.MessageRoleAssistant, "done", time.Unix(101, 0)},
		{uuid.New(), models.MessageRoleUser, "unfinished", time.Unix(102, 0)},
	}

	for _, row := range legacyRows {
		_, err := testInfra.PG.SQLDB.ExecContext(
			ctx,
			insertLegacyMessage,
			row.id,
			chat.ID,
			row.role,
			row.content,
			row.at,
		)
		require.NoError(t, err)
	}

	require.NoError(t, testInfra.PG.MigrateUp(".", &migrations.FS))

	repo := repositories.Message
	got, err := repo.WithContext(ctx).
		Where(repo.ChatID.Eq(chat.ID)).
		Order(repo.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, got, len(legacyRows))
	assert.Equal(t, got[0].TurnID, got[1].TurnID)
	assert.NotEqual(t, got[1].TurnID, got[2].TurnID)
	require.NotNil(t, got[0].TurnComplete)
	require.NotNil(t, got[1].TurnComplete)
	require.NotNil(t, got[2].TurnComplete)
	assert.True(t, *got[0].TurnComplete)
	assert.True(t, *got[1].TurnComplete)
	assert.False(t, *got[2].TurnComplete)
}

// TestDB_MigrationsReversible rolls every migration down then back up, proving
// each down is symmetric to its up (no irreversible down). It ends UP with an
// empty schema, so it's safe regardless of test order.
func TestDB_MigrationsReversible(t *testing.T) {
	ctx := t.Context()

	require.NoError(
		t,
		testInfra.PG.MigrateDown(".", migrationCount(t), &migrations.FS),
	)
	require.NoError(t, testInfra.PG.MigrateUp(".", &migrations.FS))

	count, err := repositories.User.WithContext(ctx).Count()
	require.NoError(t, err)
	assert.Zero(t, count)
}
