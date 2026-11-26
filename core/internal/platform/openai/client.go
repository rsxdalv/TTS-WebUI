package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxClip caps a clip read from an endpoint.
const maxClip = 256 << 20

// UpstreamError is a failure on the endpoint's side or on the way to it. Its
// text is what the user is shown: the key has been removed from it.
type UpstreamError struct{ Msg string }

func (e *UpstreamError) Error() string { return e.Msg }

// Options configure a Client.
type Options struct {
	Store *Store
	// Root is the install folder; clips go to <Root>/outputs.
	Root string
	// HTTP overrides the HTTP client (tests). Redirects are never followed.
	HTTP *http.Client
	// Now overrides the clock (tests).
	Now func() time.Time
	// Logf receives one line per call (endpoint name and outcome, never the key).
	Logf func(format string, args ...any)
}

// Client calls the endpoints of a Store.
type Client struct {
	o    Options
	http *http.Client
}

// NewClient returns a Client.
func NewClient(o Options) *Client {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	return &Client{o: o, http: noRedirect(o.HTTP)}
}

// noRedirect is hc (or the default client) with redirects reported, never followed:
// a redirect would carry the request (and a key) somewhere the user did not enter.
func noRedirect(hc *http.Client) *http.Client {
	if hc == nil {
		hc = &http.Client{
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		}
	}
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// SpeechRequest is the body of a generation. Empty Model and Voice use the
// endpoint's own.
type SpeechRequest struct {
	Endpoint string  `json:"endpoint"`
	Input    string  `json:"input"`
	Model    string  `json:"model"`
	Voice    string  `json:"voice"`
	Speed    float64 `json:"speed"`
}

// Result is where a generated clip went.
type Result struct {
	Folder  string  `json:"folder"`
	Wav     string  `json:"wav"`
	PNG     string  `json:"png"`
	JSON    string  `json:"json"`
	Seconds float64 `json:"seconds"`
	Bytes   int     `json:"bytes"`
}

// Speak sends the text to the endpoint, checks that a WAV came back and saves
// it in outputs/.
func (c *Client) Speak(ctx context.Context, req SpeechRequest) (Result, error) {
	ep, key, err := c.o.Store.get(req.Endpoint)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(req.Input) == "" {
		return Result{}, invalidf("the text is empty")
	}
	if len(req.Input) > 100000 {
		return Result{}, invalidf("the text is longer than 100000 characters")
	}
	if req.Speed < 0 || req.Speed > 4 {
		return Result{}, invalidf("the speed must be between 0.25 and 4 (0 leaves it to the endpoint)")
	}
	model := firstNonEmpty(strings.TrimSpace(req.Model), ep.Model)
	voice := firstNonEmpty(strings.TrimSpace(req.Voice), ep.Voice)
	if model == "" || voice == "" {
		return Result{}, invalidf("set a model and a voice (for example tts-1 and alloy) on the endpoint or in the form")
	}
	wavBytes, w, err := c.fetchWav(ctx, ep, key, model, voice, req.Input, req.Speed)
	if err != nil {
		c.o.Logf("openai %s: generate failed: %s", ep.Name, err)
		return Result{}, err
	}
	sv, err := saveGeneration(c.o.Root, c.o.Now(), meta{
		Text: req.Input, Endpoint: ep.Name, Model: model, Voice: voice, Speed: req.Speed, ResponseFormat: "wav",
	}, wavBytes, w)
	if err != nil {
		return Result{}, fmt.Errorf("saving the clip in outputs/: %w", err)
	}
	c.o.Logf("openai %s: saved %s (%.1f s)", ep.Name, sv.Folder, w.seconds())
	return Result{Folder: sv.Folder, Wav: sv.Wav, PNG: sv.PNG, JSON: sv.JSON, Seconds: w.seconds(), Bytes: len(wavBytes)}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// fetchWav makes the call and validates the answer.
func (c *Client) fetchWav(ctx context.Context, ep Endpoint, key, model, voice, input string, speed float64) ([]byte, wavInfo, error) {
	body := map[string]any{"model": model, "voice": voice, "input": input, "response_format": "wav"}
	if speed > 0 && speed != 1 {
		body["speed"] = speed
	}
	payload, _ := json.Marshal(body)
	status, ctype, data, err := c.do(ctx, ep, key, http.MethodPost, "/audio/speech", payload)
	if err != nil {
		return nil, wavInfo{}, err
	}
	if status != http.StatusOK {
		return nil, wavInfo{}, upstreamStatus(ep, key, "/audio/speech", status, data)
	}
	w, err := parseWav(data)
	if err != nil {
		if errors.Is(err, errNotWav) {
			return nil, wavInfo{}, &UpstreamError{Msg: scrub(key, fmt.Sprintf(
				"the endpoint answered 200 with %s, not a WAV file (%s). It must support response_format wav%s",
				describeType(ctype), snippet(data), ""))}
		}
		return nil, wavInfo{}, &UpstreamError{Msg: scrub(key, "the endpoint sent a WAV file that cannot be read: "+err.Error())}
	}
	if w.frames() == 0 {
		return nil, wavInfo{}, &UpstreamError{Msg: "the endpoint sent a WAV file with no audio in it"}
	}
	return data, w, nil
}

// do sends one request and reads the whole answer (capped).
// Never run against a real third-party server: see "Not verified: real remote endpoint" in INTEGRATION.md.
func (c *Client) do(ctx context.Context, ep Endpoint, key, method, path string, payload []byte) (int, string, []byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, ep.URL+path, body)
	if err != nil {
		return 0, "", nil, invalidf("bad endpoint URL")
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "audio/wav, application/json")
	req.Header.Set("User-Agent", "tts-webui-core")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", nil, &UpstreamError{Msg: scrub(key, unreachable(ep, err))}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxClip+1))
	if err != nil {
		return 0, "", nil, &UpstreamError{Msg: scrub(key, "the connection to "+hostOf(ep)+" broke while the answer was read: "+shortErr(err))}
	}
	if len(data) > maxClip {
		return 0, "", nil, &UpstreamError{Msg: "the answer is larger than 256 MB"}
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), data, nil
}

func hostOf(ep Endpoint) string {
	if u, err := url.Parse(ep.URL); err == nil {
		return u.Host
	}
	return ep.URL
}

// unreachable words a failure before any answer.
func unreachable(ep Endpoint, err error) string {
	var dns *net.DNSError
	switch {
	case errors.As(err, &dns):
		return fmt.Sprintf("the host %q was not found: check the URL of the endpoint", dns.Name)
	case errors.Is(err, context.DeadlineExceeded):
		return "no answer from " + hostOf(ep) + " in time"
	case errors.Is(err, context.Canceled):
		return "the request was cancelled"
	}
	return fmt.Sprintf("cannot reach %s: %s", hostOf(ep), shortErr(err))
}

// shortErr drops the URL that net/http puts in front of the cause.
func shortErr(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	s := err.Error()
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// upstreamStatus words a non-200 answer.
func upstreamStatus(ep Endpoint, key, path string, status int, data []byte) error {
	msg := remoteMessage(data)
	var what string
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		if key == "" {
			what = fmt.Sprintf("the endpoint wants a key (HTTP %d) and none is stored: add one to the endpoint", status)
		} else {
			what = fmt.Sprintf("the endpoint refused the key (HTTP %d): check the key of the endpoint", status)
		}
	case status == http.StatusNotFound:
		what = fmt.Sprintf("HTTP 404 for %s%s: check the URL (the base normally ends in /v1) and the model name", ep.URL, path)
	case status == http.StatusTooManyRequests:
		what = "the endpoint says too many requests or no quota left (HTTP 429)"
	case status >= 300 && status < 400:
		what = fmt.Sprintf("the endpoint redirects to another address (HTTP %d): use the address it redirects to as the URL", status)
	case status >= 500:
		what = fmt.Sprintf("the endpoint failed (HTTP %d)", status)
	default:
		what = fmt.Sprintf("the endpoint refused the request (HTTP %d)", status)
	}
	if msg != "" {
		what += ": " + msg
	}
	return &UpstreamError{Msg: scrub(key, what)}
}

// remoteMessage is the error text an OpenAI-style (or FastAPI) body carries.
func remoteMessage(data []byte) string {
	var e struct {
		Error  json.RawMessage `json:"error"`
		Detail json.RawMessage `json:"detail"`
		Msg    string          `json:"message"`
	}
	if json.Unmarshal(data, &e) == nil {
		var s struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Error, &s) == nil && s.Message != "" {
			return truncate(s.Message)
		}
		var str string
		if json.Unmarshal(e.Error, &str) == nil && str != "" {
			return truncate(str)
		}
		if json.Unmarshal(e.Detail, &str) == nil && str != "" {
			return truncate(str)
		}
		if e.Msg != "" {
			return truncate(e.Msg)
		}
	}
	return snippet(data)
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// snippet is the start of a body, when it is text.
func snippet(data []byte) string {
	if len(data) > 200 {
		data = data[:200]
	}
	for _, b := range data {
		if b < 0x09 || (b > 0x0d && b < 0x20) {
			return "binary data"
		}
	}
	return strings.Join(strings.Fields(string(data)), " ")
}

func describeType(ct string) string {
	if ct == "" {
		return "no content type"
	}
	return ct
}

// scrub removes the key from a message that is going to be shown or logged.
func scrub(key, s string) string {
	if len(key) >= 4 {
		s = strings.ReplaceAll(s, key, "[key]")
	}
	return s
}

// TestResult is the answer of a test call.
type TestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Test calls the endpoint once. With a model and a voice stored it makes a
// real speech request for one word, the same call a generation makes, so the
// URL, the key, the model and the voice are all checked; without them it asks
// for the model list, which checks the URL and the key.
func (c *Client) Test(ctx context.Context, name string) (TestResult, error) {
	ep, key, err := c.o.Store.get(name)
	if err != nil {
		return TestResult{}, err
	}
	if ep.Model != "" && ep.Voice != "" {
		_, w, err := c.fetchWav(ctx, ep, key, ep.Model, ep.Voice, "Test.", 0)
		if err != nil {
			var ue *UpstreamError
			if errors.As(err, &ue) {
				c.o.Logf("openai %s: test failed", ep.Name)
				return TestResult{Message: ue.Msg}, nil
			}
			return TestResult{}, err
		}
		c.o.Logf("openai %s: test ok", ep.Name)
		return TestResult{OK: true, Message: fmt.Sprintf("The endpoint answered a speech request with %.1f s of audio.", w.seconds())}, nil
	}
	status, _, data, err := c.do(ctx, ep, key, http.MethodGet, "/models", nil)
	if err != nil {
		var ue *UpstreamError
		if errors.As(err, &ue) {
			return TestResult{Message: ue.Msg}, nil
		}
		return TestResult{}, err
	}
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented {
		return TestResult{Message: fmt.Sprintf("The endpoint is there but does not list models (HTTP %d). Set a model and a voice on the endpoint to test with a speech request.", status)}, nil
	}
	if status != http.StatusOK {
		var ue *UpstreamError
		errors.As(upstreamStatus(ep, key, "/models", status, data), &ue)
		return TestResult{Message: ue.Msg}, nil
	}
	return TestResult{OK: true, Message: "The endpoint answered the model list. Set a model and a voice on the endpoint to test a speech request."}, nil
}
