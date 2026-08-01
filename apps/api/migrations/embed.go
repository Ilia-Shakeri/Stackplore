// Package migrations exposes embedded, ordered database migrations.
package migrations

import "embed"

// FS contains the forward-only SQL migration files.
//
//go:embed *.sql
var FS embed.FS
