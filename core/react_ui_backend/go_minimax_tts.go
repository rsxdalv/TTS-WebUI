package react_ui_backend

// MiniMax Cloud TTS (extension_minimax_cloud_tts) in Go. Same request to the
// MiniMax t2a_v2 API, same files: outputs/<timestamp>_<text>/<text>.mp3 with a
// .json next to it holding {text, model, voice_id, provider, _type, _version},
// the shape the History tab and the extension wrote before.
//
// The API key comes from the request, else the MINIMAX_API_KEY environment
// variable. It is sent to MiniMax only; it is never logged, stored or returned.
//
// The answer is {"audio": {name, data, url, orig_name}, "metadata": {...}} like
// the Gradio route; url is served by /api/webui-generations.

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "go-minimax-tts",
		Patterns: []string{"POST /api/go-data/minimax-tts"},
		Handler:  newMiniMaxTTS,
	})
}

// minimaxURL is the API endpoint; tests point it at a local server.
var minimaxURL = "https://api.minimax.io/v1/t2a_v2"

const maxMiniMaxText = 20000

type minimaxIn struct {
	Text    string `json:"text"`
	Model   string `json:"model"`
	VoiceID string `json:"voice_id"`
	APIKey  string `json:"api_key"`
}

func newMiniMaxTTS(deps Deps) http.Handler {
	lookup := envLookup(deps.Root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !requireJSON(w, r) {
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		var in minimaxIn
		// The page sends its whole parameter object (seed, use_random_seed, ...).
		if err := json.Unmarshal(body, &in); err != nil {
			writeAPIError(w, http.StatusBadRequest, "body must be a JSON object")
			return
		}
		in.Text = strings.TrimSpace(in.Text)
		key := strings.TrimSpace(in.APIKey)
		if key == "" {
			key = lookup("MINIMAX_API_KEY")
		}
		switch {
		case key == "":
			writeAPIError(w, http.StatusBadRequest, "MiniMax API key is required. Set MINIMAX_API_KEY or enter it in the API Key field.")
			return
		case in.Text == "" || len(in.Text) > maxMiniMaxText:
			writeAPIError(w, http.StatusBadRequest, "Please enter text to synthesize (at most 20000 bytes).")
			return
		case in.Model == "" || in.VoiceID == "" || len(in.Model) > 100 || len(in.VoiceID) > 100:
			writeAPIError(w, http.StatusBadRequest, "model and voice_id are required")
			return
		}
		audio, status, err := callMiniMax(r.Context(), key, in)
		if err != nil {
			writeAPIError(w, status, err.Error())
			return
		}
		folder, file, meta, err := saveMiniMax(deps.Root, audio, in)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "could not save the audio")
			return
		}
		link := (&url.URL{Path: "/api/webui-generations/outputs/" + folder + "/" + file}).EscapedPath()
		writeJSONStatus(w, http.StatusOK, map[string]any{
			"audio": map[string]any{
				"name": filepath.ToSlash(filepath.Join("outputs", folder, file)), "data": "", "url": link, "orig_name": file,
			},
			"metadata": meta,
		})
	})
}

// callMiniMax returns the mp3 bytes, or an error with the HTTP status to answer.
// Error texts never contain the key.
func callMiniMax(ctx context.Context, key string, in minimaxIn) ([]byte, int, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":         in.Model,
		"text":          in.Text,
		"voice_setting": map[string]any{"voice_id": in.VoiceID},
		"audio_setting": map[string]any{"format": "mp3"},
	})
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, minimaxURL, bytes.NewReader(payload))
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("bad request")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("Network error: could not reach MiniMax")
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode != http.StatusOK {
		snippet := string(data)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, http.StatusBadGateway, fmt.Errorf("MiniMax API error (%d): %s", resp.StatusCode, strings.ReplaceAll(snippet, key, "***"))
	}
	var result struct {
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("MiniMax API returned something that is not JSON")
	}
	if result.BaseResp.StatusCode != 0 {
		msg := result.BaseResp.StatusMsg
		if msg == "" {
			msg = "Unknown error"
		}
		return nil, http.StatusBadGateway, fmt.Errorf("MiniMax API error: %s", strings.ReplaceAll(msg, key, "***"))
	}
	audio, err := hex.DecodeString(result.Data.Audio)
	if err != nil || len(audio) == 0 {
		return nil, http.StatusBadGateway, fmt.Errorf("MiniMax API returned no audio data. Check your API key and request parameters.")
	}
	return audio, 0, nil
}

// safeTextName is the first 50 characters, letters, digits, space, "_" and "-"
// kept, spaces turned into "_".
func safeTextName(text string) string {
	var b strings.Builder
	n := 0
	for _, c := range text {
		if n == 50 {
			break
		}
		n++
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == ' ' || c == '_' || c == '-' {
			b.WriteRune(c)
		}
	}
	s := strings.ReplaceAll(strings.TrimSpace(b.String()), " ", "_")
	if s == "" || windowsUnsafeSegment(s+".mp3") {
		return "minimax_tts"
	}
	return s
}

func saveMiniMax(root string, audio []byte, in minimaxIn) (folder, file string, meta map[string]any, err error) {
	safe := safeTextName(in.Text)
	base := time.Now().Format("20060102_150405") + "_" + safe
	outputs := filepath.Join(root, "outputs")
	folder = base
	var dir string
	for i := 2; ; i++ {
		dir = filepath.Join(outputs, folder)
		if err = os.MkdirAll(outputs, 0o755); err != nil {
			return
		}
		if err = os.Mkdir(dir, 0o755); err == nil {
			break
		}
		if !os.IsExist(err) || i > 99 {
			return
		}
		folder = fmt.Sprintf("%s_%d", base, i)
	}
	file = safe + ".mp3"
	if err = os.WriteFile(filepath.Join(dir, file), audio, 0o644); err != nil {
		return
	}
	meta = map[string]any{
		"text": in.Text, "model": in.Model, "voice_id": in.VoiceID,
		"provider": "minimax", "_type": "minimax_cloud_tts", "_version": "0.1.0",
	}
	js, _ := json.MarshalIndent(meta, "", "  ")
	err = os.WriteFile(filepath.Join(dir, safe+".json"), js, 0o644)
	return
}
