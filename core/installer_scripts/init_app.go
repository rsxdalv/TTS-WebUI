package installer_scripts

import (
	"context"
	"path/filepath"
)

// InitApp ports installer_scripts/init_app.bat and init_app.sh: activate the
// install environment, then run "node init_app.js". init_app.js itself is
// ported separately (init_app_nodejs.go); this script only starts it.
//
// exitCode is the exit code of node, which is what both scripts end with.
func InitApp(ctx context.Context, s *Step) (exitCode int, err error) {
	res, err := s.Run.Run(ctx, Command{
		Name:      "node",
		Args:      []string{filepath.Join(s.Root, "installer_scripts", "init_app.js")},
		Activated: true,
	})
	if err != nil {
		return 1, err
	}
	return res.ExitCode, nil
}
