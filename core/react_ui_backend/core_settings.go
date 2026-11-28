package react_ui_backend

// Server settings page (the Go-core part of the gradio_settings tab): the
// options of config.json["gradio_interface_options"] that the Go core reads
// (server_name, server_port, auth, ssl_*). The Gradio-only options of the old
// tab (inline, share, show_api, ...) are not offered; they stay in the file
// untouched, because a save merges into the section and replaces it whole.
//
// Secrets never leave Go: GET says whether a password is set and returns the
// user names only; PUT keeps what it is not given.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "core-settings",
		Patterns: []string{"GET /api/go-data/server-settings", "PUT /api/go-data/server-settings"},
		Handler:  func(d Deps) http.Handler { return newServerSettings(d.Core, gradioPrefix(d.GradioBackend)) },
	})
}

const settingsSection = "gradio_interface_options"

type settingsView struct {
	ServerName        string   `json:"server_name"`
	ServerPort        *int     `json:"server_port"`
	SSLKeyfile        string   `json:"ssl_keyfile"`
	SSLCertfile       string   `json:"ssl_certfile"`
	SSLVerify         *bool    `json:"ssl_verify"`
	AuthUsers         []string `json:"auth_users"`
	SSLKeyfilePassSet bool     `json:"ssl_keyfile_password_set"`
	// GradioPrefix is where Gradio lives on the public port ("" is "/"). Read only.
	GradioPrefix string `json:"gradio_prefix"`
}

// gradioPrefix is the path of the Gradio backend URL without its slashes:
// "http://127.0.0.1:7770/gradio-legacy/" gives "/gradio-legacy", the root gives "".
func gradioPrefix(backend string) string {
	u, err := url.Parse(strings.TrimSpace(backend))
	if err != nil {
		return ""
	}
	p := strings.TrimRight(u.Path, "/")
	if p == "" || !strings.HasPrefix(p, "/") {
		return ""
	}
	return p
}

func strOf(m map[string]json.RawMessage, k string) string {
	var s string
	_ = json.Unmarshal(m[k], &s)
	return s
}

// authUsers lists the user names of the three forms the config accepts:
// "user:password", ["user","password"] and [["user","password"], ...].
func authUsers(raw json.RawMessage) []string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s == "" {
			return nil
		}
		return []string{strings.SplitN(s, ":", 2)[0]}
	}
	var flat []string
	if json.Unmarshal(raw, &flat) == nil {
		if len(flat) == 0 {
			return nil
		}
		return []string{flat[0]}
	}
	var nested [][]string
	if json.Unmarshal(raw, &nested) == nil {
		var out []string
		for _, p := range nested {
			if len(p) > 0 {
				out = append(out, p[0])
			}
		}
		return out
	}
	return nil
}

func viewOf(section map[string]json.RawMessage) settingsView {
	v := settingsView{
		ServerName:  strOf(section, "server_name"),
		SSLKeyfile:  strOf(section, "ssl_keyfile"),
		SSLCertfile: strOf(section, "ssl_certfile"),
		AuthUsers:   authUsers(section["auth"]),
	}
	var p int
	if json.Unmarshal(section["server_port"], &p) == nil {
		v.ServerPort = &p
	}
	var b bool
	if json.Unmarshal(section["ssl_verify"], &b) == nil {
		v.SSLVerify = &b
	}
	v.SSLKeyfilePassSet = strOf(section, "ssl_keyfile_password") != ""
	if v.AuthUsers == nil {
		v.AuthUsers = []string{}
	}
	return v
}

// loadSection reads the section from the config document.
func loadSection(core *Core, r *http.Request) (map[string]json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := core.getJSON(r.Context(), "/api/config", &doc); err != nil {
		return nil, err
	}
	section := map[string]json.RawMessage{}
	if raw, ok := doc[settingsSection]; ok {
		_ = json.Unmarshal(raw, &section)
		if section == nil {
			section = map[string]json.RawMessage{}
		}
	}
	return section, nil
}

