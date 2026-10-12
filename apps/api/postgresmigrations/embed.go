// Package postgresmigrations exposes immutable PostgreSQL schema migrations.
package postgresmigrations

import "embed"

// FS contains the forward-only PostgreSQL migration files.
//
//go:embed *.sql
var FS embed.FS
