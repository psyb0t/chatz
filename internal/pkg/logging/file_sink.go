package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/slogging/slogconf"
)

// LogFilePath is where the file sink writes, relative to the process CWD
// (/app in the production image — see docker-compose.yml's bind mount of
// ./chatz.log:/app/chatz.log so it's readable from the host). Hardcoded
// per config.go's convention: no existing CHATZ_* env var covers a log-file
// path, so this stays a constant rather than adding config surface for a
// single fixed value.
const LogFilePath = "chatz.log"

// filePerm matches the file's own default 0o644 (owner read/write, world
// read) — logs aren't secret-bearing once redaction (redact.go) is wired
// ahead of this sink, and the admin needs to read/attach the file without sudo.
const filePerm = 0o644

// AddFileSink opens LogFilePath (truncating any content from a previous
// boot — this file is a per-run diagnostic tail, not a rotating archive) and
// stacks a JSON handler writing to it alongside whatever slogconf
// already set up (stdout/stderr). The app's normal stdout stream — the one
// `docker compose logs` reads — is untouched.
//
// Returns the opened file so the caller can close it on shutdown; callers
// that don't care about a clean close (the CLI process just exits) may ignore
// it.
func AddFileSink() (io.Closer, error) {
	f, err := os.OpenFile(
		LogFilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		filePerm,
	)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "open chatz.log")
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	slogconf.AddSink(NewRedactingHandler(handler))

	return f, nil
}
