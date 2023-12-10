package static

import (
	"embed"
)

//go:embed templates/index.tmpl
var IndexTemplate string

//go:embed **
var StaticContent embed.FS
