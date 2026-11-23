package appconfig

import (
	"os"
	"reflect"
	"testing"
)

func TestExtensionListsReadWhatTheOperationsWrite(t *testing.T) {
	s, _ := newStore(t)
	got, err := s.ExtensionLists() // creates the default file, whose lists are empty
	if err != nil || len(got.Disabled)+len(got.Autostart)+len(got.DisabledDecorators) != 0 {
		t.Fatalf("default: %+v %v", got, err)
	}
	s.SetExtensionDisabled("a", true)
	s.SetExtensionDisabled("b", true)
	s.SetExtensionAutostart("c", true)
	if got, _ = s.ExtensionLists(); !reflect.DeepEqual(got, ExtensionLists{Disabled: []string{"a", "b"}, Autostart: []string{"c"}}) {
		t.Fatalf("%+v", got)
	}
}

func TestExtensionListsOfAHandEditedFile(t *testing.T) {
	s, _ := newStore(t)
	write := func(doc string) {
		t.Helper()
		if err := os.WriteFile(s.path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// No section at all, and entries that are not strings (they match no package).
	write(`{"gradio_interface_options":{}}`)
	if got, err := s.ExtensionLists(); err != nil || got.Disabled != nil {
		t.Fatalf("no section: %+v %v", got, err)
	}
	write(`{"extensions":{"disabled":["x",1,null,"y"],"disabled_decorators":["decorator_save_ogg"]}}`)
	got, err := s.ExtensionLists()
	if err != nil || !reflect.DeepEqual(got, ExtensionLists{Disabled: []string{"x", "y"}, DisabledDecorators: []string{"decorator_save_ogg"}}) {
		t.Fatalf("%+v %v", got, err)
	}
	// A wrong type is refused, not read as "nothing disabled".
	for _, bad := range []string{`{"extensions":[]}`, `{"extensions":{"disabled":"a"}}`, `{"extensions":{"autostart":{}}}`, `[1]`, `{`} {
		write(bad)
		if _, err := s.ExtensionLists(); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	// Reading never changes the file.
	write(`{"extensions":{"disabled":["x"]}}`)
	s.ExtensionLists()
	if b, _ := os.ReadFile(s.path); string(b) != `{"extensions":{"disabled":["x"]}}` {
		t.Errorf("file rewritten: %s", b)
	}
}
