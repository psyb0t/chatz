// Package migrations embeds the SQL migration files into the binary so the
// binary is self-contained (no directory to mount). Both PostgreSQL and SQLite
// run through their dialect-specific common-go migration helpers.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

// SQLiteFS contains the SQLite baseline and all later SQLite-specific schema
// migrations. PostgreSQL history is intentionally never replayed on SQLite.
// Future schema changes add matching migration numbers under both dialects.
//
//go:embed sqlite/*.sql
var SQLiteFS embed.FS