// settingsPut is the body of PUT; a field that is absent stays as it is.
// auth and ssl_keyfile_password: null removes, a value sets.
type settingsPut struct {
	ServerName     *string         `json:"server_name"`
	ServerPort     *int            `json:"server_port"`
	SSLKeyfile     *string         `json:"ssl_keyfile"`
	SSLCertfile    *string         `json:"ssl_certfile"`
	SSLVerify      *bool           `json:"ssl_verify"`
	Auth           json.RawMessage `json:"auth"`
	SSLKeyfilePass json.RawMessage `json:"ssl_keyfile_password"`
}

func badText(s string) bool {
	return len(s) > 1024 || strings.ContainsAny(s, "\x00\r\n")
}

func newServerSettings(core *Core, prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			section, err := loadSection(core, r)
			if err != nil {
				coreError(w, err)
				return
			}
			view := viewOf(section)
			view.GradioPrefix = prefix
			writeJSONStatus(w, http.StatusOK, view)
			return
		}
		if !requireJSON(w, r) {
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		var in settingsPut
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil || dec.More() {
			writeAPIError(w, http.StatusBadRequest, "body must be one JSON object with known fields")
			return
		}
		for _, s := range []*string{in.ServerName, in.SSLKeyfile, in.SSLCertfile} {
			if s != nil && badText(*s) {
				writeAPIError(w, http.StatusBadRequest, "text values must be short single lines")
				return
			}
		}
		if in.ServerPort != nil && (*in.ServerPort < 1 || *in.ServerPort > 65535) {
			writeAPIError(w, http.StatusBadRequest, "server_port must be between 1 and 65535")
			return
		}
		section, err := loadSection(core, r)
		if err != nil {
			coreError(w, err)
			return
		}
		set := func(k string, v any) {
			b, _ := json.Marshal(v)
			section[k] = b
		}
		if in.ServerName != nil {
			set("server_name", *in.ServerName)
		}
		if in.ServerPort != nil {
			set("server_port", *in.ServerPort)
		}
		if in.SSLKeyfile != nil {
			set("ssl_keyfile", *in.SSLKeyfile)
		}
		if in.SSLCertfile != nil {
			set("ssl_certfile", *in.SSLCertfile)
		}
		if in.SSLVerify != nil {
			set("ssl_verify", *in.SSLVerify)
		}
		if in.SSLKeyfilePass != nil {
			if string(in.SSLKeyfilePass) == "null" {
				delete(section, "ssl_keyfile_password")
			} else {
				var p string
				if json.Unmarshal(in.SSLKeyfilePass, &p) != nil || badText(p) {
					writeAPIError(w, http.StatusBadRequest, "ssl_keyfile_password must be a short string or null")
					return
				}
				set("ssl_keyfile_password", p)
			}
		}
		if in.Auth != nil {
			if string(in.Auth) == "null" {
				delete(section, "auth")
			} else {
				var a struct {
					User     string `json:"user"`
					Password string `json:"password"`
				}
				d := json.NewDecoder(bytes.NewReader(in.Auth))
				d.DisallowUnknownFields()
				if d.Decode(&a) != nil || a.User == "" || a.Password == "" || badText(a.User) || badText(a.Password) {
					writeAPIError(w, http.StatusBadRequest, "auth must be null or {user, password}, both not empty")
					return
				}
				set("auth", []string{a.User, a.Password})
			}
		}
		payload, _ := json.Marshal(map[string]any{"namespace": settingsSection, "values": section})
		resp, err := core.do(r.Context(), http.MethodPost, "/api/config/replace-section", bytes.NewReader(payload), "application/json")
		if err != nil {
			coreError(w, err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			writeAPIError(w, http.StatusBadGateway, "the core refused the settings")
			return
		}
		writeJSONStatus(w, http.StatusOK, viewOf(section))
	})
}
