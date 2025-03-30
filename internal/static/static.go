package static

import "embed"

//go:embed css/* js/* images/*
var StaticFS embed.FS
