package config

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_SecretsBox covers the four outcomes of building the AEAD box from
// the configured key: an unset key means "secrets not configured" (nil box, no
// error), a valid base64 key of the right length builds a box, and a key that
// is either not base64 or the wrong length is a startup error.
func TestConfig_SecretsBox(t *testing.T) {
	t.Parallel()

	t.Run("empty key returns nil box without error", func(t *testing.T) {
		t.Parallel()

		box, err := Config{SecretsKey: ""}.SecretsBox()
		require.NoError(t, err)
		assert.Nil(t, box)
	})

	t.Run("valid base64 key builds a box", func(t *testing.T) {
		t.Parallel()

		key := base64.StdEncoding.EncodeToString(
			make([]byte, secrets.KeySize),
		)
		box, err := Config{SecretsKey: key}.SecretsBox()
		require.NoError(t, err)
		assert.NotNil(t, box)
	})

	t.Run("non-base64 key errors", func(t *testing.T) {
		t.Parallel()

		box, err := Config{SecretsKey: "not!base64!"}.SecretsBox()
		require.Error(t, err)
		assert.Nil(t, box)
	})

	t.Run("wrong-length key errors", func(t *testing.T) {
		t.Parallel()

		key := base64.StdEncoding.EncodeToString(make([]byte, 10))
		box, err := Config{SecretsKey: key}.SecretsBox()
		require.Error(t, err)
		assert.Nil(t, box)
	})
}

// TestConfig_ReadinessConfig maps the config's database + backup fields plus
// the build identity into the operations readiness config for the health probe.
func TestConfig_ReadinessConfig(t *testing.T) {
	t.Parallel()

	c := Config{
		DBDriver:         db.Driver("postgres"),
		BackupStatusPath: "/data/status.json",
		BackupMaxAge:     12 * time.Hour,
	}
	got := c.ReadinessConfig("v1.2.3", "abc123")

	assert.Equal(t, "v1.2.3", got.AppVersion)
	assert.Equal(t, "abc123", got.Commit)
	assert.Equal(t, db.Driver("postgres"), got.DatabaseDriver)
	assert.Equal(t, "/data/status.json", got.BackupStatusPath)
	assert.Equal(t, 12*time.Hour, got.BackupMaxAge)
}
