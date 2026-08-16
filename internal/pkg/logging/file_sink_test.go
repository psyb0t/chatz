package logging

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multiWriterLogger builds a logger writing JSON to BOTH an in-memory stdout
// stand-in and a real temp file, mirroring the shape AddFileSink stacks onto
// slogconf's existing handler (stdout/stderr untouched, file added
// alongside).
func multiWriterLogger(
	t *testing.T, stdout *bytes.Buffer, file *os.File,
) *slog.Logger {
	t.Helper()

	multi := io.MultiWriter(stdout, file)

	return slog.New(slog.NewJSONHandler(multi, nil))
}

func TestFileSink_MultiWriterConstruction(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "chatz.log")

	f, err := os.OpenFile(
		path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm,
	)
	require.NoError(t, err, "multi-writer construction must not error")
	t.Cleanup(func() { _ = f.Close() })

	stdout := &bytes.Buffer{}
	logger := multiWriterLogger(t, stdout, f)

	logger.Info("hello", "k", "v")

	assert.Contains(t, stdout.String(), `"msg":"hello"`,
		"stdout stream must still receive the record")

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(written), `"msg":"hello"`,
		"file sink must also receive the record")
}

// TestAddFileSink_OpensAndTruncates proves AddFileSink opens LogFilePath
// relative to CWD, that a subsequent call truncates rather than appends (the
// per-boot-truncate contract), and that a returned closer works.
func TestAddFileSink_OpensAndTruncates(t *testing.T) {
	dir := t.TempDir()

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	prevDefault := slog.Default()

	t.Cleanup(func() { slog.SetDefault(prevDefault) })

	closer, err := AddFileSink()
	require.NoError(t, err)
	require.NotNil(t, closer)

	slog.Info("first boot line")
	require.NoError(t, closer.Close())

	firstRead, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)
	assert.Contains(t, string(firstRead), "first boot line")

	closer2, err := AddFileSink()
	require.NoError(t, err)

	t.Cleanup(func() { _ = closer2.Close() })

	secondRead, err := os.ReadFile(LogFilePath)
	require.NoError(t, err)
	assert.NotContains(t, string(secondRead), "first boot line",
		"AddFileSink must truncate on open, not append across boots")
}
