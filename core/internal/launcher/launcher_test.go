package launcher_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// newTestInstall sets up a minimal install root and returns a running test server.
func newTestInstall(t *testing.T) (paths.Root, *server.Server, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(0)
	if err != nil {
		t.Fatal(err)
	}
	l := launcher.New(root)
	l.Register(srv)
	return root, srv, srv.Token()
}

func doRequest(t *testing.T, srv *server.Server, token, method, path, body string) *http.Response {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Host = srv.Addr() // match the actual server port (may be 0 in tests)
	req.Header.Set("X-TTS-Token", token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w.Result()
}

func TestStepsStatus_ValidJSON(t *testing.T) {
	_, srv, token := newTestInstall(t)
	resp := doRequest(t, srv, token, http.MethodGet, "/api/steps/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var statuses []launcher.StepStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		t.Fatalf("decoding status: %v", err)
	}
	if len(statuses) != 7 {
		t.Fatalf("expected 7 steps, got %d", len(statuses))
	}

	stepNames := make(map[string]bool)
	for _, s := range statuses {
		stepNames[s.Step] = true
	}
	for _, want := range []string{"download", "preflight", "mamba", "gpu", "install", "update", "run"} {
		if !stepNames[want] {
			t.Errorf("missing step %q in status response", want)
		}
	}
}

func TestStepsStatus_NoToken_Forbidden(t *testing.T) {
	_, srv, _ := newTestInstall(t)
	req := httptest.NewRequest(http.MethodGet, "/api/steps/status", nil)
	req.Host = srv.Addr()
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without token, got %d", w.Result().StatusCode)
	}
}

func TestStepsStatus_DownloadDone_WhenScriptsExist(t *testing.T) {
	_, srv, token := newTestInstall(t)
	// installer_scripts/ already exists (created by newTestInstall).
	resp := doRequest(t, srv, token, http.MethodGet, "/api/steps/status", "")
	var statuses []launcher.StepStatus
	_ = json.NewDecoder(resp.Body).Decode(&statuses)
	for _, s := range statuses {
		if s.Step == "download" && !s.Done {
			t.Error("expected download=done when installer_scripts/ exists")
		}
	}
}

func TestGPUEndpoint_WritesMarker(t *testing.T) {
	root, srv, token := newTestInstall(t)
	resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/gpu", `{"label":"NVIDIA GPU"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	data, err := os.ReadFile(root.MarkerFile(".gpu"))
	if err != nil {
		t.Fatalf("reading .gpu: %v", err)
	}
	if !strings.Contains(string(data), "NVIDIA GPU") {
		t.Errorf(".gpu does not contain 'NVIDIA GPU': %q", data)
	}
}

func TestGPUEndpoint_MissingLabel_BadRequest(t *testing.T) {
	_, srv, token := newTestInstall(t)
	resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/gpu", `{}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty label, got %d", resp.StatusCode)
	}
}

func TestPreflightEndpoint_Returns7Checks(t *testing.T) {
	root, srv, token := newTestInstall(t)
	resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/preflight", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var pr struct {
		Checks []struct{ ID string } `json:"checks"`
		OK     bool                  `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		t.Fatalf("decoding preflight response: %v", err)
	}
	if len(pr.Checks) == 0 {
		t.Error("expected at least one preflight check")
	}
	// If all passed, marker should exist.
	if pr.OK {
		if _, err := os.Stat(root.MarkerFile(".preflight_ok")); err != nil {
			t.Error("expected .preflight_ok marker when preflight passes")
		}
	}
}
