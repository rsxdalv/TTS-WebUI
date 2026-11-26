package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func fakeWav() []byte {
	return append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 24)...)
}

// connect runs the server against a fake backend and returns a client session.
func connect(t *testing.T, backend http.Handler) *sdk.ClientSession {
	t.Helper()
	ts := httptest.NewServer(backend)
	t.Cleanup(ts.Close)
	b, err := NewBackend(ts.URL + "/v1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ct, st := sdk.NewInMemoryTransports()
	ss, err := New(b).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) *sdk.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func firstText(r *sdk.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestListTools(t *testing.T) {
	cs := connect(t, http.NotFoundHandler())
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "generate_speech,list_models" && strings.Join(names, ",") != "list_models,generate_speech" {
		t.Fatalf("tools = %v", names)
	}
}

func TestListModels(t *testing.T) {
	cs := connect(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"kokoro"},{"id":"piper"}]}`))
	}))
	if got := firstText(call(t, cs, "list_models", nil)); got != "kokoro\npiper" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateSpeech(t *testing.T) {
	var gotBody string
	cs := connect(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write(fakeWav())
	}))
	res := call(t, cs, "generate_speech", map[string]any{"text": "hello", "voice": "af"})
	if res.IsError {
		t.Fatalf("error: %s", firstText(res))
	}
	audio, ok := res.Content[0].(*sdk.AudioContent)
	if !ok || audio.MIMEType != "audio/wav" || len(audio.Data) != len(fakeWav()) {
		t.Fatalf("content = %#v", res.Content[0])
	}
	if !strings.Contains(gotBody, `"input":"hello"`) || !strings.Contains(gotBody, `"voice":"af"`) || !strings.Contains(gotBody, `"response_format":"wav"`) {
		t.Fatalf("body = %s", gotBody)
	}
}

func TestGenerateSpeechErrors(t *testing.T) {
	huge := strings.Repeat("a", MaxTextChars+1)
	cases := []struct {
		name    string
		backend http.HandlerFunc
		args    map[string]any
		want    string
	}{
		{"empty text", nil, map[string]any{"text": "  "}, "text is required"},
		{"too long", nil, map[string]any{"text": huge}, "limit is"},
		{"not wav", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) }, map[string]any{"text": "x"}, "not with a WAV"},
		{"upstream 500", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", 500) }, map[string]any{"text": "x"}, "answered 500"},
		{"too big", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, MaxAudioBytes+1))
		}, map[string]any{"text": "x"}, "larger than"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := c.backend
			if h == nil {
				h = func(http.ResponseWriter, *http.Request) { t.Error("backend must not be called") }
			}
			res := call(t, connect(t, h), "generate_speech", c.args)
			if !res.IsError || !strings.Contains(firstText(res), c.want) {
				t.Fatalf("isError=%v text=%q, want %q", res.IsError, firstText(res), c.want)
			}
		})
	}
}

func TestBackendDown(t *testing.T) {
	b, _ := NewBackend("http://127.0.0.1:1/v1")
	if _, err := b.Models(context.Background()); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("err = %v", err)
	}
}

func TestCancellationReachesBackend(t *testing.T) {
	released := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body) // lets the server notice a closed connection
		<-r.Context().Done()
		close(released)
	}))
	defer ts.Close()
	b, _ := NewBackend(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	if _, err := b.Speak(ctx, SpeechRequest{Text: "x"}); err == nil {
		t.Fatal("expected an error")
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("backend request was not cancelled")
	}
}

func TestNewBackendValidation(t *testing.T) {
	for _, bad := range []string{"ftp://x", "http://", "http://u:p@host/v1", "::"} {
		if _, err := NewBackend(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := NewBackend(""); err != nil {
		t.Errorf("default rejected: %v", err)
	}
}

func TestCheckListen(t *testing.T) {
	tok := strings.Repeat("t", MinTokenLen)
	for addr, ok := range map[string]bool{
		"127.0.0.1:7779": true, "localhost:7779": true, "[::1]:7779": true,
		"0.0.0.0:7779": false, "192.168.1.5:7779": false, ":7779": false, "example.com:1": false, "nonsense": false,
	} {
		if err := CheckListen(addr, tok); (err == nil) != ok {
			t.Errorf("%s: err=%v, want ok=%v", addr, err, ok)
		}
	}
	if err := CheckListen("127.0.0.1:7779", "short"); err == nil {
		t.Error("short token accepted")
	}
}

func TestHTTPAuthAndOrigin(t *testing.T) {
	b, _ := NewBackend("")
	token := strings.Repeat("s", MinTokenLen)
	ts := httptest.NewServer(Handler(New(b), token))
	defer ts.Close()

	do := func(hdr map[string]string) int {
		req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := do(nil); got != http.StatusUnauthorized {
		t.Errorf("no token: %d", got)
	}
	if got := do(map[string]string{"Authorization": "Bearer wrong-wrong-wrong"}); got != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", got)
	}
	if got := do(map[string]string{"Authorization": "Bearer " + token, "Origin": "http://evil.example"}); got != http.StatusForbidden {
		t.Errorf("bad origin: %d", got)
	}
	// A rebound host name (the test server listens on loopback).
	req, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(`{}`))
	req.Host = "evil.example"
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("bad host: %d", resp.StatusCode)
	}
}
