// Package webui embeds the static UI bundle served by the core.
//
// The embedded directory must live inside this package directory (go:embed
// cannot reach outside it). The repository ships a placeholder index.html in
// assets/. The release build copies the Vite output from core/ui/dist/ over
// internal/webui/assets/ before running `go build`.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:assets
var embedded embed.FS

// FS returns the embedded UI rooted at the assets directory, so that
// "index.html" is at the top level.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "assets")
	if err != nil {
		panic(err) // unreachable: "assets" is guaranteed by the embed directive
	}
	return sub
}
