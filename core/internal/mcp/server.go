// Package mcp is the Model Context Protocol server: a thin adapter from MCP
// tools to the OpenAI-compatible speech API. See documentation/core-rewrite/MCP.md.
package mcp

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rsxdalv/TTS-WebUI/core/internal/version"
)

// Bounds on the free-form inputs (the speed range is the one the OpenAI API documents).
const (
	maxNameLen = 200
	minSpeed   = 0.25
	maxSpeed   = 4.0
)

type speechArgs struct {
	Text  string  `json:"text" jsonschema:"the text to speak (at most 5000 characters)"`
	Model string  `json:"model,omitempty" jsonschema:"model id from list_models; the backend default if empty"`
	Voice string  `json:"voice,omitempty" jsonschema:"voice name; the backend default if empty"`
	Speed float64 `json:"speed,omitempty" jsonschema:"speaking speed from 0.25 to 4, 1 is normal"`
}

type noArgs struct{}

// New builds the server with its tools.
func New(b *Backend) *sdk.Server {
	s := sdk.NewServer(&sdk.Implementation{Name: "tts-webui", Version: version.String()}, nil)

	sdk.AddTool(s, &sdk.Tool{
		Name:        "list_models",
		Description: "List the text-to-speech model ids the backend offers.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ noArgs) (*sdk.CallToolResult, any, error) {
		ids, err := b.Models(ctx)
		if err != nil {
			return failure(err), nil, nil
		}
		if len(ids) == 0 {
			return text("The backend lists no models."), nil, nil
		}
		return text(strings.Join(ids, "\n")), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "generate_speech",
		Description: "Turn text into speech. Returns a WAV audio clip.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, a speechArgs) (*sdk.CallToolResult, any, error) {
		if strings.TrimSpace(a.Text) == "" {
			return failure(fmt.Errorf("text is required")), nil, nil
		}
		if n := utf8.RuneCountInString(a.Text); n > MaxTextChars {
			return failure(fmt.Errorf("text is %d characters; the limit is %d", n, MaxTextChars)), nil, nil
		}
		if len(a.Model) > maxNameLen || len(a.Voice) > maxNameLen {
			return failure(fmt.Errorf("model and voice names are limited to %d bytes", maxNameLen)), nil, nil
		}
		if a.Speed != 0 && (a.Speed < minSpeed || a.Speed > maxSpeed) {
			return failure(fmt.Errorf("speed must be between %g and %g", minSpeed, maxSpeed)), nil, nil
		}
		wav, err := b.Speak(ctx, SpeechRequest{Text: a.Text, Model: a.Model, Voice: a.Voice, Speed: a.Speed})
		if err != nil {
			return failure(err), nil, nil
		}
		return &sdk.CallToolResult{Content: []sdk.Content{
			&sdk.AudioContent{Data: wav, MIMEType: "audio/wav"},
			&sdk.TextContent{Text: fmt.Sprintf("WAV audio, %d bytes.", len(wav))},
		}}, nil, nil
	})
	return s
}

func text(s string) *sdk.CallToolResult {
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: s}}}
}

func failure(err error) *sdk.CallToolResult {
	r := text(err.Error())
	r.IsError = true
	return r
}
