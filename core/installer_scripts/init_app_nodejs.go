package installer_scripts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file ports installer_scripts/init_app.js. The name ends in _nodejs, not
// _js, because Go treats a _js suffix as a build constraint.
//
// Left out on purpose, with the reasons:
//
//   - js/server.js, js/webConsole.js, js/openBrowser.js and js/index.html: the
//     status page on 127.0.0.1:7771. The launcher always passes --silent, so no
//     browser opens it, and the launcher shows the same output in its own log.
//     Port 7771 is also the extension manager's (CONTRACTS.md, section 9). In
//     the original, a busy 7771 crashes init_app.js with EADDRINUSE; the port
//     has no listener, so that cannot happen.
//   - js/installerState.js: its state is only readable through the server's
//     /poll, so nothing could observe it.
//   - js/menu.js: the terminal menu becomes Step.Prompt (menu.go).
//   - js/shell.js $shSync: init_app.js and js/initializeApp.js never call it.
//
// Changed on purpose: the react-ui build runs the static export
// (`npm run build:static`), the only build react-ui has since Next.js was removed.
//
// Log differences that remain: no "Unified server started" or "Silent mode"
// lines; displayError(error) prints "Error: <message>" without Node's stack;
// a command that cannot start reports Go's error text instead of Node's
// "spawn X ENOENT"; output of commands that run at the same time interleaves
// differently.

// initAppVersion is the version init_app.js prints.
const initAppVersion = "0.2.0"

// InitAppJS ports installer_scripts/init_app.js. s.Root is the install root,
// the working directory of the original, and every command runs inside
// installer_files/env. exitCode is what node would exit with. err is set only
// when ctx was cancelled.
func InitAppJS(ctx context.Context, s *Step) (exitCode int, err error) {
	p := newJSProcess(ctx, s)
	defer p.flowCancel()
	defer p.stopBackground(0)

	ok, err := p.initAppMain(p.flowCtx)
	if crash := p.crashed(); crash != nil {
		return p.unhandledRejection(crash), nil
	}
	var exit processExitError
	if errors.As(err, &exit) {
		return exit.code, nil
	}
	if err != nil {
		return 1, err
	}

	p.displayMessage("\n\nFinished init app.\n")
	// setTimeout(() => processExit(result ? 0 : 1), 100): commands that were
	// never awaited get 100 ms more, then the process ends.
	p.stopBackground(100 * time.Millisecond)
	if crash := p.crashed(); crash != nil {
		return p.unhandledRejection(crash), nil
	}
	if ok {
		return 0, nil
	}
	return 1, nil
}

// unhandledRejection is how Node 22 ends on a rejected promise nobody handles:
// it prints the error to stderr and exits 1.
func (p *jsProcess) unhandledRejection(err error) int {
	p.consoleError("Error: " + err.Error())
	return 1
}

func (p *jsProcess) checkConda(ctx context.Context) error {
	err := func() error {
		p.displayMessage("Checking conda installation...")

		p.displayMessage("")
		// verify conda paths. These four are not awaited.
		p.shBackground("conda info --envs")
		p.shBackground("node --version")
		p.shBackground("python --version")
		p.shBackground("pip --version")

		return p.sh(ctx, "conda --version")
	}()
	if err != nil {
		if mustRethrow(ctx, err) {
			return err
		}
		p.displayError("Please install conda from https://docs.conda.io/projects/conda/en/latest/user-guide/install/index.html")
		return processExitError{1}
	}
	return nil
}

// getGitCommitHash uses git rev-parse HEAD, which works even if the branch is
// detached.
func (p *jsProcess) getGitCommitHash(ctx context.Context) (string, error) {
	out, err := p.execCapture(ctx, "git rev-parse HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (p *jsProcess) appliedGitVersionFile() string {
	return p.installerFile(".git_version")
}

// appliedGitVersionSave ports AppliedGitVersion.save. It is the last step of a
// successful install.
func (p *jsProcess) appliedGitVersionSave(ctx context.Context) error {
	hash, err := p.getGitCommitHash(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile(p.appliedGitVersionFile(), []byte(hash), 0o666)
}

func (p *jsProcess) syncRepo(ctx context.Context) (bool, error) {
	if !exists(filepath.Join(p.s.Root, ".git")) {
		p.displayMessage("CRITICAL: No git repository found, skipping update...")
		return false, errors.New("No git repository found")
	}

	p.displayMessage("Pulling updates from tts-webui")
	updated, err := func() (bool, error) {
		if err := p.sh(ctx, "git pull"); err != nil {
			return false, err
		}
		newHash, err := p.getGitCommitHash(ctx)
		if err != nil {
			return false, err
		}
		applied, ok, err := p.readGeneric(p.appliedGitVersionFile())
		if err != nil {
			return false, err
		}
		if ok && applied == newHash {
			p.displayMessage("Current git version: " + newHash)
			p.displayMessage("No updates found, skipping...")
			return false, nil
		}
		return true, nil
	}()
	if err != nil {
		if mustRethrow(ctx, err) {
			return false, err
		}
		p.displayMessage("There was a problem while pulling updates. Warning: missing updates might cause issues. Continuing...")
		p.displayMessage("Error:")
		p.displayError("Error: " + err.Error())
		return false, nil
	}
	return updated, nil
}

// initAppMain ports main(). The original first calls startServer(), which the
// port leaves out (see the top of this file).
func (p *jsProcess) initAppMain(ctx context.Context) (bool, error) {
	p.displayMessage("\n\nStarting init app (version: " + initAppVersion + ")...\n\n")

	ok, err := func() (bool, error) {
		if err := p.checkConda(ctx); err != nil {
			return false, err
		}
		isUpdated, err := p.syncRepo(ctx)
		if err != nil {
			return false, err
		}
		if !isUpdated {
			p.shBackground("pip show torch torchvision torchaudio")
			return true, nil
		}

		// require("./js/initializeApp.js") reads versions.json.
		if err := p.loadVersions(); err != nil {
			return false, err
		}
		if err := p.initializeApp(ctx); err != nil {
			return false, err
		}
		if err := p.setupReactUI(ctx); err != nil {
			return false, err
		}
		if err := p.repairTorch(); err != nil {
			return false, err
		}

		if err := p.appliedGitVersionSave(ctx); err != nil {
			return false, err
		}
		return true, nil
	}()
	if err != nil {
		if mustRethrow(ctx, err) {
			return false, err
		}
		p.displayError(err.Error())
		return false, nil
	}
	return ok, nil
}
