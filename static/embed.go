package static

import (
	"embed"
)

//go:embed templates/index.tmpl
var IndexTemplate string

//go:generate bash -c "cd js && bun run build"
//go:embed **
var StaticContent embed.FS
