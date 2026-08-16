// Package logging holds process-boot logging hardening that sits on top of
// slogconf's default handler: secret redaction (this file) and the
// gitignored chatz.log file sink (filesink.go). Both are wired from
// cmd/init.go, the framework's designated hook for custom slog handlers.
package logging

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"

	"github.com/psyb0t/ctxerrors"
)

// redactedValue replaces any attr whose key looks secret-shaped.
const redactedValue = "[REDACTED]"

// maxRedactDepth bounds recursion into nested slog groups so a maliciously
// (or accidentally) deep group tree can't blow the stack.
const maxRedactDepth = 8

// sensitiveKeyRE matches attr keys that must never reach a log sink in
// plaintext: passwords, tokens, secrets, API keys, auth headers, cookies.
// Case-insensitive so "Authorization", "authorization", and "AUTHORIZATION"
// all match.
var sensitiveKeyRE = regexp.MustCompile(
	`(?i)(password|token|secret|api[_-]?key|authorization|cookie)`,
)

var sensitiveAssignmentRE = regexp.MustCompile(
	`(?i)\b(password|token|secret|api[_-]?key|authorization|cookie)` +
		`\s*[:=]\s*(?:Bearer\s+)?[^\s,;]+`,
)

// RedactText masks sensitive key/value pairs inside text before the text is
// assigned to an otherwise non-sensitive log key such as message content.
// JSON is traversed structurally; ordinary text uses a conservative
// key/value-pattern fallback.
func RedactText(text string) string {
	var value any
	if json.Unmarshal([]byte(text), &value) == nil {
		redacted, changed := redactJSONValue(value)
		if changed {
			encoded, err := json.Marshal(redacted)
			if err == nil {
				return string(encoded)
			}
		}
	}

	return sensitiveAssignmentRE.ReplaceAllString(text, "$1="+redactedValue)
}

func redactJSONValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		changed := false

		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			if sensitiveKeyRE.MatchString(key) {
				out[key] = redactedValue
				changed = true

				continue
			}

			redacted, nestedChanged := redactJSONValue(nested)
			out[key] = redacted
			changed = changed || nestedChanged
		}

		return out, changed
	case []any:
		changed := false

		out := make([]any, len(typed))
		for i, nested := range typed {
			redacted, nestedChanged := redactJSONValue(nested)
			out[i] = redacted
			changed = changed || nestedChanged
		}

		return out, changed
	default:
		return value, false
	}
}

// RedactingHandler wraps an inner slog.Handler and masks the value of any
// attr (at any nesting depth, including inside slog groups) whose key
// matches sensitiveKeyRE. It's the safety net for the DEBUG-level SQL
// logging already enabled in this deployment and for any future MCP
// bearer-token / API-key field that ends up in a log call.
type RedactingHandler struct {
	inner slog.Handler
}

// NewRedactingHandler wraps inner so every record it handles is redacted
// first.
func NewRedactingHandler(inner slog.Handler) *RedactingHandler {
	return &RedactingHandler{inner: inner}
}

// WrapDefaultWithRedaction rewraps slog.Default()'s current handler (the
// stdout/stderr split slogconf already set up at its own init time)
// in a RedactingHandler, so every log line — not just future AddSink
// sinks like the chatz.log file — gets secret-shaped fields masked.
//
// Must run AFTER slogconf's blank-import init (guaranteed by Go's
// init ordering: imported packages initialize before the importing package's
// own init() runs) and BEFORE AddFileSink, so the file sink is added on top
// of the now-redacting default rather than bypassing it.
func WrapDefaultWithRedaction() {
	current := slog.Default().Handler()
	slog.SetDefault(slog.New(NewRedactingHandler(current)))
}

// Enabled delegates to the inner handler's level gate.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle redacts every attr on the record (recursively through groups) before
// delegating to the inner handler.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	redacted := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	r.Attrs(func(a slog.Attr) bool {
		redacted.AddAttrs(redactAttr(a, 0))

		return true
	})

	if err := h.inner.Handle(ctx, redacted); err != nil {
		return ctxerrors.Wrap(err, "handle redacted record")
	}

	return nil
}

// WithAttrs redacts the pre-bound attrs (from logger.With(...)) the same way
// as per-call attrs, then delegates.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redactedAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redactedAttrs[i] = redactAttr(a, 0)
	}

	return &RedactingHandler{inner: h.inner.WithAttrs(redactedAttrs)}
}

// WithGroup delegates group-scoping to the inner handler; group members still
// pass through Handle/WithAttrs and get redacted there.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr masks a's value if its key matches sensitiveKeyRE, and recurses
// into group values (bounded by maxRedactDepth) so a secret nested inside a
// group attr — e.g. a "headers" group carrying "Authorization" — is caught
// too.
func redactAttr(a slog.Attr, depth int) slog.Attr {
	if depth > maxRedactDepth {
		return a
	}

	if sensitiveKeyRE.MatchString(a.Key) {
		return slog.String(a.Key, redactedValue)
	}

	if a.Value.Kind() != slog.KindGroup {
		return a
	}

	group := a.Value.Group()
	redactedGroup := make([]slog.Attr, len(group))

	for i, ga := range group {
		redactedGroup[i] = redactAttr(ga, depth+1)
	}

	return slog.Attr{Key: a.Key, Value: slog.GroupValue(redactedGroup...)}
}
