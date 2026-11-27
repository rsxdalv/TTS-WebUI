package launcher

import (
	"fmt"
	"html"
	"net/http"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
)

// noHostPage answers GET / of the public port while there is no Gradio host: it
// says why and links to the Vite UI, which runs without it.
func noHostPage(detail string) http.Handler {
	reactPort := supervise.DefaultReactPort
	if n, err := reactPortFromEnv(); err == nil && n != 0 {
		reactPort = n
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>TTS WebUI</title>
<style>body{font:16px system-ui,sans-serif;max-width:40rem;margin:3rem auto;padding:0 1rem;line-height:1.5}</style>
<h1>The Gradio pages are not installed</h1>
<p>%s</p>
<p><a id="ui" href="#">Open the TTS WebUI</a> (it works without them). The Gradio host is installed from the launcher's Extensions page.</p>
<script>document.getElementById("ui").href = location.protocol + "//" + location.hostname + ":%d/"</script>`,
			html.EscapeString(detail), reactPort)
	})
}
