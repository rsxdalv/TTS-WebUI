package uv

import "fmt"

// Tool is a program the engines need and the platform provides (D26): where it
// comes from, how it is checked, and which of its folders go on the engine's PATH.
//
// The pins were read on 2026-11-24. An unsupported OS or architecture has no row
// and the platform says so (it never picks another archive). "System" rows are
// tools the platform does not fetch (see INTEGRATION.md for why).
type Tool struct {
	Name    string
	Version string
	Archive Archive
	// Strip is how many leading path components of every archive member are dropped
	// (a release archive's single top folder).
	Strip int
	// Path are folders of the extracted tool (relative, slash separated) put first on
	// PATH. Lib are folders put on LD_LIBRARY_PATH (Linux): a package such as
	// torchcodec loads ffmpeg's shared libraries, not the program.
	Path, Lib []string
	// Probe is a file of the extracted tool that must exist for it to count as there.
	Probe string
}

// ffmpeg is a BtbN FFmpeg-Builds "shared" build of the 2026-10-31 monthly
// autobuild (the monthly ones are kept; the daily ones are removed). The checksums
// are the `digest` field of each asset of the release, read from the GitHub API on
// 2026-11-24; the Windows one was also computed from a real download.
const (
	ffmpegVersion = "8.1.3"
	ffmpegRelease = "https://github.com/BtbN/FFmpeg-Builds/releases/download/autobuild-2026-10-31-12-59/"
	// MinGit comes from git-for-windows; the checksum is the asset's `digest`.
	gitVersion = "2.56.1"
	gitRelease = "https://github.com/git-for-windows/git/releases/download/v2.56.1.windows.1/"
)

var ffmpegTools = map[string]Tool{
	"windows/amd64": {Name: "ffmpeg", Version: ffmpegVersion, Strip: 1,
		Archive: Archive{ffmpegRelease + "ffmpeg-n8.1.3-27-g8dca1586f4-win64-gpl-shared-8.1.zip", "3adc00b67118a8c4bdd6624a000e139df283b1eeb8b126fcec4849bd5b778dd4"},
		Path:    []string{"bin"}, Probe: "bin/ffmpeg.exe"},
	"linux/amd64": {Name: "ffmpeg", Version: ffmpegVersion, Strip: 1,
		Archive: Archive{ffmpegRelease + "ffmpeg-n8.1.3-27-g8dca1586f4-linux64-gpl-shared-8.1.tar.xz", "202fe67679a0860b155817a0907ffcc5c1ea82e186acaa21cb87d307f54fd59a"},
		Path:    []string{"bin"}, Lib: []string{"lib"}, Probe: "bin/ffmpeg"},
	"linux/arm64": {Name: "ffmpeg", Version: ffmpegVersion, Strip: 1,
		Archive: Archive{ffmpegRelease + "ffmpeg-n8.1.3-27-g8dca1586f4-linuxarm64-gpl-shared-8.1.tar.xz", "9d3161962e57f0f621b87c1fe4287ad14b4fd12c74c243436143319e4e14a0a0"},
		Path:    []string{"bin"}, Lib: []string{"lib"}, Probe: "bin/ffmpeg"},
}

var gitTools = map[string]Tool{
	"windows/amd64": {Name: "git", Version: gitVersion,
		Archive: Archive{gitRelease + "MinGit-2.56.1-64-bit.zip", "4a92850b6e917da4fd010669b614b55dccf5b0ec333a534cc4be4700dfefdafd"},
		Path:    []string{"cmd"}, Probe: "cmd/git.exe"},
	"windows/arm64": {Name: "git", Version: gitVersion,
		Archive: Archive{gitRelease + "MinGit-2.56.1-arm64.zip", "96571e442e90523a90e5fd8661b4c4e43bc192910f3c6db938816e6955b4cc8c"},
		Path:    []string{"cmd"}, Probe: "cmd/git.exe"},
}

// ToolNames are the tools of the platform, in install order.
var ToolNames = []string{"git", "ffmpeg"}

// ToolFor is the pinned tool for an OS and architecture. ok is false when the
// platform does not fetch it there: git on Linux and macOS (the system's is
// used) and ffmpeg on macOS and Windows arm64 (no pinned build).
func ToolFor(name, goos, goarch string) (Tool, bool) {
	key := goos + "/" + goarch
	switch name {
	case "git":
		t, ok := gitTools[key]
		return t, ok
	case "ffmpeg":
		t, ok := ffmpegTools[key]
		return t, ok
	}
	return Tool{}, false
}

// gitInstallHint is how to get the system's git, for the OS the platform does not fetch it on.
func gitInstallHint(goos string) string {
	switch goos {
	case "linux":
		return "Install it with your package manager: apt install git (Debian, Ubuntu), dnf install git (Fedora), pacman -S git (Arch), apk add git (Alpine). In Docker or Colab, install it in the image"
	case "darwin":
		return "Install it with xcode-select --install (or brew install git)"
	}
	return "Install Git for your system and put it on PATH"
}

// systemToolHint is what the install says when a tool is not fetched.
func systemToolHint(name, goos string) string {
	switch {
	case name == "git" && goos == "linux":
		return "git: not fetched on Linux (there is no official static build); the system's git is used. " + gitInstallHint(goos)
	case name == "git" && goos == "darwin":
		return "git: not fetched on macOS; the system's git is used. " + gitInstallHint(goos)
	case name == "ffmpeg" && goos == "darwin":
		return "ffmpeg: no pinned build for macOS; engines find the system's (brew install ffmpeg)"
	}
	return fmt.Sprintf("%s: no pinned build for this OS and architecture; engines find the system's", name)
}
