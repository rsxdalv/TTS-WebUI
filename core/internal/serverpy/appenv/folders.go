// Package appenv does the process-level startup preparation that server.py
// does today (output folders, .env creation and loading, flag parsing, port
// planning), so the Go launcher can do it before any child starts.
package appenv

import (
	"os"
	"path/filepath"
)

// EnsureFolders creates outputs and favorites under root if missing
// (server.py create_output_folders). Anything that already exists, even a
// file, is left alone, like os.path.exists.
func EnsureFolders(root string) error {
	for _, d := range []string{"outputs", "favorites"} {
		p := filepath.Join(root, d)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}
	return nil
}
