package web

import "embed"

// FS holds embedded dashboard assets.
//
//go:embed index.html static
var FS embed.FS
