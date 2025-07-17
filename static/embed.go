// Package static provides embedded static content.
package static

import (
	"embed"
)

//go:embed templates/index.tmpl
var IndexTemplate string // IndexTemplate is the main HTML template.

//go:generate bash -c "cd js && bun install && bun run build"
//go:embed **
var StaticContent embed.FS // StaticContent contains all embedded static files.
