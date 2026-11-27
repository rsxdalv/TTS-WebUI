package extcatalog

import "errors"

// ErrInstallRunning is what WhileNoInstall returns when an install is running.
var ErrInstallRunning = errors.New("an install is running")

// WhileNoInstall runs fn while no install can start, and not at all when one is
// running (it returns ErrInstallRunning). The launcher's cleanup actions (remove
// the uv platform, clear the wheel cache) use it so they never run under an
// install that is using the same files.
func (s *Store) WhileNoInstall(fn func() error) error {
	if !s.install.TryLock() {
		return ErrInstallRunning
	}
	defer s.install.Unlock()
	return fn()
}
