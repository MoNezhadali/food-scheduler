// Package migrations embeds SQL migration files for all supported drivers.
package migrations

import "embed"

//go:embed sqlite
var SQLiteFS embed.FS
