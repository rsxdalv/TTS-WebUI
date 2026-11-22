package extcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// requiredKeys are the string fields every extension entry needs
// (REQUIRED_KEYS in the Custom Extensions Installer).
var requiredKeys = []string{
	"package_name", "name", "requirements", "description", "extension_type",
	"extension_class", "author", "extension_author", "license", "website",
	"extension_website", "extension_platform_version",
}

// validateEntry is _validate_entry: "" when fine, else the reason.
func validateEntry(e *Obj) string {
	var missing, bad []string
	for _, k := range requiredKeys {
		v, ok := e.Get(k)
		if !ok {
			missing = append(missing, k)
		} else if _, isStr := v.(string); !isStr {
			bad = append(bad, k)
		}
	}
	if len(missing) > 0 {
		return "Missing keys: " + strings.Join(missing, ", ")
	}
	if len(bad) > 0 {
		return "Wrong types for: " + strings.Join(bad, ", ")
	}
	return ""
}

// ParseResult is what the installer shows for pasted JSON.
type ParseResult struct {
	Entries []any          `json:"entries"`
	Preview map[string]any `json:"preview"` // Markdown marker
	Info    string         `json:"info"`
}

// ParseInput is _parse_json_input plus _render_preview: one object or an array
// of objects, validated entry by entry. Entries keep their key order.
//
// The JSON is read with the CPython-compatible scanner (pyjson.go), so the
// error the user sees is the one json.loads would give.
func ParseInput(text string) ParseResult {
	res := ParseResult{Entries: []any{}}
	text = strings.TrimSpace(text)
	if text == "" {
		res.Info = "No JSON provided"
		res.Preview = mk("Markdown", "<i>No valid entries to preview.</i>")
		return res
	}
	v, perr := pyParse(text)
	if perr != nil {
		res.Info = fmt.Sprintf("JSON parse error: %v", perr)
		res.Preview = mk("Markdown", "<i>No valid entries to preview.</i>")
		return res
	}
	var objs []*Obj
	switch x := v.(type) {
	case *Obj:
		objs = []*Obj{x}
	case []any:
		for _, e := range x {
			if o, ok := e.(*Obj); ok {
				objs = append(objs, o)
			}
		}
	default:
		res.Info = "Expected a JSON object or an array of objects"
		res.Preview = mk("Markdown", "<i>No valid entries to preview.</i>")
		return res
	}
	var problems []string
	var valid []*Obj
	for i, e := range objs {
		if msg := validateEntry(e); msg == "" {
			valid = append(valid, e)
			res.Entries = append(res.Entries, e)
		} else {
			problems = append(problems, fmt.Sprintf("Entry %d: %s", i, msg))
		}
	}
	res.Info = fmt.Sprintf("Parsed entries: %d", len(valid))
	if len(problems) > 0 {
		res.Info += " | Issues: " + strings.Join(problems, " ; ")
	}
	res.Preview = mk("Markdown", preview(valid))
	return res
}

func preview(entries []*Obj) string {
	if len(entries) == 0 {
		return "<i>No valid entries to preview.</i>"
	}
	var b strings.Builder
	b.WriteString("| Name | Package | Class | Type |\n|---|---|---|---|\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", e.Str("name"), e.Str("package_name"), e.Str("extension_class"), e.Str("extension_type"))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// mk builds a Gradio object marker (tts_webui/core_bridge/gradio_resolve.py).
func mk(kind string, value string) map[string]any {
	return map[string]any{"__gradio__": kind, "props": map[string]any{"value": value}}
}

// AddResult is what the installer shows after "Add to external list".
type AddResult struct {
	Status string `json:"status"`
	JSON   string `json:"json"`
}

// Add is _add_to_external: append entries whose package_name is not yet in
// the external file's "tabs", and write the file (indent 4, UTF-8). Go is the
// only writer. A file that exists but is not a JSON object is left alone.
func (s *Store) Add(raw []any) (AddResult, error) {
	var entries []*Obj
	for i, v := range raw {
		o, ok := v.(*Obj)
		if !ok {
			return AddResult{}, invalidf("entry %d is not an object", i)
		}
		if msg := validateEntry(o); msg != "" {
			return AddResult{}, invalidf("entry %d: %s", i, msg)
		}
		entries = append(entries, o)
	}
	if len(entries) == 0 {
		return AddResult{Status: "Nothing to add."}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.opts.Root, externalFile)
	doc := newObj()
	doc.Set("tabs", []any{})
	doc.Set("decorators", []any{})
	if b, err := os.ReadFile(path); err == nil {
		v, err := Parse(b)
		o, ok := v.(*Obj)
		if err != nil || !ok {
			return AddResult{}, invalidf("%s is not a JSON object; fix or remove it first", externalFile)
		}
		doc = o
	} else if !os.IsNotExist(err) {
		return AddResult{}, err
	}
	tabsV, _ := doc.Get("tabs")
	tabs, ok := tabsV.([]any)
	if tabsV != nil && !ok {
		return AddResult{}, invalidf(`%s: "tabs" is not a list`, externalFile)
	}
	existing := map[string]bool{}
	for _, t := range tabs {
		if o, ok := t.(*Obj); ok {
			existing[o.Str("package_name")] = true
		}
	}
	var added, skipped []string
	for _, e := range entries {
		pkg := e.Str("package_name")
		if existing[pkg] {
			skipped = append(skipped, pkg)
			continue
		}
		existing[pkg] = true
		tabs = append(tabs, e)
		added = append(added, pkg)
	}
	doc.Set("tabs", tabs)
	if err := writeAtomic(path, Marshal(doc, "    ")); err != nil {
		return AddResult{}, err
	}
	s.opts.Logf("extcatalog: %s: added %v, skipped %v", externalFile, added, skipped)
	return AddResult{
		Status: "Saved | Added: " + orNone(added) + " | Skipped (exists): " + orNone(skipped),
		JSON:   string(Marshal(doc, "  ")),
	}, nil
}

func orNone(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return strings.Join(l, ", ")
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".extensions.external.*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
