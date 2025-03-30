package templates

import "embed"

//go:embed html/*.html
var TemplateFS embed.FS
