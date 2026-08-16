package main

import (
	"log/slog"

	"github.com/psyb0t/chatz/internal/pkg/logging"
)

// This file is yours - it never gets replaced by framework updates.
// Use this init() to set up custom slog handlers, global config,
// or anything else that needs to run before the app starts.

// init runs after slogconf's own blank-import init (Go initializes
// imported packages before the importing package's init()), so
// slog.Default() is already the stdout/stderr handler this wraps.
//
//nolint:gochecknoinits
func init() {
	logging.WrapDefaultWithRedaction()

	if _, err := logging.AddFileSink(); err != nil {
		// Logging setup failing must not be fatal — the app still has
		// stdout/stderr logging via slogconf. Warn and continue.
		slog.Warn("chatz.log file sink not started",
			"err", err, "reason", "file_sink_open_failed")
	}
}
