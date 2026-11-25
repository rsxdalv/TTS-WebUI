package uv

import "fmt"

// Version is the uv this platform installs. It is pinned: a newer uv is a
// reviewed change of this file, never "latest".
const Version = "0.13.6"

// PythonVersion is the Python every venv gets unless its entry names another
// one (D25). uv downloads it into platforms/uv/python/.
const PythonVersion = "3.10.11"

// Archive is a uv release archive and the checksum its publisher lists for it.
type Archive struct {
	URL    string
	SHA256 string
}

// The checksums are the ones in the `.sha256` file next to each asset of the
// 0.13.6 release (https://github.com/astral-sh/uv/releases/tag/0.13.6), read on
// 2026-11-24.
var archives = map[string]Archive{
	"windows/amd64": {release("x86_64-pc-windows-msvc.zip"), "a2a8820d5eea32f08ef467ef3b7f66883a9b076d5737a2fc2fbb5f458d1bb50f"},
	"windows/arm64": {release("aarch64-pc-windows-msvc.zip"), "c2426c0c8e28b53d8c440e47a18d96ad4db51e159866c0e81435b196d7f0081c"},
	"linux/amd64":   {release("x86_64-unknown-linux-gnu.tar.gz"), "f29c3e94ffabd7e6f3ba521dba169a9b7ec12100bf3257087f0f24f6123f3645"},
	"linux/arm64":   {release("aarch64-unknown-linux-gnu.tar.gz"), "229e194255c7958d86d56e8790d969a6f74736592133f0dfd0076f754c642c1d"},
	"darwin/arm64":  {release("aarch64-apple-darwin.tar.gz"), "60fbd5c8a1ed681845eea28d9d622fcde03c51e91a43465b64a1e17db1ed3472"},
	"darwin/amd64":  {release("x86_64-apple-darwin.tar.gz"), "63ba17fdda55ad1a230d4b37fc6017ffa10c23688c83818bd3bfdf914c652a02"},
}

func release(asset string) string {
	return "https://github.com/astral-sh/uv/releases/download/" + Version + "/uv-" + asset
}

// ArchiveFor is the pinned archive for an OS and architecture. An unsupported
// pair is an error, never a different archive.
func ArchiveFor(goos, goarch string) (Archive, error) {
	a, ok := archives[goos+"/"+goarch]
	if !ok {
		return Archive{}, fmt.Errorf("no pinned uv for %s/%s", goos, goarch)
	}
	return a, nil
}
