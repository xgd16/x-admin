package migrations

import "embed"

// FS holds versioned SQL migrations, embedded into the server binary.
//
//go:embed *.sql
var FS embed.FS
