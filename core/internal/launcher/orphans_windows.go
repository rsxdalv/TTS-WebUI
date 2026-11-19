//go:build windows

package launcher

import "github.com/rsxdalv/TTS-WebUI/core/internal/paths"

// On Windows every app process is in a job object that closes with the
// launcher, so nothing outlives it and there is nothing to record or sweep.

func (s *Supervisor) recordProcesses() {}

func (s *Supervisor) clearRecord() {}

func sweepOrphans(paths.Root) int { return 0 }
