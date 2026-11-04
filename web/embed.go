package web

import (
	"embed"
	"io/fs"
)

// DistFS contains the production compiled React 19 web assets.
//
//go:embed all:dist
var DistFS embed.FS

// GetSubFS returns the sub filesystem rooted at 'dist'.
func GetSubFS() (fs.FS, error) {
	return fs.Sub(DistFS, "dist")
}
