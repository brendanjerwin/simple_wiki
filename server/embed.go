package server

import (
	"embed"
)

//go:embed templates/index.tmpl
var IndexTemplate string

//go:embed static/**
var StaticContent embed.FS
