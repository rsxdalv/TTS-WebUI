package appconfig

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
)

// Operations on single settings. Python used to change a shared in-memory dict
// and save the whole document; each operation here changes one thing in
// config.json through Store.update, so concurrent changes do not lose each
// other and the bytes stay what json.dump(indent=2) writes.
//
// API (POST, JSON object body, same token as /api/config), under OpsPrefix:
//
//	extension-enable   {package_name}            -> {changed, disabled:false, info}
//	extension-disable  {package_name}            -> {changed, disabled:true,  info}
//	extension-toggle   {package_name}            -> {changed:true, disabled, info}
//	extension-autostart {package_name, enabled}  -> {changed, enabled}
//	set-value          {namespace, key, value}   -> {changed}
//	replace-section    {namespace, values}       -> {changed}
//
// info is the toast text the Python handler passes to gr.Info.

// OpsPrefix is the path prefix of the operations.
const OpsPrefix = Path + "/"

var packageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.\-]{0,199}$`)

func checkPackage(name string) error {
	if !packageName.MatchString(name) {
		return &invalidError{fmt.Sprintf("package_name %q is not a package name", name)}
	}
	return nil
}

// checkName validates a namespace or key: non-empty, short, printable.
func checkName(what, s string) error {
	if s == "" || len(s) > 200 {
		return &invalidError{what + " must be a non-empty string of at most 200 bytes"}
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return &invalidError{what + " must not contain control characters"}
		}
	}
	return nil
}

// section returns doc[name] as an object, creating it when absent. A value of
// another type is refused, never overwritten.
func section(doc *object, name string) (*object, error) {
	v, ok := doc.get(name)
	if !ok {
		o := &object{}
		doc.set(name, o)
		return o, nil
	}
	o, ok := v.(*object)
	if !ok {
		return nil, &invalidError{fmt.Sprintf("config[%q] is not an object", name)}
	}
	return o, nil
}

// stringList returns the array at section[key] (creating it) and its setter.
func stringList(sec *object, key string) ([]any, error) {
	v, ok := sec.get(key)
	if !ok {
		return []any{}, nil
	}
	l, ok := v.([]any)
	if !ok {
		return nil, &invalidError{fmt.Sprintf("config.extensions.%s is not a list", key)}
	}
	return l, nil
}

func contains(l []any, name string) bool {
	for _, e := range l {
		if s, ok := e.(string); ok && s == name {
			return true
		}
	}
	return false
}

// listAdd appends name unless present; listRemove drops every occurrence.
// Both report whether the list changed.
func listAdd(sec *object, key, name string) (bool, error) {
	l, err := stringList(sec, key)
	if err != nil {
		return false, err
	}
	_, existed := sec.get(key)
	if contains(l, name) {
		return !existed, nil
	}
	sec.set(key, append(l, name))
	return true, nil
}

func listRemove(sec *object, key, name string) (bool, error) {
	l, err := stringList(sec, key)
	if err != nil {
		return false, err
	}
	_, existed := sec.get(key)
	out := make([]any, 0, len(l))
	for _, e := range l {
		if s, ok := e.(string); ok && s == name {
			continue
		}
		out = append(out, e)
	}
	sec.set(key, out)
	return len(out) != len(l) || !existed, nil
}

// SetExtensionDisabled adds or removes name in extensions.disabled and reports
// whether the file changed.
func (s *Store) SetExtensionDisabled(name string, disabled bool) (bool, error) {
	if err := checkPackage(name); err != nil {
		return false, err
	}
	var changed bool
	err := s.update(func(doc *object) (bool, error) {
		sec, err := section(doc, "extensions")
		if err != nil {
			return false, err
		}
		if disabled {
			changed, err = listAdd(sec, "disabled", name)
		} else {
			changed, err = listRemove(sec, "disabled", name)
		}
		return changed, err
	})
	if err == nil && changed {
		s.opts.Logf("config: extension %s %s", name, map[bool]string{true: "disabled", false: "enabled"}[disabled])
	}
	return changed, err
}

// ToggleExtension flips name in extensions.disabled and returns the new state
// (true: now disabled). The decision is made under the lock, from the file.
func (s *Store) ToggleExtension(name string) (bool, error) {
	if err := checkPackage(name); err != nil {
		return false, err
	}
	var now bool
	err := s.update(func(doc *object) (bool, error) {
		sec, err := section(doc, "extensions")
		if err != nil {
			return false, err
		}
		l, err := stringList(sec, "disabled")
		if err != nil {
			return false, err
		}
		now = !contains(l, name)
		if now {
			_, err = listAdd(sec, "disabled", name)
		} else {
			_, err = listRemove(sec, "disabled", name)
		}
		return true, err
	})
	if err == nil {
		s.opts.Logf("config: extension %s %s", name, map[bool]string{true: "disabled", false: "enabled"}[now])
	}
	return now, err
}

// SetExtensionAutostart adds or removes name in extensions.autostart.
func (s *Store) SetExtensionAutostart(name string, enabled bool) (bool, error) {
	if err := checkPackage(name); err != nil {
		return false, err
	}
	var changed bool
	err := s.update(func(doc *object) (bool, error) {
		sec, err := section(doc, "extensions")
		if err != nil {
			return false, err
		}
		if enabled {
			changed, err = listAdd(sec, "autostart", name)
		} else {
			changed, err = listRemove(sec, "autostart", name)
		}
		return changed, err
	})
	return changed, err
}

// SetValue sets doc[namespace][key] = value (value is any JSON value), creating
// the namespace when absent. Values are never logged (they may be secrets).
func (s *Store) SetValue(namespace, key string, value any) error {
	if err := checkName("namespace", namespace); err != nil {
		return err
	}
	if err := checkName("key", key); err != nil {
		return err
	}
	return s.update(func(doc *object) (bool, error) {
		sec, err := section(doc, namespace)
		if err != nil {
			return false, err
		}
		sec.set(key, value)
		return true, nil
	})
}

// ReplaceSection sets doc[namespace] to the object values.
func (s *Store) ReplaceSection(namespace string, values *object) error {
	if err := checkName("namespace", namespace); err != nil {
		return err
	}
	for _, it := range values.Items {
		if err := checkName("key", it.Key); err != nil {
			return err
		}
	}
	return s.update(func(doc *object) (bool, error) {
		doc.set(namespace, values)
		return true, nil
	})
}

// OpsHandler serves the operations under OpsPrefix.
func (s *Store) OpsHandler() http.Handler { return http.HandlerFunc(s.serveOps) }

const (
	toastEnabled  = "Enabled extension. Please restart the application for changes to take effect."
	toastDisabled = "Disabled extension. Please restart the application for changes to take effect."
)

func toast(disabled bool) string {
	if disabled {
		return toastDisabled
	}
	return toastEnabled
}

func (s *Store) serveOps(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	op := strings.TrimPrefix(r.URL.Path, OpsPrefix)
	switch op {
	case "extension-enable", "extension-disable", "extension-toggle", "extension-autostart", "set-value", "replace-section":
	default:
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, err.Error())
		return
	}
	v, err := parse(raw)
	body, ok := v.(*object)
	if err != nil || !ok {
		writeErr(w, http.StatusBadRequest, "body must be a JSON object")
		return
	}
	res, err := s.runOp(op, body)
	if err != nil {
		code := http.StatusInternalServerError
		var inv *invalidError
		if errors.As(err, &inv) {
			code = http.StatusBadRequest
		} else {
			s.opts.Logf("config: %s: %v", op, err)
		}
		writeErr(w, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// fields checks body has exactly the keys in want.
func fields(body *object, want ...string) error {
	for _, it := range body.Items {
		known := false
		for _, k := range want {
			known = known || k == it.Key
		}
		if !known {
			return &invalidError{fmt.Sprintf("unknown field %q", it.Key)}
		}
	}
	for _, k := range want {
		if _, ok := body.get(k); !ok {
			return &invalidError{fmt.Sprintf("missing field %q", k)}
		}
	}
	return nil
}

func strField(body *object, k string) (string, error) {
	s, ok := mustGet(body, k).(string)
	if !ok {
		return "", &invalidError{fmt.Sprintf("%s must be a string", k)}
	}
	return s, nil
}

func mustGet(o *object, k string) any { v, _ := o.get(k); return v }

func (s *Store) runOp(op string, body *object) (map[string]any, error) {
	switch op {
	case "extension-enable", "extension-disable", "extension-toggle":
		if err := fields(body, "package_name"); err != nil {
			return nil, err
		}
		name, err := strField(body, "package_name")
		if err != nil {
			return nil, err
		}
		if op == "extension-toggle" {
			disabled, err := s.ToggleExtension(name)
			if err != nil {
				return nil, err
			}
			return map[string]any{"changed": true, "disabled": disabled, "info": toast(disabled)}, nil
		}
		disabled := op == "extension-disable"
		changed, err := s.SetExtensionDisabled(name, disabled)
		if err != nil {
			return nil, err
		}
		return map[string]any{"changed": changed, "disabled": disabled, "info": toast(disabled)}, nil
	case "extension-autostart":
		if err := fields(body, "package_name", "enabled"); err != nil {
			return nil, err
		}
		name, err := strField(body, "package_name")
		if err != nil {
			return nil, err
		}
		enabled, ok := mustGet(body, "enabled").(bool)
		if !ok {
			return nil, &invalidError{"enabled must be a boolean"}
		}
		changed, err := s.SetExtensionAutostart(name, enabled)
		if err != nil {
			return nil, err
		}
		return map[string]any{"changed": changed, "enabled": enabled}, nil
	case "set-value":
		if err := fields(body, "namespace", "key", "value"); err != nil {
			return nil, err
		}
		ns, err := strField(body, "namespace")
		if err != nil {
			return nil, err
		}
		key, err := strField(body, "key")
		if err != nil {
			return nil, err
		}
		if err := s.SetValue(ns, key, mustGet(body, "value")); err != nil {
			return nil, err
		}
		return map[string]any{"changed": true}, nil
	default: // replace-section
		if err := fields(body, "namespace", "values"); err != nil {
			return nil, err
		}
		ns, err := strField(body, "namespace")
		if err != nil {
			return nil, err
		}
		vals, ok := mustGet(body, "values").(*object)
		if !ok {
			return nil, &invalidError{"values must be an object"}
		}
		if err := s.ReplaceSection(ns, vals); err != nil {
			return nil, err
		}
		return map[string]any{"changed": true}, nil
	}
}
