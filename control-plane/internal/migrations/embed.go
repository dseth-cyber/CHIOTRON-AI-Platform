// Package migrations carries the AI Platform's schema history inside the
// binary so a deployment can never drift from the migrations it was built with.
package migrations

import "embed"

// Files holds every `NNNN_description.sql` migration in this package.
// Dir names the directory to read inside Files.
//
//go:embed *.sql
var Files embed.FS

const Dir = "."
