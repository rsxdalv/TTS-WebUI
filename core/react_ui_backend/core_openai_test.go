package react_ui_backend

import (
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

const sentinelKey = "sk-SENTINEL-9f8e7d6c5b4a"

func tinyWav() []byte {
	data := make([]byte, 3200)
	for i := range data {
		data[i] = byte(i * 7)
	}
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(data)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], 16000)
	binary.LittleEndian.PutUint32(h[28:], 32000)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(data)))
	return append(h, data...)
}

// openaiFront builds the real chain: front door, claim, control API, store and a
// fake remote endpoint that echoes the key it was sent in its error text.
func openaiFront(t *testing.T) (front, root string, remote *httptest.Server, h http.Handler) {
	t.Helper()
	remote = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+sentinelKey {
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"error":{"message":"bad key `+strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")+`"}}`)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(tinyWav())
	}))
	t.Cleanup(remote.Close)
	root = exportFixture(t)
	st := openai.NewStore(root)
	ctl := httptest.NewServer(openai.NewHandler(openai.NewClient(openai.Options{Store: st, Root: root}), fakeToken))
	t.Cleanup(ctl.Close)
	core := &Core{}
	core.Set(ctl.URL, fakeToken)
	var err error
	h, err = NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, "core-openai"), Core: core})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL, root, remote, h
}

func TestOpenAIRoutesNeverReturnTheKey(t *testing.T) {
	front, root, remote, _ := openaiFront(t)
	put := `{"url":"` + remote.URL + `/v1","model":"tts-1","voice":"alloy","key":"` + sentinelKey + `"}`
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
	call("PUT", "/api/go-data/openai/endpoints/main", put, 200)
	call("GET", "/api/go-data/openai/endpoints", "", 200)
	call("POST", "/api/go-data/openai/endpoints/main/test", "", 200)
	res := call("POST", "/api/go-data/openai/speech", `{"endpoint":"main","input":"Hello there"}`, 200)
	if !strings.Contains(res, `"wav":"outputs/`) {
		t.Fatalf("speech: %s", res)
	}
	// A wrong key: the remote echoes it in its error; the answer must not.
	call("PUT", "/api/go-data/openai/endpoints/bad", `{"url":"`+remote.URL+`/v1","model":"m","voice":"v","key":"sk-WRONG-key-1234"}`, 200)
	call("POST", "/api/go-data/openai/speech", `{"endpoint":"bad","input":"x"}`, 502)
	test := call("POST", "/api/go-data/openai/endpoints/bad/test", "", 200)
	if !strings.Contains(test, `"ok":false`) || !strings.Contains(test, "refused the key") {
		t.Fatalf("test: %s", test)
	}
	// Bad input and unknown names.
	call("PUT", "/api/go-data/openai/endpoints/x", `{"url":"ftp://a"}`, 400)
	call("POST", "/api/go-data/openai/speech", `{"endpoint":"nope","input":"x"}`, 404)
	call("PUT", "/api/go-data/openai/endpoints/main", `{"url":"`+remote.URL+`/v1","key":"`+sentinelKey+`","extra":1}`, 400)
	for _, b := range bodies {
		if strings.Contains(b, sentinelKey) || strings.Contains(b, "sk-WRONG-key-1234") {
			t.Errorf("a key came back: %s", b)
		}
	}
	// No file but keys.json holds the key, and nothing under outputs/.
	keysFile := filepath.Join(root, "platforms", "openai", "keys.json")
	if b, _ := os.ReadFile(keysFile); !strings.Contains(string(b), sentinelKey) {
		t.Fatal("the key was not stored")
	}
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || p == keysFile || fi.Size() > 1<<20 {
			return nil
		}
		if b, _ := os.ReadFile(p); strings.Contains(string(b), sentinelKey) {
			t.Errorf("the key is in %s", p)
		}
		return nil
	})
}

func TestOpenAIRoutesRefuseAnotherDeviceAndAnotherSite(t *testing.T) {
	front, _, remote, h := openaiFront(t)
	put := `{"url":"` + remote.URL + `/v1","model":"tts-1","voice":"alloy","key":"` + sentinelKey + `"}`
	if r := request(t, "PUT", front+"/api/go-data/openai/endpoints/main", strings.NewReader(put), jsonHdr); r.StatusCode != 200 {
		t.Fatalf("put: %d", r.StatusCode)
	}
	// Another device: a peer address that is not loopback, or a request a proxy forwarded.
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
		if c := lan("PUT", "/api/go-data/openai/endpoints/main", put, peer, ""); c != 403 {
			t.Errorf("PUT from %s: %d", peer, c)
		}
		if c := lan("DELETE", "/api/go-data/openai/endpoints/main", "", peer, ""); c != 403 {
			t.Errorf("DELETE from %s: %d", peer, c)
		}
		if c := lan("POST", "/api/go-data/openai/endpoints/main/test", "", peer, ""); c != 403 {
			t.Errorf("test from %s: %d", peer, c)
		}
		// Listing and generating are allowed.
		if c := lan("GET", "/api/go-data/openai/endpoints", "", peer, ""); c != 200 {
			t.Errorf("GET from %s: %d", peer, c)
		}
		if c := lan("POST", "/api/go-data/openai/speech", `{"endpoint":"main","input":"hi"}`, peer, ""); c != 200 {
			t.Errorf("speech from %s: %d", peer, c)
		}
	}
	if c := lan("PUT", "/api/go-data/openai/endpoints/main", put, "127.0.0.1:5000", "192.168.1.20"); c != 403 {
		t.Errorf("forwarded PUT: %d", c)
	}
	if c := lan("PUT", "/api/go-data/openai/endpoints/main", put, "127.0.0.1:5000", ""); c != 200 {
		t.Errorf("local PUT: %d", c)
	}
	// Another site in a browser on this machine: refused by the front door's Origin check.
	hdr := map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example"}
	if r := request(t, "PUT", front+"/api/go-data/openai/endpoints/main", strings.NewReader(put), hdr); r.StatusCode != 403 {
		t.Errorf("cross-site PUT: %d", r.StatusCode)
	}
	if r := request(t, "PUT", front+"/api/go-data/openai/endpoints/main", strings.NewReader(put), map[string]string{"Content-Type": "text/plain"}); r.StatusCode != 415 {
		t.Errorf("text/plain PUT: %d", r.StatusCode)
	}
	if r := request(t, "PUT", front+"/api/go-data/openai/endpoints/a..b", strings.NewReader(put), jsonHdr); r.StatusCode == 200 {
		t.Errorf("bad name accepted")
	}
}
