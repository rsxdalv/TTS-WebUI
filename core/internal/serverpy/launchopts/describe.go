package launchopts

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// The grouped, compact listing print_gradio_options.py printed, with the
// secrets left out.

var groups = []struct {
	name string
	keys []string
}{
	{"Server", []string{"server_name", "server_port", "max_threads", "prevent_thread_lock", "quiet", "root_path", "share_server_address", "share_server_protocol", "strict_cors", "state_session_capacity", "enable_monitoring", "ssr_mode"}},
	{"Display", []string{"inline", "inbrowser", "width", "height", "favicon_path", "show_api"}},
	{"Access", []string{"share", "auth_message", "auth_dependency", "max_file_size"}},
	{"Security", []string{"ssl_keyfile", "ssl_certfile", "ssl_keyfile_password", "ssl_verify", "allowed_paths", "blocked_paths"}},
	{"Debug", []string{"debug", "show_error", "app_kwargs"}},
}

// secretOptions are shown as "******" when they have a value. The listing has
// never printed auth (it is in no group); it stays out of the lines on purpose.
var secretOptions = map[string]bool{"ssl_keyfile_password": true}

const masked = "******"

// describe returns the lines Python prints before it launches.
func describe(opts map[string]any) []string {
	lines := []string{fmt.Sprintf("Starting Gradio server: http://%s:%s", orDefault(opts["server_name"], "127.0.0.1"), orDefault(opts["server_port"], "7770"))}
	for _, g := range groups {
		var parts []string
		for _, k := range g.keys {
			v, ok := opts[k]
			if !ok {
				continue
			}
			if s, isStr := v.(string); isStr && s != "" && secretOptions[k] {
				v = masked
			}
			parts = append(parts, k+": "+formatValue(v))
		}
		if len(parts) > 0 {
			lines = append(lines, fmt.Sprintf("  • %-8s > %s", g.name, strings.Join(parts, ", ")))
		}
	}
	if opts["server_name"] == "0.0.0.0" {
		lines = append(lines,
			"Notice: Server is open to the internet",
			fmt.Sprintf("Gradio server will be available on http://localhost:%s", pyStr(opts["server_port"])))
	}
	return lines
}

func orDefault(v any, def string) string {
	if v == nil {
		return def
	}
	return pyStr(v)
}

// formatValue is the listing's formatting: strings in double quotes, cut to 17
// characters and "..." when longer than 20, everything else as Python's str().
func formatValue(v any) string {
	if s, ok := v.(string); ok {
		if r := []rune(s); len(r) > 20 {
			return `"` + string(r[:17]) + `..."`
		}
		return `"` + s + `"`
	}
	return pyStr(v)
}

// pyStr is Python's str() of a decoded JSON value.
func pyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case json.Number:
		return x.String()
	case int:
		return fmt.Sprint(x)
	}
	return pyRepr(v)
}

// pyRepr is Python's repr() of a decoded JSON value (strings in single quotes).
func pyRepr(v any) string {
	switch x := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(x, "'", `\'`) + "'"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = pyRepr(k) + ": " + pyRepr(x[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return pyStr(v)
}
