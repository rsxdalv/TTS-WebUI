// Package version holds build metadata injected at link time via -ldflags -X.
package version

// These variables are overridden at build time, e.g.
//
//	-ldflags "-X <module>/internal/version.Version=1.2.3"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a one-line human readable description of the build.
func String() string {
	return Version + " (commit " + Commit + ", built " + Date + ")"
}
