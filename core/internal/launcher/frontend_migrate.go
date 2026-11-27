package launcher

import (
	"log/slog"
	"os"
	"path/filepath"
)

// migrateFrontendDir moves what git does not track out of the old react-ui
// folder into frontend/ after an update renamed the folder: uploaded inputs,
// installed extensions and node_modules. Tracked files were already moved by
// git. Nothing is overwritten; anything that cannot be moved stays where it is.
func migrateFrontendDir(root string) {
	oldDir := filepath.Join(root, "react-ui")
	newDir := filepath.Join(root, "frontend")
	if st, err := os.Stat(oldDir); err != nil || !st.IsDir() {
		return
	}
	if st, err := os.Stat(newDir); err != nil || !st.IsDir() {
		return // the update did not bring the new folder; leave the old one alone
	}
	move := func(rel string) {
		src, dst := filepath.Join(oldDir, rel), filepath.Join(newDir, rel)
		if _, err := os.Lstat(src); err != nil {
			return
		}
		if _, err := os.Lstat(dst); err == nil {
			return
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return
		}
		if err := os.Rename(src, dst); err != nil {
			slog.Warn("frontend: could not move from react-ui", "path", rel, "err", err)
			return
		}
		slog.Info("frontend: moved from react-ui", "path", rel)
	}
	move(filepath.Join("public", "file-input-cache"))
	move("node_modules")
	if entries, err := os.ReadDir(filepath.Join(oldDir, "src", "extensions")); err == nil {
		for _, e := range entries {
			move(filepath.Join("src", "extensions", e.Name()))
		}
	}
}
