package launcher

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestReactUIArgsWithProxyOffAreTheOriginalOnes(t *testing.T) {
	root := filepath.Join("C:", "install")
	want := []string{"-p", "env", "run", "npm", "--prefix", filepath.Join(root, "react-ui"), "start", "--", "-p", "3000"}
	if got := reactUIArgs("env", root, 3000, false); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReactUIArgsWithProxyOnStayOnLoopback(t *testing.T) {
	root := filepath.Join("C:", "install")
	want := []string{"-p", "env", "run", "npm", "--prefix", filepath.Join(root, "react-ui"), "start", "--", "-p", "41234", "-H", "127.0.0.1"}
	if got := reactUIArgs("env", root, 41234, true); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
