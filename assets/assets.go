package assets

import "embed"

// FS holds the embedded font files.
//
//go:embed fonts/*.ttf
var FS embed.FS
