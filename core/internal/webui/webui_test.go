package webui

import (
	"io/fs"
	"testing"
)

func TestIndexEmbedded(t *testing.T) {
	if _, err := fs.Stat(FS(), "index.html"); err != nil {
		t.Fatalf("index.html not embedded: %v", err)
	}
}
