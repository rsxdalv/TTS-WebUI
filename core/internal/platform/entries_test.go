package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeCmd records the argv of every command and runs a program that cannot
// fail (the test binary's own "true"-like behaviour is not needed: Run is
// replaced by pointing Program at a missing file when fail is set).
type fakeCmd struct {
	argvs [][]string
	fail  int // 1-based index of the command that cannot start; 0 = none
}

func (f *fakeCmd) cmd(e *Entry, argv []string) (Command, error) {
	f.argvs = append(f.argvs, append([]string(nil), argv...))
	if f.fail == len(f.argvs) {
		return Command{}, errors.New("boom")
	}
	return Command{}, errors.New("recorded")
}

func lines() (*[]string, Emit) {
	var out []string
	return &out, func(l string) { out = append(out, l) }
}

func TestPipInstallCommandAndMessages(t *testing.T) {
	f := &fakeCmd{}
	x := Entries{Root: t.TempDir(), Cmd: f.cmd}
	out, emit := lines()
	ok := x.PipInstall(context.Background(), Entry{Package: "p", Name: "P", Requirements: []string{"p>=1", "q"}}, false, emit)
	if ok {
		t.Fatal("a command that cannot be resolved must fail the install")
	}
	want := []string{"python", "-m", "pip", "install", "p>=1", "q", GradioPin}
	if !reflect.DeepEqual(f.argvs[0], want) {
		t.Fatalf("argv = %q, want %q", f.argvs[0], want)
	}
	if got := (*out)[len(*out)-1]; got != "Failed to install P dependencies" {
		t.Fatalf("last line = %q", got)
	}
	f = &fakeCmd{}
	x.Cmd = f.cmd
	x.PipInstall(context.Background(), Entry{Package: "p", Requirements: []string{"p"}}, true, func(string) {})
	if f.argvs[0][4] != "--dry-run" && f.argvs[0][3] != "--dry-run" {
		t.Fatalf("dry run lacks --dry-run: %q", f.argvs[0])
	}
}

func TestVenvSetupStepsAndRefusals(t *testing.T) {
	root := t.TempDir()
	f := &fakeCmd{}
	x := Entries{Root: root, Cmd: f.cmd}
	out, emit := lines()
	if x.Create(context.Background(), Entry{Package: "../x", Own: true, Requirements: []string{"x"}}, emit) {
		t.Fatal("a bad package name must be refused")
	}
	if len(f.argvs) != 0 || !strings.Contains((*out)[len(*out)-1], "is not a valid package name for a virtual environment") {
		t.Fatalf("ran %v, last %q", f.argvs, (*out)[len(*out)-1])
	}
	out, emit = lines()
	if x.Create(context.Background(), Entry{Package: "e", Own: true}, emit) || (*out)[len(*out)-1] != "Failed: bad requirements for e" {
		t.Fatalf("no requirements: %q", *out)
	}
	e := Entry{Package: "e", Name: "E", Own: true, Requirements: []string{"e"}, Torch: []string{"torch==1", "--index-url", "u"}}
	x.Create(context.Background(), e, func(string) {})
	// The first step cannot run, so only it was tried.
	if len(f.argvs) != 1 || !reflect.DeepEqual(f.argvs[0], []string{"uv", "venv", VenvDir(root, "e"), "--allow-existing"}) {
		t.Fatalf("steps = %q", f.argvs)
	}
}

func TestInstalledIn(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(VenvDir(root, "own"), 0o755); err != nil {
		t.Fatal(err)
	}
	dists := map[string]string{NormalizeDist("My-Pkg"): "1.2"}
	cases := []struct {
		e    Entry
		want EntryStatus
	}{
		{Entry{Package: "own", Own: true}, EntryStatus{Installed: true}},
		{Entry{Package: "gone", Own: true}, EntryStatus{}},
		{Entry{Package: "extensions.builtin.x"}, EntryStatus{Installed: true}},
		{Entry{Package: "my_pkg"}, EntryStatus{Installed: true, Version: "1.2"}},
		{Entry{Package: "other"}, EntryStatus{}},
	}
	for _, c := range cases {
		if got := InstalledIn(root, c.e, dists); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.e.Package, got, c.want)
		}
	}
}

func TestOwnVenvCommand(t *testing.T) {
	root := t.TempDir()
	e := &Entry{Package: "e", Own: true}
	if _, ok := OwnVenvCommand(root, e, []string{"python", "-V"}, nil); ok {
		t.Fatal("no venv yet")
	}
	py := VenvPython(root, "e")
	_ = os.MkdirAll(filepath.Dir(py), 0o755)
	_ = os.WriteFile(py, nil, 0o755)
	c, ok := OwnVenvCommand(root, e, []string{"python", "-V"}, []string{"K=V"})
	if !ok || c.Program != py || !reflect.DeepEqual(c.Args, []string{"-V"}) || c.Env[0] != "K=V" {
		t.Fatalf("got %+v %v", c, ok)
	}
	if _, ok := OwnVenvCommand(root, e, []string{"uv", "venv"}, nil); ok {
		t.Fatal("uv is not the venv's")
	}
	if _, ok := OwnVenvCommand(root, nil, []string{"python"}, nil); ok {
		t.Fatal("no entry, no venv")
	}
}
