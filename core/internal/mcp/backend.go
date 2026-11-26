package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Limits. They keep one tool call from using unbounded memory or time.
const (
	DefaultBackend  = "http://127.0.0.1:7778/v1"
	MaxTextChars    = 5000
	MaxAudioBytes   = 20 << 20
	maxModelsBytes  = 1 << 20
	maxErrorSnippet = 200
	requestTimeout  = 5 * time.Minute
)

// Backend is the OpenAI-compatible speech API the tools call. Its address is
// fixed at start; no tool argument can change it.
type Backend struct {
	base string
	hc   *http.Client
}

// NewBackend checks the base URL and returns a client for it.
func NewBackend(base string) (*Backend, error) {
	if base == "" {
		base = DefaultBackend
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("backend %q must be an http or https URL", base)
	}
	if u.User != nil {
		return nil, errors.New("backend URL must not contain credentials")
	}
	return &Backend{
		base: strings.TrimRight(base, "/"),
		hc: &http.Client{
			Timeout: requestTimeout,
			// An upstream must not bounce a call somewhere else.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

// Models returns the model ids the backend lists.
func (b *Backend) Models(ctx context.Context) ([]string, error) {
	data, err := b.do(ctx, http.MethodGet, "/models", nil, maxModelsBytes)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, errors.New("the backend's model list is not valid JSON")
	}
	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// SpeechRequest is what a client may choose. Everything else is fixed.
type SpeechRequest struct {
	Text  string
	Model string
	Voice string
	Speed float64
}

// Speak returns a WAV file for the request.
func (b *Backend) Speak(ctx context.Context, r SpeechRequest) ([]byte, error) {
	body := map[string]any{"input": r.Text, "response_format": "wav"}
	if r.Model != "" {
		body["model"] = r.Model
	}
	if r.Voice != "" {
		body["voice"] = r.Voice
	}
	if r.Speed > 0 && r.Speed != 1 {
		body["speed"] = r.Speed
	}
	payload, _ := json.Marshal(body)
	data, err := b.do(ctx, http.MethodPost, "/audio/speech", payload, MaxAudioBytes)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("the backend answered 200 but not with a WAV file; it must support response_format wav")
	}
	return data, nil
}

// do sends one request and reads at most limit bytes of the answer.
func (b *Backend) do(ctx context.Context, method, path string, payload []byte, limit int64) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.base+path, body)
	if err != nil {
		return nil, errors.New("could not build the backend request")
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("the backend is not reachable; is the OpenAI-compatible API running?")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, errors.New("reading the backend's answer failed")
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("the backend's answer is larger than the %d byte limit", limit)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the backend answered %d: %s", resp.StatusCode, snippet(data))
	}
	return data, nil
}

// snippet makes a short, single-line, printable excerpt of an error body.
func snippet(data []byte) string {
	s := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, string(data))
	s = strings.TrimSpace(s)
	if len(s) > maxErrorSnippet {
		s = s[:maxErrorSnippet] + "..."
	}
	return s
}
