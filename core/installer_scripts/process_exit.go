package installer_scripts

import "fmt"

// processExitError ports installer_scripts/js/processExit.js. process.exit ends
// Node at once: no catch block runs, and init_app.js never prints "Finished
// init app.". The port returns this error instead, every catch passes it on
// (mustRethrow), and InitAppJS turns it into the exit code.
type processExitError struct{ code int }

func (e processExitError) Error() string { return fmt.Sprintf("process.exit(%d)", e.code) }
