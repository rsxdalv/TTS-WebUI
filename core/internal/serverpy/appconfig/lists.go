package appconfig

import "fmt"

// ExtensionLists are the three lists of config.json's "extensions" section that
// decide which extensions load.
type ExtensionLists struct {
	Disabled           []string // interface extensions that show a "[Disabled]" tab
	Autostart          []string // native extensions started with the WebUI
	DisabledDecorators []string // decorator packages and decorator function names
}

// ExtensionLists reads the lists from the file. A missing section or key is an
// empty list; a value of another type is refused, like the operations in
// ops.go do, and entries that are not strings can never match a package name
// and are dropped.
func (s *Store) ExtensionLists() (ExtensionLists, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadLocked()
	if err != nil {
		return ExtensionLists{}, err
	}
	v, err := parse(data)
	if err != nil {
		return ExtensionLists{}, fmt.Errorf("config.json is not valid JSON: %w", err)
	}
	doc, ok := v.(*object)
	if !ok {
		return ExtensionLists{}, &invalidError{"config.json is not a JSON object"}
	}
	sec, err := section(doc, "extensions")
	if err != nil {
		return ExtensionLists{}, err
	}
	var out ExtensionLists
	for _, f := range []struct {
		key string
		dst *[]string
	}{
		{"disabled", &out.Disabled},
		{"autostart", &out.Autostart},
		{"disabled_decorators", &out.DisabledDecorators},
	} {
		l, err := stringList(sec, f.key)
		if err != nil {
			return ExtensionLists{}, err
		}
		for _, e := range l {
			if name, ok := e.(string); ok {
				*f.dst = append(*f.dst, name)
			}
		}
	}
	return out, nil
}
