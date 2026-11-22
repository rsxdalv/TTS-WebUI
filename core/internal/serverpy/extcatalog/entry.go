package extcatalog

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// The Extension Catalog tab installs the entries it parsed. They come from a
// pasted text or from the catalog iframe's messages, so the requirements in
// them are untrusted input: they are validated hard before anything runs.
//
// The by-name install (POST /api/extcatalog/install) is unchanged: it takes
// the requirements from the catalog files, not from the caller. This file adds
// the entry-based install (POST /api/extcatalog/install-entry), where the
// caller hands over the entry and Go uses its own requirements and proxy.

// serveInstallEntry streams the install of one entry, like serveInstall does
// for a package name.
func (s *Store) serveInstallEntry(w http.ResponseWriter, r *http.Request) {
	o, err := body(w, r)
	if err == nil {
		err = only(o, "entry")
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	entry, ok := o.Vals["entry"].(*Obj)
	if !ok {
		s.fail(w, invalidf("entry must be an object"))
		return
	}
	if msg := validateEntry(entry); msg != "" {
		s.fail(w, invalidf("entry: %s", msg))
		return
	}
	if !s.install.TryLock() {
		s.fail(w, fmt.Errorf("%w: another install is running", errBusy))
		return
	}
	defer s.install.Unlock()
	s.stream(w, r, func(ctx context.Context, emit func(string)) bool {
		return s.installEntry(ctx, entry, emit)
	})
}

// installEntry installs one entry: its own requirements and its own proxy
// ("native" means the venv path), never a package looked up by name.
func (s *Store) installEntry(ctx context.Context, entry *Obj, emit func(string)) bool {
	name := entry.Str("name")
	if name == "" {
		name = entry.Str("package_name")
	}
	req := entry.Str("requirements")
	if req == "" || req == "builtin" {
		emit(fmt.Sprintf("%s has no requirements to install", name))
		return false
	}
	toks, err := validateRequirements(req)
	if err != nil {
		emit(fmt.Sprintf("Failed to install %s dependencies: %v", name, err))
		return false
	}
	if entry.Str("proxy") == "native" {
		return s.venvSetupTokens(ctx, entry, toks, emit)
	}
	return s.pipInstallTokens(ctx, entry, toks, emit)
}

// reqSchemes are the URL schemes a requirement may use. Everything else is
// refused: file:// and git+file:// install a local directory (its setup.py
// runs), ssh:// and git+ssh:// start a shell, http:// is not verified, and
// pip's ext:: runs any command.
var reqSchemes = map[string]bool{"https": true, "git+https": true}

// validateRequirements checks an untrusted requirements string and returns
// its tokens (split like Python's shlex.split).
//
// A requirement must be a package name with optional version specifiers
// (pkg[extra]>=1.0) or an https URL (git+https://host/repo@ref). It may not
// be an option (pip takes options from the same place), a local path, or
// anything with a shell metacharacter in it.
func validateRequirements(req string) ([]string, error) {
	toks, err := shellSplit(req)
	if err != nil {
		return nil, fmt.Errorf("requirements refused: bad requirements: %v", err)
	}
	if len(toks) == 0 {
		return nil, fmt.Errorf("requirements refused: no requirements")
	}
	for _, t := range toks {
		if err := checkRequirement(t); err != nil {
			return nil, err
		}
	}
	return toks, nil
}

// checkRequirement refuses one requirement token.
func checkRequirement(tok string) error {
	refuse := func(reason string) error {
		return fmt.Errorf("requirements refused: %s %q", reason, tok)
	}
	if tok == "" {
		return refuse("empty requirement")
	}
	if strings.HasPrefix(tok, "-") {
		return refuse("option-looking value")
	}
	for _, r := range tok {
		if r < 0x20 || r == 0x7f {
			return refuse("control character in")
		}
		switch r {
		case ';', '&', '|', '$', '`':
			return refuse("shell metacharacter in")
		}
	}
	if strings.Contains(tok, "ext::") {
		return refuse("pip extension requirement")
	}
	if i := strings.Index(tok, "://"); i >= 0 {
		scheme := tok[:i]
		if !reqSchemes[scheme] {
			return fmt.Errorf("requirements refused: URL scheme %q in %q (allowed: https, git+https)", scheme, tok)
		}
		return nil
	}
	// No scheme: a plain name, not a path. A path would make pip run the
	// setup.py of a local directory.
	if strings.ContainsAny(tok, `/\`) || strings.HasPrefix(tok, ".") {
		return refuse("local path")
	}
	if isDriveLetter(tok) {
		return refuse("local path")
	}
	return nil
}

// isDriveLetter reports whether tok starts with a Windows drive letter
// ("C:..."), which pip would take as a local path.
func isDriveLetter(tok string) bool {
	if len(tok) < 2 || tok[1] != ':' {
		return false
	}
	c := tok[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
