//go:build integration

package app

import (
	"context"
	"strings"
	"testing"
)

// TestLatestVersion_RealPyPI fetches the actual PyPI endpoint.
// Run with: go test -tags integration ./internal/app/
func TestLatestVersion_RealPyPI(t *testing.T) {
	ver, err := LatestVersion(context.Background())
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if ver == "" {
		t.Fatal("got empty version from PyPI")
	}
	// Sanity: version should look like X.Y or X.Y.Z
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		t.Errorf("version %q does not look like semver", ver)
	}
	t.Logf("latest tts-webui on PyPI: %s", ver)
}
