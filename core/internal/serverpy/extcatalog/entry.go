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
	return s.pipInstallTokens(ctx, entry, toks, false, emit)
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
//
// The rule is a whitelist, not a blacklist: a token is either a plain package
// name (PEP 508 name, extras and version specifiers) or an https URL. Every
// other shape is refused, so nothing that pip would treat as a path, a URL of
// another scheme or an option can get through.
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
		// The rest of the URL: a host, a path and an optional @ref.
		if !validHTTPSURL(tok[i+3:]) {
			return refuse("malformed URL in")
		}
		return nil
	}
	// No scheme: a plain package name, not a path and not a direct URL.
	if !validPackageName(tok) {
		return refuse("not a package name")
	}
	return nil
}

// validPackageName reports whether tok is a PEP 508 name with optional
// extras and version specifiers: "name", "name[extra]", "name>=1.0",
// "name[extra]==1.0", "name>=1.0,<2.0".
func validPackageName(tok string) bool {
	name, rest := tok, ""
	if i := strings.IndexAny(tok, "[<>=!~;"); i >= 0 {
		name, rest = tok[:i], tok[i:]
	}
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
			if i == 0 || i == len(name)-1 {
				return false
			}
		default:
			// A space, a backslash, a slash, an "@": not a package name.
			return false
		}
	}
	if rest == "" {
		return true
	}
	// The extras: "[a,b]" and nothing else.
	if strings.HasPrefix(rest, "[") {
		j := strings.Index(rest, "]")
		if j < 0 {
			return false
		}
		for _, e := range strings.Split(rest[1:j], ",") {
			if e == "" {
				return false
			}
			for i, r := range e {
				switch {
				case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				case r == '-' || r == '_' || r == '.':
					if i == 0 || i == len(e)-1 {
						return false
					}
				default:
					return false
				}
			}
		}
		rest = rest[j+1:]
	}
	if rest == "" {
		return true
	}
	// The specifiers: ">=1.0", "==1.0", "<2.0", "!=1.0", "~=1.0", each with a
	// version number that may hold letters, digits and dots.
	for _, spec := range strings.Split(rest, ",") {
		if spec == "" {
			return false
		}
		op := ""
		for _, o := range []string{"===", "==", ">=", "<=", "!=", "~=", ">", "<"} {
			if strings.HasPrefix(spec, o) {
				op = o
				break
			}
		}
		if op == "" {
			return false
		}
		ver := spec[len(op):]
		if ver == "" {
			return false
		}
		for _, r := range ver {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			case r == '.' || r == '*' || r == '+' || r == '-' || r == '!':
			default:
				return false
			}
		}
	}
	return true
}

// validHTTPSURL checks the part of an https URL that follows the scheme:
// a host, then a path, then an optional @ref.
//
// The "@" of a git ref (git+https://host/repo@main) is in the path, not in
// the userinfo: only an "@" before the first "/" is a userinfo.
func validHTTPSURL(rest string) bool {
	if rest == "" {
		return false
	}
	// A userinfo ("user@host") is allowed, but only with a plain name.
	if i := strings.Index(rest, "@"); i >= 0 {
		if j := strings.IndexAny(rest, "/?#"); j < 0 || i < j {
			user := rest[:i]
			if user == "" || strings.ContainsAny(user, "/\\ ") {
				return false
			}
			rest = rest[i+1:]
		}
	}
	// The host: letters, digits, dots and dashes (an IP is digits and dots).
	host, path := rest, ""
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		host, path = rest[:i], rest[i:]
	}
	if host == "" {
		return false
	}
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == ':':
		default:
			return false
		}
	}
	// The path: no spaces, no backslashes (a Windows path), no "..".
	for _, r := range path {
		if r <= 0x20 || r == 0x7f || r == '\\' {
			return false
		}
	}
	if strings.Contains(path, "..") {
		return false
	}
	return true
}
