package onnx

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/dltest"
)

// A broken runtime or model download (404, cut off, reset, truncated, empty) is
// refused with the URL in the message, leaves nothing behind, and the same install
// works once the server is right.
func TestBrokenDownloadsAreRefusedAndTheRetrySucceeds(t *testing.T) {
	f := newFixture(t)
	rt := dltest.NewSwitch(t, f.rt, dltest.OK)
	md := dltest.NewSwitch(t, f.md, dltest.OK)
	f.p.runtime.URL = rt.URL + "/rt"
	f.p.models[0].Archive.URL = md.URL + "/model"
	ctx := context.Background()
	e := platform.Entry{Package: "tts.fake"}

	for _, m := range dltest.Failures {
		rt.Set(m)
		err := f.p.Install(ctx, func(string) {})
		if err == nil || !strings.Contains(err.Error(), rt.URL+"/rt") {
			t.Fatalf("runtime, %s: want an error naming the URL, got %v", m, err)
		}
		if f.p.Status(ctx).Ready {
			t.Errorf("runtime, %s: ready after a refused download", m)
		}
		dltest.AssertEmpty(t, f.p.Dir())
	}
	rt.Set(dltest.OK)
	if err := f.p.Install(ctx, func(string) {}); err != nil {
		t.Fatal(err)
	}

	for _, m := range dltest.Failures {
		md.Set(m)
		var lines []string
		if f.p.Create(ctx, e, func(l string) { lines = append(lines, l) }) {
			t.Fatalf("model, %s: Create accepted a broken download", m)
		}
		if !strings.Contains(strings.Join(lines, "\n"), md.URL+"/model") {
			t.Errorf("model, %s: the message does not name the file: %v", m, lines)
		}
		if st := f.p.EntryStatus(ctx, e); st.Installed || st.Partial {
			t.Errorf("model, %s: status %+v", m, st)
		}
		dltest.AssertEmpty(t, filepath.Join(f.p.Dir(), "models"))
	}
	md.Set(dltest.OK)
	if !f.p.Create(ctx, e, func(string) {}) || !f.p.EntryStatus(ctx, e).Installed {
		t.Fatal("the retry of the model failed")
	}
}
