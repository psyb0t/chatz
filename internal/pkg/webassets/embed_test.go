package webassets

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFS returns a usable, non-nil filesystem rooted at dist/.
func TestFS(t *testing.T) {
	t.Parallel()

	fsys, err := FS()
	require.NoError(t, err)
	assert.NotNil(t, fsys)
}

// TestIsPlaceholder covers all three branches: the committed placeholder
// (marker present), a real build (marker absent), and a missing index.html
// (treated as a placeholder so a broken embed never poses as a real UI).
func TestIsPlaceholder(t *testing.T) {
	t.Parallel()

	t.Run("placeholder marker present", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"index.html": {Data: []byte("<html>CHATZ_SPA_PLACEHOLDER</html>")},
		}
		assert.True(t, IsPlaceholder(fsys))
	})

	t.Run("real build has no marker", func(t *testing.T) {
		t.Parallel()

		fsys := fstest.MapFS{
			"index.html": {Data: []byte("<html><body>real app</body></html>")},
		}
		assert.False(t, IsPlaceholder(fsys))
	})

	t.Run("missing index.html reads as placeholder", func(t *testing.T) {
		t.Parallel()

		assert.True(t, IsPlaceholder(fstest.MapFS{}))
	})
}
