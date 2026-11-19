package appenv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// splitEnv splits "NAME=value", allowing the Windows "=C:=C:\x" form.
func splitEnv(e string) (string, string, bool) {
	if e == "" {
		return "", "", false
	}
	i := strings.IndexByte(e[1:], '=')
	if i < 0 {
		return e, "", false
	}
	i++
	return e[:i], e[i+1:], true
}

func canon(k string, fold bool) string {
	if fold {
		return strings.ToUpper(k)
	}
	return k
}

// lookupEnv finds name in env; the last occurrence wins.
func lookupEnv(env []string, name string, fold bool) (string, bool) {
	cn := canon(name, fold)
	val, found := "", false
	for _, e := range env {
		if k, v, ok := splitEnv(e); ok && canon(k, fold) == cn {
			val, found = v, true
		}
	}
	return val, found
}

// FindDotenv mirrors python-dotenv find_dotenv() as called by
// dotenv_manager/init.py: the search starts at the directory of init.py
// (root/tts_webui/dotenv_manager) and walks up to the filesystem root. In
// practice that is root/.env, unless a .env exists in tts_webui/dotenv_manager
// or tts_webui, which then shadows it. Returns "" if none is found.
func FindDotenv(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	dir := filepath.Join(abs, "tts_webui", "dotenv_manager")
	for {
		p := filepath.Join(dir, ".env")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// EnvReport is the result of loading the dotenv files.
type EnvReport struct {
	Env []string
	// Warnings lists statements python-dotenv would log as unparsable.
	Warnings []string
	// WarnLines is the start line of each such statement, .env first.
	WarnLines []int
}

// LoadEnv gives the environment server.py has after init():
// load_dotenv() (the .env FindDotenv resolves, never overriding base) then
// load_dotenv(".env.user", override=True). base is not modified.
func LoadEnv(root string, base []string) ([]string, error) {
	r, err := LoadEnvReport(root, base)
	return r.Env, err
}

// LoadEnvReport is LoadEnv plus parse warnings.
func LoadEnvReport(root string, base []string) (EnvReport, error) {
	return loadEnv(root, base, runtime.GOOS == "windows")
}

type envPair struct {
	name string
	val  string
}

// loadEnv: fold=true reproduces Windows, where os.environ upper-cases names
// (so `k in os.environ` is case-insensitive and ${var} lookups of inherited
// variables only match upper-case names).
func loadEnv(root string, base []string, fold bool) (EnvReport, error) {
	var rep EnvReport
	var entries []envPair
	index := map[string]int{} // canonical name -> entries index
	for _, e := range base {
		k, v, ok := splitEnv(e)
		if !ok {
			continue
		}
		c := canon(k, fold)
		if i, dup := index[c]; dup {
			entries[i].val = v
			continue
		}
		index[c] = len(entries)
		entries = append(entries, envPair{k, v})
	}
	apply := func(path string, override bool) error {
		if path == "" {
			return nil
		}
		st, err := os.Stat(path)
		if err != nil || !st.Mode().IsRegular() {
			return nil // python-dotenv: not a file -> empty stream
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(raw) {
			return fmt.Errorf("%s: not valid UTF-8", path)
		}
		// Python opens the file in text mode: universal newlines.
		text := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(string(raw))
		binds, errLines := parseDotenv(text)
		for _, l := range errLines {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: could not parse statement starting at line %d", filepath.Base(path), l))
			rep.WarnLines = append(rep.WarnLines, l)
		}
		// resolve_variables
		var order []string
		newVals := map[string]*string{}
		for _, b := range binds {
			var res *string
			if b.hasValue {
				osenv := map[string]*string{}
				for _, e := range entries {
					v := e.val
					osenv[canon(e.name, fold)] = &v
				}
				env := map[string]*string{}
				first, second := osenv, newVals
				if !override {
					first, second = newVals, osenv
				}
				for k, v := range first {
					env[k] = v
				}
				for k, v := range second {
					env[k] = v
				}
				s := resolveVars(b.value, env)
				res = &s
			}
			if _, seen := newVals[b.key]; !seen {
				order = append(order, b.key)
			}
			newVals[b.key] = res
		}
		// set_as_environment_variables
		for _, k := range order {
			v := newVals[k]
			c := canon(k, fold)
			i, exists := index[c]
			if exists && !override {
				continue
			}
			if v == nil {
				continue
			}
			if strings.ContainsAny(k, "=\x00") || strings.Contains(*v, "\x00") {
				return fmt.Errorf("%s: invalid environment variable %q", path, k)
			}
			if exists {
				entries[i].val = *v
			} else {
				index[c] = len(entries)
				entries = append(entries, envPair{k, *v})
			}
		}
		return nil
	}
	if err := apply(FindDotenv(root), false); err != nil {
		return rep, err
	}
	if err := apply(filepath.Join(root, ".env.user"), true); err != nil {
		return rep, err
	}
	for _, e := range entries {
		rep.Env = append(rep.Env, e.name+"="+e.val)
	}
	return rep, nil
}
