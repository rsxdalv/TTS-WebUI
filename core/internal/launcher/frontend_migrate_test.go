package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateFrontendDir(t *testing.T) {
	root := t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("react-ui/public/file-input-cache/a.wav", "audio")
	w("react-ui/node_modules/x/index.js", "js") // stale dependencies are not moved
	w("react-ui/src/extensions/ext_a/package.json", "{}")
	w("react-ui/src/extensions/keep.txt", "old")
	w("frontend/package.json", "{}")
	w("frontend/src/extensions/keep.txt", "new")

	migrateFrontendDir(root)

	for _, rel := range []string{"public/file-input-cache/a.wav", "src/extensions/ext_a/package.json"} {
		if _, err := os.Stat(filepath.Join(root, "frontend", filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s not moved: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "frontend", "node_modules")); err == nil {
		t.Error("node_modules was moved")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "frontend", "src", "extensions", "keep.txt")); string(b) != "new" {
		t.Errorf("existing file was overwritten: %q", b)
	}
	migrateFrontendDir(root) // second run changes nothing and does not fail
}

func TestMigrateFrontendDirWithoutNewFolder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "react-ui", "public", "file-input-cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	migrateFrontendDir(root)
	if _, err := os.Stat(filepath.Join(root, "react-ui", "public", "file-input-cache")); err != nil {
		t.Error("moved something although frontend/ does not exist")
	}
}
