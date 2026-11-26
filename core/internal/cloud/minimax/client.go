package minimax

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

const (
	// apiPath is the synchronous text-to-speech route (extension main.py: MINIMAX_API_URL).
	apiPath = "/v1/t2a_v2"
	// maxChars is the documented limit of the text field ("less than 10,000 characters").
	maxChars = 10000
	// defaultMaxResponse caps the JSON answer, which holds the audio as hex:
	// 128 MiB is 64 MiB of audio, about 17 minutes of 16-bit mono at 32 kHz.
	defaultMaxResponse = 128 << 20
	// audioFormat is what the call asks for. The Python extension asks for mp3;
	// the History page lists WAV, and the MiniMax reference lists wav among the
	// formats. See documentation/core-rewrite/EXT-MINIMAX.md.
	audioFormat = "wav"
)

// Options configure a Client.
type Options struct {
	Store *Store
	// Root is the install folder; clips go to <Root>/outputs.
	Root string
	// HTTP overrides the HTTP client (tests). Redirects are never followed.
	HTTP *http.Client
	// Now overrides the clock (tests).
	Now func() time.Time
	// Logf receives one line per call (the outcome, never the key).
	Logf func(format string, args ...any)
	// MaxResponse overrides the answer size cap in bytes (tests).
	MaxResponse int64
}

// Client calls MiniMax with the settings and the key of a Store.
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
	if o.MaxResponse <= 0 {
		o.MaxResponse = defaultMaxResponse
	}
	return &Client{o: o, http: openai.NoRedirectClient(o.HTTP)}
}

// SpeechRequest is the body of a generation. Empty Model and VoiceID use the settings'.
type SpeechRequest struct {
	Text    string `json:"text"`
	Model   string `json:"model"`
	VoiceID string `json:"voice_id"`
}

// Result is where a generated clip went.
type Result struct {
	Folder  string  `json:"folder"`
	Wav     string  `json:"wav"`
	PNG     string  `json:"png"`
	JSON    string  `json:"json"`
	Seconds float64 `json:"seconds"`
	Bytes   int     `json:"bytes"`
	Text    string  `json:"text"`
	Model   string  `json:"model"`
	VoiceID string  `json:"voice_id"`
}

// Speak sends the text to MiniMax, checks that a WAV came back and saves it in
// outputs/ in the History layout.
func (c *Client) Speak(ctx context.Context, req SpeechRequest) (Result, error) {
	set, key, err := c.o.Store.credentials()
	if err != nil {
		return Result{}, err
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return Result{}, openai.Invalidf("the text is empty")
	}
	if utf8.RuneCountInString(text) >= maxChars {
		return Result{}, openai.Invalidf("the text must be less than %d characters (MiniMax's limit)", maxChars)
	}
	model := first(strings.TrimSpace(req.Model), set.Model)
	voice := first(strings.TrimSpace(req.VoiceID), set.Voice)
	if len(model) > 200 || len(voice) > 200 || strings.ContainsAny(model+voice, "\r\n\x00") {
		return Result{}, openai.Invalidf("model and voice are single lines of at most 200 characters")
	}
	if key == "" {
		return Result{}, noKeyError()
	}
	wav, secs, trace, err := c.fetch(ctx, set, key, model, voice, text)
	if err != nil {
		c.o.Logf("minimax: generate failed: %s", err)
		return Result{}, err
	}
	sv, _, err := openai.SaveClipFields(c.o.Root, c.o.Now(), "minimax", text, "0.1.0", []openai.Field{
		{Key: "model", Value: model}, {Key: "voice_id", Value: voice},
		{Key: "provider", Value: "minimax"}, {Key: "_type", Value: "minimax_cloud_tts"},
		{Key: "response_format", Value: audioFormat},
	}, wav)
	if err != nil {
		return Result{}, fmt.Errorf("saving the clip in outputs/: %w", err)
	}
	c.o.Logf("minimax: saved %s (%.1f s, trace %s)", sv.Folder, secs, trace)
	return Result{Folder: sv.Folder, Wav: sv.Wav, PNG: sv.PNG, JSON: sv.JSON, Seconds: secs, Bytes: len(wav),
		Text: text, Model: model, VoiceID: voice}, nil
}

// answer is the part of the JSON answer this code reads (see EXT-MINIMAX.md).
type answer struct {
	Data struct {
		Audio string `json:"audio"`
	} `json:"data"`
	TraceID  string `json:"trace_id"`
	BaseResp *struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// fetch makes the call and validates the answer: the JSON, base_resp, the hex,
// the WAV. It returns the WAV, its length in seconds and MiniMax's trace id.
func (c *Client) fetch(ctx context.Context, set Settings, key, model, voice, text string) ([]byte, float64, string, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":         model,
		"text":          text,
		"voice_setting": map[string]any{"voice_id": voice},
		"audio_setting": map[string]any{"format": audioFormat},
	})
	status, data, err := c.do(ctx, set, key, payload)
	if err != nil {
		return nil, 0, "", err
	}
	var a answer
	jsonErr := json.Unmarshal(data, &a)
	if status != http.StatusOK {
		return nil, 0, "", statusError(set, key, status, data, a)
	}
	if jsonErr != nil {
		return nil, 0, "", &openai.UpstreamError{Msg: openai.Scrub(key, "MiniMax answered 200 with something that is not JSON: "+openai.Snippet(data))}
	}
	if a.BaseResp != nil && a.BaseResp.StatusCode != 0 {
		return nil, 0, a.TraceID, baseRespError(key, a)
	}
	if a.Data.Audio == "" {
		return nil, 0, a.TraceID, &openai.UpstreamError{Msg: "MiniMax answered without audio data" + traceSuffix(a.TraceID) + ": check the model and the voice"}
	}
	wav, err := hex.DecodeString(a.Data.Audio)
	if err != nil {
		return nil, 0, a.TraceID, &openai.UpstreamError{Msg: "MiniMax sent audio that is not valid hex" + traceSuffix(a.TraceID)}
	}
	secs, err := openai.CheckWav(wav)
	if err != nil {
		if errors.Is(err, openai.ErrNotWav) {
			return nil, 0, a.TraceID, &openai.UpstreamError{Msg: "MiniMax sent audio that is not a WAV file (" + describeAudio(wav) +
				") although wav was requested" + traceSuffix(a.TraceID)}
		}
		return nil, 0, a.TraceID, &openai.UpstreamError{Msg: "MiniMax sent a WAV file that cannot be read: " + err.Error() + traceSuffix(a.TraceID)}
	}
	return wav, secs, a.TraceID, nil
}

