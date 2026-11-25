package extcatalog

import (
	"strings"
	"testing"
)

func entryOf(t *testing.T, js string) *Obj {
	t.Helper()
	v, err := Parse([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	return v.(*Obj)
}

func TestEntryEnvReadsTheEnvObject(t *testing.T) {
	e := entryOf(t, `{"package_name":"p","env":{"TORCH_FORCE_NO_WEIGHTS_ONLY_LOAD":"1","A_B":"x"}}`)
	got, err := entryEnv("p", e)
	if err != nil || strings.Join(got, ",") != "A_B=x,TORCH_FORCE_NO_WEIGHTS_ONLY_LOAD=1" {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, err := entryEnv("p", entryOf(t, `{"package_name":"p"}`)); err != nil || got != nil {
		t.Fatalf("no env: %v, %v", got, err)
	}
}

func TestEntryEnvRefusesWhatAnEngineMayNotSet(t *testing.T) {
	for _, js := range []string{
		`{"env":{"PATH":"x"}}`, `{"env":{"PYTHONPATH":"x"}}`, `{"env":{"TTS_CORE_TOKEN":"x"}}`,
		`{"env":{"lower":"x"}}`, `{"env":{"A":1}}`, `{"env":["A=1"]}`,
	} {
		if got, err := entryEnv("p", entryOf(t, js)); err == nil {
			t.Errorf("%s accepted: %v", js, got)
		}
	}
}
