package audiocpp

import "fmt"

// Version is the audio.cpp release this platform installs. It is pinned: a newer
// release is a reviewed change of this file, never "latest".
const Version = "v0.9.6"

// The builds are rows, like the torch rows of the uv platform: the user picks one
// when the platform is installed. CPU runs anywhere; Vulkan needs a GPU driver
// with Vulkan (NVIDIA, AMD and Intel all ship one).
const (
	RowCPU    = "cpu"
	RowVulkan = "vulkan"
)

// Rows lists the rows with what each costs to download (the zip's size, MB).
var Rows = []Row{
	{Name: RowCPU, Label: "CPU", ApproxMB: 34, Detail: "runs on any machine"},
	{Name: RowVulkan, Label: "Vulkan (GPU)", ApproxMB: 84, Detail: "needs a GPU driver with Vulkan"},
}

// Row is one build of the audio.cpp server.
type Row struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	ApproxMB int    `json:"approx_mb"`
	Detail   string `json:"detail"`
}

// ValidRow reports whether name is a row.
func ValidRow(name string) bool {
	for _, r := range Rows {
		if r.Name == name {
			return true
		}
	}
	return false
}

// Archive is a release asset and the sha256 GitHub lists for it.
type Archive struct {
	URL    string
	SHA256 string
	Size   int64
}

// The digests are the `digest` fields of the assets of release v0.9.6
// (https://api.github.com/repos/0xShug0/audio.cpp/releases/tags/v0.9.6), read on
// 2026-11-25; the CPU zip was also downloaded and hashed to the same value. Only
// the Windows x64 builds are pinned: the Linux and macOS ones were not run, and a
// pin nobody ran is not offered.
var archives = map[string]Archive{
	"windows/amd64/cpu":    {release("windows-x64-cpu.zip"), "9f4d9956400e9510b0ca48ce4fe576c887673ad6f6feeadc82ee679f27d794f7", 35341052},
	"windows/amd64/vulkan": {release("windows-x64-vulkan.zip"), "94c6e1df6d98176eb5f0e3ad6c16e419f548bce3c65ccd8da97abf4c3966a7aa", 87148788},
}

func release(asset string) string {
	return "https://github.com/0xShug0/audio.cpp/releases/download/" + Version + "/audio-" + Version + "-bin-" + asset
}

// ArchiveFor is the pinned archive for an OS, an architecture and a row. An
// unsupported combination is an error, never a different archive.
func ArchiveFor(goos, goarch, row string) (Archive, error) {
	if !ValidRow(row) {
		return Archive{}, fmt.Errorf("audio.cpp build %q is not one of cpu, vulkan", row)
	}
	a, ok := archives[goos+"/"+goarch+"/"+row]
	if !ok {
		return Archive{}, fmt.Errorf("no pinned audio.cpp %s build for %s/%s (only Windows x64 is pinned in this version)", Version, goos, goarch)
	}
	return a, nil
}