// do sends the request and reads the whole answer (capped).
// Never run against the real MiniMax service: see "Not verified" in EXT-MINIMAX.md.
func (c *Client) do(ctx context.Context, set Settings, key string, payload []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, set.BaseURL+apiPath, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, openai.Invalidf("bad base URL")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tts-webui-core")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, &openai.UpstreamError{Msg: openai.Scrub(key, openai.Unreachable(Host(set.BaseURL), err))}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.o.MaxResponse+1))
	if err != nil {
		return 0, nil, &openai.UpstreamError{Msg: openai.Scrub(key, "the connection to "+Host(set.BaseURL)+" broke while the answer was read: "+openai.ShortErr(err))}
	}
	if int64(len(data)) > c.o.MaxResponse {
		return 0, nil, &openai.UpstreamError{Msg: fmt.Sprintf("the answer is larger than %d MB", c.o.MaxResponse>>20)}
	}
	return resp.StatusCode, data, nil
}

func traceSuffix(id string) string {
	if id == "" {
		return ""
	}
	return " (trace id " + openai.Truncate(id) + ")"
}

// describeAudio names what a non-WAV answer looks like, for the message.
func describeAudio(b []byte) string {
	switch {
	case len(b) == 0:
		return "empty"
	case bytes.HasPrefix(b, []byte("ID3")), len(b) > 1 && b[0] == 0xFF && b[1]&0xE0 == 0xE0:
		return "it looks like MP3"
	case bytes.HasPrefix(b, []byte("fLaC")):
		return "it looks like FLAC"
	case bytes.HasPrefix(b, []byte("OggS")):
		return "it looks like Ogg"
	}
	return "unknown format"
}

// baseRespError words a non-zero base_resp.status_code. The codes are the ones
// the MiniMax reference lists for this route; any other code is shown as it came.
func baseRespError(key string, a answer) error {
	code, msg := a.BaseResp.StatusCode, openai.Truncate(a.BaseResp.StatusMsg)
	var what string
	switch code {
	case 1004:
		what = "MiniMax refused the key (1004, authentication failed): check the API key"
	case 1002, 1039:
		what = fmt.Sprintf("MiniMax says the rate limit was exceeded (%d): try again later", code)
	case 1001:
		what = "MiniMax timed out (1001)"
	case 1042:
		what = "MiniMax refused the text: more than 10% invalid characters (1042)"
	case 2013:
		what = "MiniMax refused the request parameters (2013): check the model and the voice"
	default:
		what = fmt.Sprintf("MiniMax error %d", code)
	}
	if msg != "" {
		what += ": " + msg
	}
	return &openai.UpstreamError{Msg: openai.Scrub(key, what+traceSuffix(a.TraceID))}
}

// statusError words a non-200 HTTP answer.
func statusError(set Settings, key string, status int, data []byte, a answer) error {
	if a.BaseResp != nil && a.BaseResp.StatusCode != 0 {
		return baseRespError(key, a)
	}
	var what string
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		what = fmt.Sprintf("MiniMax refused the key (HTTP %d): check the API key", status)
	case status == http.StatusNotFound:
		what = fmt.Sprintf("HTTP 404 for %s%s: check the base URL", set.BaseURL, apiPath)
	case status == http.StatusTooManyRequests:
		what = "MiniMax says too many requests or no quota left (HTTP 429)"
	case status >= 300 && status < 400:
		what = fmt.Sprintf("the server redirects to another address (HTTP %d): use the address it redirects to as the base URL", status)
	case status >= 500:
		what = fmt.Sprintf("MiniMax failed (HTTP %d)", status)
	default:
		what = fmt.Sprintf("MiniMax refused the request (HTTP %d)", status)
	}
	if s := openai.Snippet(data); s != "" {
		what += ": " + openai.Truncate(s)
	}
	return &openai.UpstreamError{Msg: openai.Scrub(key, what)}
}

// TestResult is the answer of a test call.
type TestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Test makes the same call a generation makes, for one word, and saves nothing:
// the base URL, the key, the model and the voice are all checked. A failure of
// MiniMax's side is a TestResult with OK false, not an error. It uses a little
// of the account's quota.
func (c *Client) Test(ctx context.Context) (TestResult, error) {
	set, key, err := c.o.Store.credentials()
	if err != nil {
		return TestResult{}, err
	}
	if key == "" {
		return TestResult{Message: noKeyMsg}, nil
	}
	_, secs, _, err := c.fetch(ctx, set, key, set.Model, set.Voice, "Test.")
	if err != nil {
		var ue *openai.UpstreamError
		if errors.As(err, &ue) {
			c.o.Logf("minimax: test failed")
			return TestResult{Message: ue.Msg}, nil
		}
		return TestResult{}, err
	}
	c.o.Logf("minimax: test ok")
	return TestResult{OK: true, Message: fmt.Sprintf("MiniMax answered a speech request with %.1f s of audio.", secs)}, nil
}
