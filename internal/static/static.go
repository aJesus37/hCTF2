package static

import "embed"

//go:embed js/* images/* css/*
var StaticFS embed.FS
