package launcher

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServerPyEnabled(t *testing.T) {
	for _, tc := range []struct {
		val     string
		set     bool
		want    bool
		wantErr bool
	}{
		{set: false, want: false},
		{val: "", set: true, want: false},
		{val: "0", set: true, want: false},
		{val: "1", set: true, want: true},
		{val: " 1 ", set: true, want: true},
		{val: "yes", set: true, wantErr: true},
		{val: "true", set: true, wantErr: true},
	} {
		lookup := func(k string) (string, bool) {
			if k == EnvServerPy && tc.set {
				return tc.val, true
			}
			return "", false
		}
		got, err := serverPyEnabled(lookup)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("%s=%q (set=%v): got %v, %v; want %v, err=%v", EnvServerPy, tc.val, tc.set, got, err, tc.want, tc.wantErr)
		}
	}
}

// envMap turns KEY=VALUE pairs into a map.
func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}

func get(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("X-TTS-Core-Token", token)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServerPyRoutesToGradioAndServesControlAPI(t *testing.T) {
	sp, err := startServerPyOn(makeRoot(t), "127.0.0.1", 0, []string{"no-such-micromamba", "run"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()

	env := envMap(sp.gradioEnv)
	for _, k := range []string{"TTS_CORE_URL", "TTS_CORE_TOKEN", "TTS_GRADIO_PORT", "TTS_GRADIO_HOST"} {
		if env[k] == "" {
			t.Fatalf("gradio env has no %s: %q", k, sp.gradioEnv)
		}
	}
	if env["TTS_GRADIO_HOST"] != "127.0.0.1" {
		t.Errorf("TTS_GRADIO_HOST = %q, want 127.0.0.1", env["TTS_GRADIO_HOST"])
	}
	if !strings.HasPrefix(env["TTS_CORE_URL"], "http://127.0.0.1:") {
		t.Errorf("TTS_CORE_URL = %q, want a loopback URL", env["TTS_CORE_URL"])
	}
	if len(env["TTS_CORE_TOKEN"]) != 64 {
		t.Errorf("token has %d hex chars, want 64", len(env["TTS_CORE_TOKEN"]))
	}

	// A stand-in Gradio on the port the gradio child is told to bind.
	gln, err := net.Listen("tcp", "127.0.0.1:"+env["TTS_GRADIO_PORT"])
	if err != nil {
		t.Fatal(err)
	}
	gsrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "gradio "+r.URL.Path)
	})}
	go func() { _ = gsrv.Serve(gln) }()
	defer gsrv.Close()

	if code, body := get(t, "http://"+sp.publicAddr+"/config", ""); code != 200 || body != "gradio /config" {
		t.Fatalf("router GET /config = %d %q, want 200 \"gradio /config\"", code, body)
	}

	token := env["TTS_CORE_TOKEN"]
	if code, _ := get(t, env["TTS_CORE_URL"]+"/api/extensions", ""); code != http.StatusForbidden {
		t.Errorf("control API without token = %d, want 403", code)
	}
	if code, body := get(t, env["TTS_CORE_URL"]+"/api/extensions", token); code != 200 {
		t.Errorf("GET /api/extensions = %d %q, want 200", code, body)
	}
	if code, body := get(t, env["TTS_CORE_URL"]+"/api/routes", token); code != 200 || !strings.Contains(body, env["TTS_GRADIO_PORT"]) {
		t.Errorf("GET /api/routes = %d %q, want 200 listing the gradio route", code, body)
	}

	sp.Close()
	sp.Close() // idempotent
	if c, err := net.DialTimeout("tcp", sp.publicAddr, time.Second); err == nil {
		c.Close()
		t.Errorf("router still listening on %s after Close", sp.publicAddr)
	}
	if c, err := net.DialTimeout("tcp", strings.TrimPrefix(env["TTS_CORE_URL"], "http://"), time.Second); err == nil {
		c.Close()
		t.Errorf("control API still listening after Close")
	}
}

func TestServerPyPublicPortInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	_, err = startServerPyOn(makeRoot(t), "127.0.0.1", port, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatalf("got %v, want a port-in-use error", err)
	}
}

func TestServerPyUsesConfigPort(t *testing.T) {
	root := makeRoot(t)
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	cfg := `{"gradio_interface_options": {"auth": null, "server_name": "127.0.0.1", "server_port": ` + strconv.Itoa(port) + `}}`
	if err := os.WriteFile(filepath.Join(root.Dir(), "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, err := startServerPy(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	if want := "127.0.0.1:" + strconv.Itoa(port); sp.publicAddr != want {
		t.Fatalf("router on %s, want %s (config.json server_port)", sp.publicAddr, want)
	}
}

func TestSupervisorRejectsBadServerPySwitch(t *testing.T) {
	t.Setenv(EnvServerPy, "yes")
	sup := newSupervisor(makeRoot(t))
	defer sup.Stop()
	err := sup.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), EnvServerPy) {
		t.Fatalf("got %v, want an error naming %s", err, EnvServerPy)
	}
}
