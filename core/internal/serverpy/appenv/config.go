package appenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigError means server.py would crash on this config.json (KeyError,
// ValueError, ...) or the file is unusable.
type ConfigError struct{ Msg string }

func (e *ConfigError) Error() string { return "config.json: " + e.Msg }

func cfgErr(f string, a ...any) error { return &ConfigError{fmt.Sprintf(f, a...)} }

// Credential is one gradio login. Formatting it never prints the password.
type Credential struct {
	User     string
	Password string
}

func (c Credential) String() string   { return c.User + ":******" }
func (c Credential) GoString() string { return "appenv.Credential{" + c.String() + "}" }
func (c Credential) Format(f fmt.State, _ rune) {
	fmt.Fprint(f, c.String())
}

// Auth is the parsed gradio `auth` option; empty means no authentication.
type Auth struct {
	Credentials []Credential
}

func (a Auth) Enabled() bool { return len(a.Credentials) > 0 }

// String is safe to log.
func (a Auth) String() string {
	if !a.Enabled() {
		return "auth: none"
	}
	parts := make([]string, len(a.Credentials))
	for i, c := range a.Credentials {
		parts[i] = c.String()
	}
	return "auth: " + strings.Join(parts, ",")
}

// Config is the part of config.json the launcher needs.
type Config struct {
	Port        int
	Host        string
	InBrowser   bool
	Share       bool
	Auth        Auth
	FromDefault bool // no config.json: load_config.py defaults
}

// default_config in load_config.py, gradio_interface_options part.
func defaultConfig() Config {
	return Config{Port: 7770, Host: "127.0.0.1", InBrowser: true, FromDefault: true}
}

// LoadConfig reads root/config.json like load_config.py: a present file is
// used as is (there is no merge with the defaults), a missing one means the
// defaults. Python also writes the default file then; this does not, Python
// will when it starts. Keys server.py reads with [] (auth, server_name,
// server_port) must exist, as in Python.
func LoadConfig(root string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(root, "config.json"))
	if os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	if err != nil {
		return Config{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return Config{}, cfgErr("%v", err)
	}
	raw, ok := doc["gradio_interface_options"]
	if !ok {
		return Config{}, cfgErr("missing gradio_interface_options")
	}
	o, ok := raw.(map[string]any)
	if !ok {
		return Config{}, cfgErr("gradio_interface_options is not an object")
	}
	var c Config
	for _, k := range []string{"auth", "server_name", "server_port"} {
		if _, ok := o[k]; !ok {
			return Config{}, cfgErr("gradio_interface_options.%s missing (server.py would raise KeyError)", k)
		}
	}
	if c.Port, err = pyInt(o["server_port"]); err != nil {
		return Config{}, cfgErr("server_port: %v", err)
	}
	if c.Port < 1 || c.Port > 65535 {
		return Config{}, cfgErr("server_port %d out of range", c.Port)
	}
	switch h := o["server_name"].(type) {
	case nil:
		c.Host = "127.0.0.1"
	case string:
		c.Host = h
		if h == "" {
			c.Host = "127.0.0.1"
		}
	default:
		return Config{}, cfgErr("server_name must be a string")
	}
	c.InBrowser = truthy(o["inbrowser"])
	c.Share = truthy(o["share"])
	if c.Auth, err = parseAuth(o["auth"]); err != nil {
		return Config{}, err
	}
	return c, nil
}

// pyInt is Python int(x) for JSON values.
func pyInt(v any) (int, error) {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return int(i), nil
		}
		f, err := x.Float64()
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("not a number: %v", x)
		}
		return int(f), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, fmt.Errorf("not an integer: %q", x)
		}
		return i, nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("not a number: %v", v)
}

// truthy is Python truthiness for a decoded JSON value.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, _ := x.Float64()
		return f != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

// parseAuth follows upgrade_gradio_options: a string is split on ":" (gradio
// then uses the first two parts, so a ":" in the password truncates it); a
// list is used as is; a list whose first element is itself a list is several
// logins.
func parseAuth(v any) (Auth, error) {
	switch x := v.(type) {
	case nil:
		return Auth{}, nil
	case string:
		p := strings.Split(x, ":")
		if len(p) < 2 {
			return Auth{}, cfgErr("auth string must be user:password")
		}
		return Auth{[]Credential{{p[0], p[1]}}}, nil
	case []any:
		if len(x) == 0 {
			return Auth{}, nil
		}
		if _, nested := x[0].([]any); nested {
			var a Auth
			for _, e := range x {
				c, err := pair(e)
				if err != nil {
					return Auth{}, err
				}
				a.Credentials = append(a.Credentials, c)
			}
			return a, nil
		}
		c, err := pair(x)
		if err != nil {
			return Auth{}, err
		}
		return Auth{[]Credential{c}}, nil
	}
	return Auth{}, cfgErr("auth must be a string, a list or null")
}

func pair(v any) (Credential, error) {
	l, ok := v.([]any)
	if !ok || len(l) < 2 {
		return Credential{}, cfgErr("auth entries must be [user, password]")
	}
	u, ok1 := l[0].(string)
	p, ok2 := l[1].(string)
	if !ok1 || !ok2 {
		return Credential{}, cfgErr("auth user and password must be strings")
	}
	return Credential{u, p}, nil
}
