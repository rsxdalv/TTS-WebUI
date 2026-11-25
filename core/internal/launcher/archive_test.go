package launcher_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
)

// release serves what CI publishes: tts-webui-installer.json beside the zip.
func release(t *testing.T, commit string, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	all := map[string]string{"archive.json": `{"commit":"` + commit + `"}`}
	for k, v := range files {
		all[k] = v
	}
	for name, body := range all {
		w, _ := zw.Create(name)
		io.WriteString(w, body)
	}
	zw.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/tts-webui-installer.json", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"commit":"`+commit+`"}`)
	})
	mux.HandleFunc("/tts-webui-installer.zip", func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s.URL + "/tts-webui-installer.zip"
}

func statusOf(t *testing.T, srv interface {
	Handler() http.Handler
	Addr() string
}, token, step string) launcher.StepStatus {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/steps/status", nil)
	req.Host = srv.Addr()
	req.Header.Set("X-TTS-Token", token)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	var all []launcher.StepStatus
	if err := json.NewDecoder(w.Result().Body).Decode(&all); err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		if s.Step == step {
			return s
		}
	}
	t.Fatalf("no step %s", step)
	return launcher.StepStatus{}
}

func TestUpdateStep_ArchiveInstall(t *testing.T) {
	root, srv, token := newFreshInstall(t)
	if err := os.WriteFile(filepath.Join(root.Dir(), "archive.json"), []byte(`{"commit":"aaaaaaaa11"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TTS_ARCHIVE_URL", release(t, "bbbbbbbb22", map[string]string{"react-ui/out/index.html": "new ui"}))

	st := statusOf(t, srv, token, "update")
	if st.Done || !strings.Contains(st.Detail, "update is available") {
		t.Fatalf("update step = %+v, want not done with an update available", st)
	}

	resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/update", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST update: %d", resp.StatusCode)
	}
	// Follow the stream to its end; the exit code must be 0.
	stream := doRequest(t, srv, token, http.MethodGet, "/api/steps/update/stream", "")
	body, _ := io.ReadAll(stream.Body)
	if !strings.Contains(string(body), "Update complete") || !strings.Contains(string(body), `"exitCode":0`) {
		t.Fatalf("stream = %s", body)
	}
	if b, _ := os.ReadFile(filepath.Join(root.Dir(), "react-ui", "out", "index.html")); string(b) != "new ui" {
		t.Errorf("the archive was not extracted: %q", b)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		st = statusOf(t, srv, token, "update")
		if st.Done || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !st.Done || st.Detail != "" {
		t.Errorf("after the update: %+v, want done with no detail", st)
	}
}

func TestUpdateStep_UnreachableIsNotUpToDate(t *testing.T) {
	root, srv, token := newFreshInstall(t)
	os.WriteFile(filepath.Join(root.Dir(), "archive.json"), []byte(`{"commit":"aaaaaaaa11"}`), 0o644)
	s := httptest.NewServer(http.NotFoundHandler())
	t.Setenv("TTS_ARCHIVE_URL", s.URL+"/tts-webui-installer.zip")
	defer s.Close()

	st := statusOf(t, srv, token, "update")
	if !strings.Contains(st.Detail, "Could not check for an update") {
		t.Fatalf("update step = %+v, want it to say it could not check", st)
	}
	resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/update", "")
	io.Copy(io.Discard, resp.Body)
	stream := doRequest(t, srv, token, http.MethodGet, "/api/steps/update/stream", "")
	body, _ := io.ReadAll(stream.Body)
	if !strings.Contains(string(body), "ERROR: could not check") || !strings.Contains(string(body), `"exitCode":1`) {
		t.Fatalf("a failed update must end with an error and exit code 1: %s", body)
	}
}

func TestUpdateStep_NoMarkerSaysSo(t *testing.T) {
	_, srv, token := newFreshInstall(t)
	st := statusOf(t, srv, token, "update")
	if !st.Done || !strings.Contains(st.Detail, "no archive.json") {
		t.Fatalf("update step = %+v, want a statement that the version is unknown", st)
	}
}

func TestUpdateStep_GitCloneSaysSo(t *testing.T) {
	root, srv, token := newFreshInstall(t)
	os.MkdirAll(filepath.Join(root.Dir(), ".git"), 0o755)
	st := statusOf(t, srv, token, "update")
	if !strings.Contains(st.Detail, "git clone") {
		t.Fatalf("update step = %+v, want the git clone notice", st)
	}
}
