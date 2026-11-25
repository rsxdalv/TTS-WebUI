package react_ui_backend

// The onnx platform's routes (P8): the engines it can run and the generate call.
// They forward to the control API (platform/onnx), which runs the program and
// saves the clip in outputs/. Cancelling the request (the browser's Cancel, or a
// closed tab) cancels the generation: the program is killed and nothing is saved.

import "net/http"

func init() {
	RegisterClaim(Claim{
		Switch: "core-onnx",
		Patterns: []string{
			"GET /api/go-data/onnx/engines",
			"POST /api/go-data/onnx/speech",
		},
		Handler: func(d Deps) http.Handler { return newONNX(d.Core) },
	})
}

func newONNX(core *Core) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, contentType := "/api/onnx/engines", ""
		var body []byte
		if r.Method == http.MethodPost {
			if !requireJSON(w, r) {
				return
			}
			b, ok := readBody(w, r)
			if !ok {
				return
			}
			path, contentType, body = "/api/onnx/speech", "application/json", b
		}
		resp, err := core.do(r.Context(), r.Method, path, bytesReader(body), contentType)
		if err != nil {
			coreError(w, err)
			return
		}
		relay(w, resp)
	})
}
