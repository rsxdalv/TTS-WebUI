package extcatalog

import (
	"context"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/onnx"
)

// An OS with no pinned sherpa-onnx runtime is not offered an install that can only fail:
// the requirement says why, as audio.cpp's does (found on Linux, where Install was offered
// and then ended with "not available for linux/amd64").
func TestONNXPlatformOnAnOSWithNoPinnedRuntimeIsNotOffered(t *testing.T) {
	s, root, _ := installStore(t)
	pr, _ := paths.New(root)
	s.opts.ONNX = onnx.New(onnx.Options{Root: pr, GOOS: "plan9", GOARCH: "mips"})
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("onnx_fake", "Fake ONNX", `"requires":["onnx"]`)+`]}`)
	g, err := s.RequiresGraph(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st := g.State("onnx_fake")
	if len(st) != 1 || st[0].State != "missing" || st[0].Installable || !strings.Contains(st[0].Would, "not available here") {
		t.Fatalf("requirement state = %+v", st)
	}
	code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"onnx_fake"}`, "tok")
	if code != 400 || !strings.Contains(body, "not available here") {
		t.Errorf("install: %d %s", code, body)
	}
}
