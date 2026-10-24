// Package migrations holds the SQL migrations of the core schema, embedded into the binary.
package migrations

import "embed"

// FS contains every *.sql migration file of this directory.
//
//go:embed *.sql
var FS embed.FS
