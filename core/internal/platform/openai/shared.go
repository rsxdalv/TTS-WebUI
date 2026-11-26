package openai

// Helpers other remote-service packages (internal/cloud/...) share with the
// openai platform so the rules about keys are kept in one place: the same
// transport rule, the same scrubbing, the same file mode, the same control-API
// check and the same HTTP client that never follows a redirect.

import "net/http"

// Scrub removes the key from a message that is going to be shown or logged.
func Scrub(key, s string) string { return scrub(key, s) }

// Snippet is the start of a response body when it is text, else "binary data".
func Snippet(data []byte) string { return snippet(data) }

// ShortErr is a network error without the URL net/http puts in front of it.
func ShortErr(err error) string { return shortErr(err) }

// Unreachable words a failure before any answer from host.
func Unreachable(host string, err error) string {
	return unreachable(Endpoint{URL: "http://" + host}, err)
}

// CheckTransport refuses a key over plain http to a public host.
func CheckTransport(rawURL string, hasKey bool) error { return checkTransport(rawURL, hasKey) }

// WriteFileAtomic replaces path with v as JSON: mode 0600, in a 0700 folder, atomically.
func WriteFileAtomic(path string, v any) error { return writeFile(path, v) }

// ReadJSON reads path into v; a missing file leaves v alone.
func ReadJSON(path string, v any) error { return readJSON(path, v) }

// Invalidf is an error the caller's input caused (wraps ErrInvalid).
func Invalidf(format string, a ...any) error { return invalidf(format, a...) }

// Truncate cuts a remote message to 300 characters.
func Truncate(s string) string { return truncate(s) }

// Authorized reports whether r is a control API request: a loopback peer
// carrying the shared token.
func Authorized(r *http.Request, token string) bool { return (&Handler{token: token}).authorized(r) }

// WriteJSON answers with v (no-store).
func WriteJSON(w http.ResponseWriter, code int, v any) { writeJSON(w, code, v) }

// WriteErr answers {"error": msg}.
func WriteErr(w http.ResponseWriter, code int, msg string) { writeErr(w, code, msg) }

// DecodeJSON reads a strict (no unknown fields, at most 1 MiB) JSON request body.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error { return decode(w, r, v) }

// NoRedirectClient returns a copy of hc (or the default client when nil) that
// never follows a redirect: a key would travel to a host the user did not enter.
func NoRedirectClient(hc *http.Client) *http.Client { return noRedirect(hc) }
