package uv

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rsxdalv/TTS-WebUI/core/internal/torchrows"
)

// venvPathBudget is how many characters the deepest file of a venv adds to the
// venv folder's own path. Measured on Windows with a real torch install (Kitten
// TTS: 138); rounded up. Windows stops at 260 characters unless long paths are on.
const venvPathBudget = 150

const windowsMaxPath = 259

// restMB is the room an install needs besides the torch row: the other packages
// of the entry, in the cache and the venv's own scripts.
const restMB = 1000

// checkPathLength refuses an install whose deepest file would pass the Windows
// path limit, before anything is downloaded.
func checkPathLength(goos, venv string) error {
	if goos != "windows" || len(venv)+venvPathBudget <= windowsMaxPath {
		return nil
	}
	return fmt.Errorf("the install folder is too deep: %s is %d characters and the engine's files need about %d more, past Windows' %d. Move the install to a shorter folder (for example C:\tts-webui)",
		venv, len(venv), venvPathBudget, windowsMaxPath)
}

// needMB is the free space an install asks for. The torch row is downloaded
// once into the cache and the venvs link to it, so it counts only while no
// installed venv uses that row yet.
func needMB(row torchrows.Row, rowShared bool) int {
	if rowShared {
		return restMB
	}
	return 2*row.ApproxMB + restMB
}

// checkDisk compares free space with what the install needs. When the free
// space cannot be read the install goes on (it cannot be checked, not refused).
func checkDisk(free uint64, known bool, row torchrows.Row, rowShared bool) error {
	if !known {
		return nil
	}
	need := needMB(row, rowShared)
	if free>>20 >= uint64(need) {
		return nil
	}
	return fmt.Errorf("not enough free disk space: %d MB free, about %d MB needed (torch row %s is about %d MB). Free some space or install on another drive",
		free>>20, need, row.Name, row.ApproxMB)
}

// rowInUse is whether an installed venv already uses the torch row, so its
// files are in the cache and a new venv adds only links.
func (p *Platform) rowInUse(row torchrows.Row) bool {
	dirs, _ := os.ReadDir(filepath.Join(p.root.Dir(), ".venvs"))
	for _, d := range dirs {
		if st, ok := p.EntryInfo(d.Name()); ok && st.TorchRow == row.Name {
			return true
		}
	}
	return false
}
