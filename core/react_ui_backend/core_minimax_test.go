package react_ui_backend

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/cloud/minimax"
)

const miniMaxSentinel = "mm-SENTINEL-4d3c2b1a0f9e"

// miniMaxFront builds the real chain: front door, claim, control API, store and a
// fake MiniMax that echoes the key it was sent in its error text.
func miniMaxFront(t *testing.T) (front, root string, remote *httptest.Server, h http.Handler) {
	t.Helper()
	remote = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/t2a_v2" {
			w.WriteHeader(404)
			return
		}
		if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got != miniMaxSentinel {
			_, _ = io.WriteString(w, `{"base_resp":{"status_code":1004,"status_msg":"bad key `+got+`"}}`)
			return
		}
		b, _ := json.Marshal(map[string]any{
			"data":      map[string]any{"audio": hex.EncodeToString(tinyWav())},
			"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
		})
		_, _ = w.Write(b)
	}))
	t.Cleanup(remote.Close)
	root = exportFixture(t)
	st := minimax.NewStore(root, func(string) string { return "" })
	ctl := httptest.NewServer(minimax.NewHandler(minimax.NewClient(minimax.Options{Store: st, Root: root}), fakeToken))
	t.Cleanup(ctl.Close)
	core := &Core{}
	core.Set(ctl.URL, fakeToken)
	var err error
	h, err = NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, "core-minimax,data-outputs,webui-generations"), Core: core})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL, root, remote, h
}

// TestMiniMaxRoutesNeverReturnTheKey calls every route and error path, then reads
// every answer and every file under the install root for the key.
func TestMiniMaxRoutesNeverReturnTheKey(t *testing.T) {
	front, root, remote, _ := miniMaxFront(t)
	var bodies []string
	call := func(method, path, body string, want int) string {
		t.Helper()
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		r := request(t, method, front+path, rd, jsonHdr)
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		bodies = append(bodies, method+" "+path+" -> "+strconv.Itoa(r.StatusCode)+" "+string(b))
		if r.StatusCode != want {
			t.Fatalf("%s %s: %d, want %d: %s", method, path, r.StatusCode, want, b)
		}
		return string(b)
	}
	const settings = "/api/go-data/minimax/settings"
	if s := call("GET", settings, "", 200); !strings.Contains(s, `"has_key":false`) || !strings.Contains(s, `"key_source":"none"`) {
		t.Fatalf("empty settings: %s", s)
	}
	call("POST", "/api/go-data/minimax/speech", `{"text":"x"}`, 400) // no key yet
	put := `{"base_url":"` + remote.URL + `","model":"speech-2.8-hd","voice":"Wise_Woman","key":"` + miniMaxSentinel + `"}`
	if s := call("PUT", settings, put, 200); !strings.Contains(s, `"has_key":true`) || !strings.Contains(s, `"key_source":"stored"`) {
		t.Fatalf("put: %s", s)
	}
	call("GET", settings, "", 200)
	if s := call("POST", "/api/go-data/minimax/test", "", 200); !strings.Contains(s, `"ok":true`) {
		t.Fatalf("test: %s", s)
	}
	res := call("POST", "/api/go-data/minimax/speech", `{"text":"Hello there"}`, 200)
	if !strings.Contains(res, `"wav":"outputs/`) || !strings.Contains(res, `"voice_id":"Wise_Woman"`) {
		t.Fatalf("speech: %s", res)
	}
	// A wrong key: the remote echoes it in its error; no answer may.
	call("PUT", settings, `{"base_url":"`+remote.URL+`","key":"mm-WRONG-key-1234"}`, 200)
	call("POST", "/api/go-data/minimax/speech", `{"text":"x"}`, 502)
	if s := call("POST", "/api/go-data/minimax/test", "", 200); !strings.Contains(s, `"ok":false`) || !strings.Contains(s, "refused the key") {
		t.Fatalf("test with a wrong key: %s", s)
	}
	// Bad input, unknown fields, a key sent in the speech body, a closed port.
	call("PUT", settings, `{"base_url":"ftp://a"}`, 400)
	call("PUT", settings, `{"base_url":"http://api.example.com","key":"`+miniMaxSentinel+`"}`, 400)
	call("PUT", settings, `{"extra":1,"key":"`+miniMaxSentinel+`"}`, 400)
	call("POST", "/api/go-data/minimax/speech", `{"text":"x","key":"`+miniMaxSentinel+`"}`, 400)
	call("POST", "/api/go-data/minimax/speech", `{"text":""}`, 400)
	call("PUT", settings, `{"base_url":"http://127.0.0.1:1","key":"`+miniMaxSentinel+`"}`, 200)
	call("POST", "/api/go-data/minimax/speech", `{"text":"x"}`, 502)
	call("POST", "/api/go-data/minimax/test", "", 200)
	// The pages that list generations.
	call("GET", "/api/go-data/outputs/outputs", "", 200)
	for _, b := range bodies {
		if strings.Contains(b, miniMaxSentinel) || strings.Contains(b, "mm-WRONG-key-1234") {
			t.Errorf("a key came back: %s", b)
		}
	}
	keyFile := filepath.Join(root, "platforms", "minimax", "key.json")
	if b, _ := os.ReadFile(keyFile); !strings.Contains(string(b), miniMaxSentinel) {
		t.Fatal("the key was not stored")
	}
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || p == keyFile || fi.Size() > 1<<20 {
			return nil
		}
		if b, _ := os.ReadFile(p); strings.Contains(string(b), miniMaxSentinel) {
			t.Errorf("the key is in %s", p)
		}
		return nil
	})
}

