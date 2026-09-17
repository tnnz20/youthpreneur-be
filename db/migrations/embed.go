// Package migrations embeds the SQL schema migrations applied by cmd/migrate.
package migrations

import "embed"

// FS contains the SQL migration files shipped with the binary, so the CLI
// resolves them regardless of the working directory.
//
//go:embed *.sql
var FS embed.FS
