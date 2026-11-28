package launcher

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
)

// startRouterCore runs the app in the new layout: the router on the public port
// serves the React UI at "/" and Gradio sits under /gradio-legacy.
func startRouterCore(t *testing.T) (*supervise.Supervisor, string) {
	t.Helper()
	root, publicPort, reactPort := coreInstall(t)
	cfg := superviseConfig(root, options{noBrowser: true})
	cfg.ReactPort = reactPort // unused in this layout; a free port keeps the config valid
	cfg.SkipReactTab = true
	sup, err := supervise.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sup.Stop)
	if err := sup.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return sup, fmt.Sprintf("http://127.0.0.1:%d", publicPort)
}

func TestRouterServesTheFrontAtRootAndGradioUnderThePrefix(t *testing.T) {
	sup, pub := startRouterCore(t)

	// The React UI and its API are the public port's "/".
	if code, body := httpGet(t, pub+"/"); code != 200 || !strings.Contains(body, "TTS WebUI") {
		t.Fatalf("public /: %d %q", code, body)
	}
	if code, body := httpGet(t, pub+"/api/go-data/logs/files"); code != 200 || strings.TrimSpace(body) != "[]" {
		t.Errorf("the front's API on the public port: %d %q", code, body)
	}
	// With no Gradio host the no-Gradio page moved under the prefix.
	if code, body := httpGet(t, pub+gradioLegacyPrefix+"/"); code != 200 || !strings.Contains(body, "Gradio pages are not installed") {
		t.Errorf("%s/: %d %q", gradioLegacyPrefix, code, body)
	}
	// The status of the React part reports the public port: no port of its own.
	for _, st := range sup.Statuses() {
		if st.Name == supervise.NameReact {
			if !strings.HasSuffix(pub, fmt.Sprintf(":%d", st.Port)) {
				t.Errorf("react-ui reports port %d, want the public port (%s)", st.Port, pub)
			}
		}
	}
}

// While the front is not mounted, "/" says so (and points at the Gradio pages).
func TestFrontStoppedPage(t *testing.T) {
	w := httptest.NewRecorder()
	frontStoppedPage().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "frontend is not running") || !strings.Contains(w.Body.String(), gradioLegacyPrefix+"/") {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

// Gradio under a prefix is told so (its URLs) and where the React UI is (its header link).
func TestGradioPrefixEnvironment(t *testing.T) {
	sp, err := startServerPyOn(makeRoot(t), "127.0.0.1", 0, testPlat(t, []string{"no-such-micromamba", "run"}), appenv.Flags{NoReact: true}, withGradioPrefix(gradioLegacyPrefix))
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	env := strings.Join(sp.gradioEnv, "\n")
	for _, want := range []string{"GRADIO_ROOT_PATH=/gradio-legacy", "TTS_REACT_UI_URL=/"} {
		if !strings.Contains(env, want) {
			t.Errorf("the Gradio environment lacks %s", want)
		}
	}
	plain, err := startServerPyOn(makeRoot(t), "127.0.0.1", 0, testPlat(t, []string{"no-such-micromamba", "run"}), appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	if strings.Contains(strings.Join(plain.gradioEnv, "\n"), "GRADIO_ROOT_PATH") {
		t.Error("Gradio at / got a root path")
	}
}
