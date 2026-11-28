package launcher

import (
	"net/http"
)

// frontStoppedPage answers "/" of the public port while the React UI is not
// running (the router serves the React UI there when it is).
func frontStoppedPage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>TTS WebUI</title>
<style>body{font:16px system-ui,sans-serif;max-width:40rem;margin:3rem auto;padding:0 1rem;line-height:1.5}</style>
<h1>The TTS WebUI frontend is not running</h1>
<p>Start it from the launcher page (Launch frontend). The Gradio pages, when installed, are under <a href="` + gradioLegacyPrefix + `/">` + gradioLegacyPrefix + `/</a>.</p>`))
	})
}
