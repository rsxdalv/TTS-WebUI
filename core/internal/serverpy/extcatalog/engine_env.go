package extcatalog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// reservedEnv are names an entry may not set: the control API's address and
// token never reach an engine, and the search paths decide which code runs.
func reservedEnv(k string) bool {
	switch k {
	case "PATH", "PYTHONPATH", "PYTHONHOME", "PYTHONSTARTUP", "TTS_WEBUI_EXTENSION_PACKAGE", "GRADIO_SERVER_PORT", "GRADIO_ROOT_PATH":
		return true
	}
	return strings.HasPrefix(k, "TTS_CORE_")
}

// EngineEnv returns the variables an own-venv entry asks for in its "env"
// object ("env": {"NAME": "value"}), as KEY=VALUE sorted by name. An engine
// needs one when its code only works with a setting the old shared env had
// (Vall-E-X loads a checkpoint that torch 2.6+ refuses without
// TORCH_FORCE_NO_WEIGHTS_ONLY_LOAD). A bad name or a reserved one is an error,
// so the engine does not start with a half-applied environment.
func (s *Store) EngineEnv(pkg string) ([]string, error) {
	e, err := s.find(pkg)
	if err != nil {
		return nil, nil // not a catalog entry: nothing to add
	}
	return entryEnv(pkg, e)
}

func entryEnv(pkg string, e *Obj) ([]string, error) {
	raw, ok := e.Get("env")
	if !ok {
		return nil, nil
	}
	m, ok := raw.(*Obj)
	if !ok {
		return nil, fmt.Errorf("%s: \"env\" must be an object of strings", pkg)
	}
	var out []string
	for _, k := range m.Keys {
		v, isStr := m.Vals[k].(string)
		if !isStr || !envName.MatchString(k) || reservedEnv(k) || strings.ContainsRune(v, 0) {
			return nil, fmt.Errorf("%s: env %q is not allowed", pkg, k)
		}
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out, nil
}
