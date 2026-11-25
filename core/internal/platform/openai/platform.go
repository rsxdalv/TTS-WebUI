package openai

import (
	"context"
	"errors"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Platform is the openai platform behind the platform seam (D23). It is built
// in: always ready, nothing to install, and an entry on it has no process, so
// Command has nothing to resolve. The endpoints are the user's, in the Store.
type Platform struct{}

var _ platform.Platform = Platform{}

// New returns the platform.
func New() Platform { return Platform{} }

var errNoProcess = errors.New("the openai platform runs no process: its entries are remote endpoints")

func (Platform) Name() string       { return Name }
func (Platform) Provides() []string { return []string{Name} }

// Status is always ready: the platform is code in this binary.
func (Platform) Status(context.Context) platform.Status { return platform.Status{Ready: true} }

func (Platform) Install(context.Context, platform.Emit) error {
	return platform.ErrNotInstallable
}
func (Platform) Repair(context.Context, platform.Emit) error {
	return platform.ErrNotInstallable
}
func (Platform) Uninstall(context.Context, platform.Emit) error {
	return platform.ErrNotInstallable
}

// Create, Check and Remove do nothing: an endpoint is added in the Store.
func (Platform) Create(_ context.Context, _ platform.Entry, emit platform.Emit) bool {
	emit("an openai entry is a remote endpoint: add it on the openai page")
	return true
}
func (Platform) Check(_ context.Context, _ platform.Entry, emit platform.Emit) bool {
	emit("nothing to install on the openai platform")
	return true
}
func (Platform) EntryStatus(context.Context, platform.Entry) platform.EntryStatus {
	return platform.EntryStatus{Installed: true}
}
func (Platform) Remove(context.Context, platform.Entry, platform.Emit) bool { return true }

func (p Platform) WithEnv([]string) platform.Platform { return p }

func (Platform) Command(*platform.Entry, []string) (platform.Command, error) {
	return platform.Command{}, errNoProcess
}
