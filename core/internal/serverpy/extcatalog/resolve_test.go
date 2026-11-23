package extcatalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
)

type fakeConfig struct {
	lists appconfig.ExtensionLists
	err   error
}

func (f fakeConfig) ExtensionLists() (appconfig.ExtensionLists, error) { return f.lists, f.err }

func resolveStore(t *testing.T, cfg ConfigSource) *Store {
	t.Helper()
	s, root := newStore(t)
	s.opts.Config = cfg
	cls := func(class string) string { return `"extension_class":"` + class + `"` }
	writeFile(t, root, "extensions.json", `{"tabs":[`+
		entry("base.tts", "Base TTS", cls("text-to-speech"))+`,`+
		entry("base.tool", "Base Tool", cls("tools"), `"proxy":"native"`)+`,`+
		entry("base.off", "Off", cls("text-to-speech"))+`],`+
		`"decorators":[`+
		decorator("deco.base", "Deco Base", "outer")+`,`+
		decorator("deco.inner", "Deco Inner", "inner")+`]}`)
	writeFile(t, root, "extensions.external.json", `{"tabs":[`+
		entry("ext.tts", "Ext TTS", cls("text-to-speech"))+`,`+
		entry("base.tts", "Base TTS (external)", cls("text-to-speech"))+`,`+
		`{"name":"No package","extension_type":"interface","extension_class":"text-to-speech"},`+
		`{"package_name":"wrong.type","name":"W","extension_type":"decorator","extension_class":"text-to-speech"}],`+
		`"decorators":[`+decorator("deco.ext", "Deco Ext", "outer")+`]}`)
	return s
}

func decorator(pkg, name, class string) string {
	return strings.Replace(entry(pkg, name, `"extension_class":"`+class+`"`), `"extension_type":"interface"`, `"extension_type":"decorator"`, 1)
}

func loadables(r Resolved) string {
	var out []string
	for _, l := range r.Extensions {
		s := l.PackageName
		if !l.Enabled {
			s += "(off)"
		}
		if l.Autostart {
			s += "(auto)"
		}
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

func TestResolveInterfaceOrderStateAndFilter(t *testing.T) {
	s := resolveStore(t, fakeConfig{lists: appconfig.ExtensionLists{
		Disabled:  []string{"base.off", "ext.tts"},
		Autostart: []string{"base.tool", "base.tts"},
	}})
	// external first, then base; the entry without package_name and the one of
	// another type are not listed; the disabled ones stay in the list.
	r, err := s.Resolve("interface", "text-to-speech")
	if err != nil {
		t.Fatal(err)
	}
	if got := loadables(r); got != "ext.tts(off),base.tts(auto),base.off(off)" {
		t.Fatalf("text-to-speech: %s", got)
	}
	if r.Extensions[1].Name != "Base TTS (external)" {
		t.Errorf("external entry should win: %q", r.Extensions[1].Name)
	}
	if r.DisabledNames != nil {
		t.Errorf("interface has no disabled names: %v", r.DisabledNames)
	}
	r, _ = s.Resolve("interface", "tools")
	if got := loadables(r); got != "base.tool(auto)" || r.Extensions[0].Proxy != "native" {
		t.Fatalf("tools: %s %+v", got, r.Extensions)
	}
	r, _ = s.Resolve("interface", "no-such-class")
	if r.Extensions == nil || len(r.Extensions) != 0 {
		t.Fatalf("an empty class must be an empty list, not null: %#v", r.Extensions)
	}
}

func TestResolveDecorators(t *testing.T) {
	s := resolveStore(t, fakeConfig{lists: appconfig.ExtensionLists{
		DisabledDecorators: []string{"deco.base", "decorator_save_ogg"},
		Autostart:          []string{"deco.ext"}, // autostart is not a decorator setting
	}})
	r, err := s.Resolve("decorator", "outer")
	if err != nil {
		t.Fatal(err)
	}
	if got := loadables(r); got != "deco.ext,deco.base(off)" {
		t.Fatalf("outer: %s", got)
	}
	if strings.Join(r.DisabledNames, ",") != "deco.base,decorator_save_ogg" {
		t.Errorf("disabled names: %v", r.DisabledNames)
	}
	if r, _ = s.Resolve("decorator", "inner"); loadables(r) != "deco.inner" {
		t.Fatalf("inner: %s", loadables(r))
	}
}

func TestResolveRefusals(t *testing.T) {
	s := resolveStore(t, fakeConfig{})
	if _, err := s.Resolve("nope", "tools"); !errors.Is(err, errInvalid) {
		t.Errorf("bad kind: %v", err)
	}
	if _, err := s.Resolve("interface", ""); !errors.Is(err, errInvalid) {
		t.Errorf("empty class: %v", err)
	}
	broken := resolveStore(t, fakeConfig{err: errors.New("config.extensions.disabled is not a list")})
	if _, err := broken.Resolve("interface", "tools"); err == nil {
		t.Error("an unreadable config must not read as no disabled extensions")
	}
	none := resolveStore(t, nil)
	if _, err := none.Resolve("interface", "tools"); err == nil {
		t.Error("no config source must be an error")
	}
}

func TestResolveOverHTTP(t *testing.T) {
	s := resolveStore(t, fakeConfig{lists: appconfig.ExtensionLists{Disabled: []string{"base.off"}}})
	h := s.Handler()
	code, body := call(t, h, "GET", Prefix+"resolve?kind=interface&extension_class=text-to-speech", "", "tok")
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	var got struct {
		Extensions []map[string]any `json:"extensions"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Extensions) != 3 || got.Extensions[2]["package_name"] != "base.off" || got.Extensions[2]["enabled"] != false {
		t.Fatalf("%s", body)
	}
	for _, k := range []string{"package_name", "name", "requirements", "proxy", "enabled", "autostart"} {
		if _, ok := got.Extensions[0][k]; !ok {
			t.Errorf("missing %q in %s", k, body)
		}
	}
	if code, _ := call(t, h, "GET", Prefix+"resolve?kind=interface", "", "tok"); code != http.StatusBadRequest {
		t.Errorf("missing class: %d", code)
	}
	if code, _ := call(t, h, "GET", Prefix+"resolve?kind=interface&extension_class=tools", "", "wrong"); code != http.StatusForbidden {
		t.Errorf("wrong token: %d", code)
	}
}

func TestDecoratorEnvCarriesBothClasses(t *testing.T) {
	s := resolveStore(t, fakeConfig{lists: appconfig.ExtensionLists{DisabledDecorators: []string{"deco.base"}}})
	env, err := s.DecoratorEnv()
	if err != nil || len(env) != 1 || !strings.HasPrefix(env[0], DecoratorsEnv+"=") {
		t.Fatalf("%v %v", env, err)
	}
	var both map[string]Resolved
	if err := json.Unmarshal([]byte(strings.TrimPrefix(env[0], DecoratorsEnv+"=")), &both); err != nil {
		t.Fatal(err)
	}
	if loadables(both["outer"]) != "deco.ext,deco.base(off)" || loadables(both["inner"]) != "deco.inner" {
		t.Fatalf("%+v", both)
	}
}