func TestMiniMaxRoutesRefuseAnotherDeviceAndAnotherSite(t *testing.T) {
	front, _, remote, h := miniMaxFront(t)
	put := `{"base_url":"` + remote.URL + `","key":"` + miniMaxSentinel + `"}`
	if r := request(t, "PUT", front+"/api/go-data/minimax/settings", strings.NewReader(put), jsonHdr); r.StatusCode != 200 {
		t.Fatalf("put: %d", r.StatusCode)
	}
	lan := func(method, path, body, peer, xff string) int {
		req := httptest.NewRequest(method, "http://127.0.0.1:"+strconv.Itoa(testPort)+path, strings.NewReader(body))
		req.RemoteAddr = peer
		req.Header.Set("Content-Type", "application/json")
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, peer := range []string{"192.168.1.20:5000", "[2001:db8::1]:5000"} {
		if c := lan("PUT", "/api/go-data/minimax/settings", put, peer, ""); c != 403 {
			t.Errorf("PUT from %s: %d", peer, c)
		}
		if c := lan("POST", "/api/go-data/minimax/test", "", peer, ""); c != 403 {
			t.Errorf("test from %s: %d", peer, c)
		}
		// Reading the settings (no key) and generating stay open to the LAN.
		if c := lan("GET", "/api/go-data/minimax/settings", "", peer, ""); c != 200 {
			t.Errorf("GET from %s: %d", peer, c)
		}
		if c := lan("POST", "/api/go-data/minimax/speech", `{"text":"hi"}`, peer, ""); c != 200 {
			t.Errorf("speech from %s: %d", peer, c)
		}
	}
	if c := lan("PUT", "/api/go-data/minimax/settings", put, "127.0.0.1:5000", "192.168.1.20"); c != 403 {
		t.Errorf("forwarded PUT: %d", c)
	}
	if c := lan("PUT", "/api/go-data/minimax/settings", put, "127.0.0.1:5000", ""); c != 200 {
		t.Errorf("local PUT: %d", c)
	}
	hdr := map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example"}
	if r := request(t, "PUT", front+"/api/go-data/minimax/settings", strings.NewReader(put), hdr); r.StatusCode != 403 {
		t.Errorf("cross-site PUT: %d", r.StatusCode)
	}
	if r := request(t, "PUT", front+"/api/go-data/minimax/settings", strings.NewReader(put), map[string]string{"Content-Type": "text/plain"}); r.StatusCode != 415 {
		t.Errorf("text/plain PUT: %d", r.StatusCode)
	}
	// The control API itself answers only a loopback peer with the token.
	ctl := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/minimax/settings", nil)
	req.RemoteAddr = "127.0.0.1:1"
	minimax.NewHandler(minimax.NewClient(minimax.Options{Store: minimax.NewStore(t.TempDir(), nil)}), "tok").ServeHTTP(ctl, req)
	if ctl.Code != 403 {
		t.Errorf("control API without the token: %d", ctl.Code)
	}
}

// The clip a generation writes is what the Vite History/Outputs pages read.
func TestMiniMaxClipIsListedByTheOutputsPage(t *testing.T) {
	front, _, remote, _ := miniMaxFront(t)
	put := `{"base_url":"` + remote.URL + `","key":"` + miniMaxSentinel + `"}`
	request(t, "PUT", front+"/api/go-data/minimax/settings", strings.NewReader(put), jsonHdr).Body.Close()
	r := request(t, "POST", front+"/api/go-data/minimax/speech", strings.NewReader(`{"text":"Hello there"}`), jsonHdr)
	var clip struct{ Wav string }
	_ = json.NewDecoder(r.Body).Decode(&clip)
	r.Body.Close()
	r = request(t, "GET", front+"/api/go-data/outputs/outputs", nil, nil)
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	var items []map[string]any
	if err := json.Unmarshal(b, &items); err != nil || len(items) != 1 {
		t.Fatalf("outputs: %s (%v)", b, err)
	}
	it := items[0]
	if it["_type"] != "minimax_cloud_tts" || it["voice_id"] != minimax.DefaultVoice || !strings.HasSuffix(it["filename"].(string), ".wav") {
		t.Fatalf("item %v", it)
	}
	// The player URL answers 200 audio/wav.
	r = request(t, "GET", front+"/"+it["filename"].(string), nil, nil)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "audio/") {
		t.Fatalf("player: %d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}
	if it["filename"] != "api/webui-generations/"+clip.Wav {
		t.Fatalf("clip %s vs item %v", clip.Wav, it["filename"])
	}
}
