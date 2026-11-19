//go:build !windows

package react_ui_backend

// realNameHidden has nothing to add outside Windows: a file has one name, which
// the URL checks already saw.
func realNameHidden(base, full string) bool { return false }
