package installer_scripts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/torchrows"
)

// This file ports installer_scripts/js/initializeApp.js: python version, torch,
// pip packages, extensions, database config and the react-ui build. Its three
// exports (initializeApp, setupReactUI, repairTorch) are called only from
// init_app.js, so they stay unexported here and InitAppJS calls them.

// Version pins. These are the only copies in the Go port; keep them in step
// with js/initializeApp.js while the original is still the default.
const (
	torchVersion    = torchrows.TorchVersion // the torch table (internal/torchrows) is the one copy
	cudaVersion     = torchrows.CUDAVersion
	cudaVersionTag  = torchrows.CUDATag
	xformersVersion = torchrows.XformersVersion // NVIDIA only; the ROCm line installs xformers unpinned
	torchPin        = "torch==" + torchVersion

	pythonVersion = "3.10.11" // 3.11 has limited support, 3.12 is not yet supported

	// majorVersion is written to .major_version after torch installs.
	majorVersion = "1.0.0"

	extensionsIndex = "https://tts-webui.github.io/extensions-index/"

	// reactUIBuild builds the UI: the static export (Vite) that Go serves
	// (CONTRACTS.md section 10). It replaces frontend/out, so the launcher
	// refuses these steps while the app runs.
	reactUIBuild = "cd frontend && npm run build:static"
)

var rocmVersionTag = map[string]string{
	"2.6.0":                "rocm6.2.4",
	"2.7.0":                "rocm6.3",
	"2.7.1":                "rocm6.3",
	torchrows.TorchVersion: torchrows.ROCmTag,
}

// extensions are installed with the core packages. The quotes are part of the
// shell line.
var extensions = []struct{ name, pkg string }{
	{"RVC", `"tts-webui-extension.rvc>=0.0.5"`},
	{"StyleTTS", `"tts-webui-extension.styletts2>=0.1.1"`}, // pypi
}

// PyTorchChoice labels. They are also the content of the .gpu marker.
const (
	choiceNVIDIA                = "NVIDIA GPU"
	choiceCustom                = "Custom (Manual torch install)"
	choiceAppleMSeries          = "Apple M Series Chip"
	choiceAMDROCm               = "AMD GPU (ROCM, Linux only)"
	choiceIntelXPU              = "Intel GPU (XPU)"
	choiceCPU                   = "CPU"
	choiceCancel                = "Cancel"
	choiceIntegratedUnsupported = "Integrated GPU (unsupported)"
)

func (p *jsProcess) ensurePythonVersion(ctx context.Context) error {
	err := func() error {
		p.displayMessage("Checking python version...")
		version, err := p.getPythonVersion(ctx)
		if err != nil {
			return err
		}
		if version != "Python "+pythonVersion {
			p.displayMessage(`Current python version is """` + version + `"""`)
			p.displayMessage("Python version is not " + pythonVersion + ". Reinstalling...")
			pythonPackage := "python=" + pythonVersion
			conda := "micromamba"
			err := p.dollar(ctx, conda+" install -y -c conda-forge "+pythonPackage)
			// The environment changed, so capture its activation again.
			p.s.ResetActivated()
			return err
		}
		return nil
	}()
	if err != nil {
		if mustRethrow(ctx, err) {
			return err
		}
		p.displayError("Failed to check/install python version")
	}
	return nil
}

func (p *jsProcess) getPythonVersion(ctx context.Context) (string, error) {
	if err := p.sh(ctx, "python --version > installer_scripts/.python_version"); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(p.installerFile(".python_version"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func (p *jsProcess) installDependencies(ctx context.Context, gpuchoice string) error {
	err := func() error {
		var err error
		switch gpuchoice {
		case choiceNVIDIA:
			err = p.dollar(ctx, "pip install -U "+torchPin+"+"+cudaVersionTag+" torchvision torchaudio=="+torchVersion+
				" xformers=="+xformersVersion+" --index-url https://download.pytorch.org/whl/"+cudaVersionTag)
		case choiceCustom:
			p.displayMessage("Please install torch manually")
			p.displayMessage("For example with CUDA " + cudaVersion + " use: pip install " + torchPin + "+" + cudaVersionTag +
				" torchvision torchaudio==" + torchVersion + " --index-url https://download.pytorch.org/whl/" + cudaVersionTag)
		case choiceAppleMSeries:
			err = p.dollar(ctx, "pip install "+torchPin+" torchvision torchaudio=="+torchVersion)
		case choiceCPU:
			err = p.dollar(ctx, "pip install "+torchPin+"+cpu torchvision torchaudio=="+torchVersion+
				" --index-url https://download.pytorch.org/whl/cpu")
		case choiceAMDROCm:
			tag, ok := rocmVersionTag[torchVersion]
			if !ok {
				tag = "undefined"
			}
			err = p.dollar(ctx, "pip install "+torchPin+" torchvision torchaudio=="+torchVersion+
				" xformers --index-url https://download.pytorch.org/whl/"+tag)
		case choiceIntelXPU:
			err = p.dollar(ctx, "pip install "+torchPin+" torchvision torchaudio=="+torchVersion+
				" --index-url https://download.pytorch.org/whl/test/xpu")
		default:
			p.displayMessage("Unsupported or cancelled. Exiting...")
			if err := p.removeGPUChoice(); err != nil {
				return err
			}
			return processExitError{1}
		}
		if err != nil {
			return err
		}

		if err := p.saveGeneric(p.installerFile(".major_version"), majorVersion); err != nil {
			return err
		}
		p.displayMessage("  Successfully installed " + torchPin + " with CUDA " + cudaVersion + " support")
		return nil
	}()
	if err != nil && !mustRethrow(ctx, err) {
		p.displayError("Error during installation: " + err.Error())
	}
	return err
}

func (p *jsProcess) askForGPUChoice(ctx context.Context) (string, error) {
	return p.menu(ctx, []string{
		choiceNVIDIA,
		choiceAppleMSeries,
		choiceCPU,
		choiceAMDROCm,
		choiceIntelXPU,
		choiceCancel,
		choiceCustom,
		choiceIntegratedUnsupported,
	}, `
These are not yet automatically supported: AMD GPU, Intel GPU, Integrated GPU.
Select the device (GPU/CPU) you are using to run the application:
(use arrow keys to move, enter to select)
  `)
}

// installerFile ports getInstallerFilesPath: a file in installer_scripts/.
func (p *jsProcess) installerFile(name string) string {
	return filepath.Join(p.s.Root, "installer_scripts", name)
}

// readGeneric returns the raw content of file and true, or false when it does
// not exist (the original returns -1, which never equals a version string).
func (p *jsProcess) readGeneric(file string) (string, bool, error) {
	if !exists(file) {
		return "", false, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", false, err
	}
	return string(raw), true, nil
}

func (p *jsProcess) saveGeneric(file, data string) error {
	return os.WriteFile(file, []byte(data), 0o666)
}

func (p *jsProcess) removeGPUChoice() error {
	gpuFile := p.installerFile(".gpu")
	if exists(gpuFile) {
		return os.Remove(gpuFile)
	}
	return nil
}

// exists ports fs.existsSync: false for any error.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (p *jsProcess) pipInstallOrFail(ctx context.Context, requirements, name string, pipFallback bool) error {
	label := name
	if label == "" {
		label = requirements
	}
	p.displayMessage("Installing " + label + " dependencies...")
	pip := "uv pip"
	if pipFallback {
		pip = "pip"
	}
	if err := p.sh(ctx, pip+" install "+requirements+" "+torchPin); err != nil {
		return err
	}
	p.displayMessage("Successfully installed " + label + " dependencies\n")
	return nil
}

func (p *jsProcess) pipInstall(ctx context.Context, requirements, name string, pipFallback bool) error {
	if err := p.pipInstallOrFail(ctx, requirements, name, pipFallback); err != nil {
		if mustRethrow(ctx, err) {
			return err
		}
		label := name
		if label == "" {
			label = requirements
		}
		p.displayMessage("Failed to install " + label + " dependencies\n")
	}
	return nil
}

func constructDependencyInstallString() string {
	pkgs := make([]string, len(extensions))
	for i, ext := range extensions {
		pkgs[i] = ext.pkg
	}
	return "-r requirements.txt " + strings.Join(pkgs, " ") + " --extra-index-url " + extensionsIndex
}

// pipInstallAll: the first install is a temporary safeguard due to mysterious
// issues with uv.
func (p *jsProcess) pipInstallAll(ctx context.Context, fi bool) error {
	pipPackagesFile := p.installerFile(".pip_packages")
	current, ok, err := p.readGeneric(pipPackagesFile)
	if err != nil {
		return err
	}
	if ok && current == p.newPipPackagesVersion {
		p.displayMessage("Dependencies are already up to date, skipping pip installs...")
		return nil
	}

	singleInstall := func() error {
		p.displayMessage("Attempting single pip install of all dependencies...")
		if err := p.pipInstallOrFail(ctx, constructDependencyInstallString(), "All dependencies", true); err != nil {
			return err
		}
		if err := p.saveGeneric(pipPackagesFile, p.newPipPackagesVersion); err != nil {
			return err
		}
		p.displayMessage("")
		return nil
	}
	err = singleInstall()
	if err == nil {
		return nil
	}
	if mustRethrow(ctx, err) {
		return err
	}
	p.displayMessage("Failed to install all dependencies, falling back to individual installs...")

	p.displayMessage("Updating dependencies...")

	if err := p.pipInstallOrFail(ctx, "-r requirements.txt", "Core Packages", fi); err != nil {
		if !mustRethrow(ctx, err) {
			p.displayMessage("Failed to install core packages")
			p.displayMessage("Please check the log file for more information")
			p.displayMessage("Exiting...")
		}
		return err
	}
	for _, ext := range extensions {
		if err := p.pipInstall(ctx, ext.pkg+" --extra-index-url "+extensionsIndex, ext.name, fi); err != nil {
			return err
		}
	}
	if err := p.saveGeneric(pipPackagesFile, p.newPipPackagesVersion); err != nil {
		return err
	}
	p.displayMessage("")
	return nil
}

func (p *jsProcess) checkIfTorchInstalled(ctx context.Context) (bool, error) {
	err := p.dollarArgs(ctx, []string{
		"python",
		"-c",
		`import importlib.util; import sys; package_name = "torch"; spec = importlib.util.find_spec(package_name); sys.exit(0) if spec else sys.exit(1)`,
	})
	if err != nil {
		if mustRethrow(ctx, err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (p *jsProcess) getGPUChoice(ctx context.Context) (string, error) {
	gpuFile := p.installerFile(".gpu")
	if exists(gpuFile) {
		gpuchoice, _, err := p.readGeneric(gpuFile)
		if err != nil {
			return "", err
		}
		p.displayMessage("  Using saved GPU choice: " + gpuchoice)
		return gpuchoice, nil
	}
	gpuchoice, err := p.askForGPUChoice(ctx)
	if err != nil {
		return "", err
	}
	p.displayMessage("  You selected: " + gpuchoice)
	if err := p.saveGeneric(gpuFile, gpuchoice); err != nil {
		return "", err
	}
	return gpuchoice, nil
}

func (p *jsProcess) applyCondaConfig(ctx context.Context) error {
	p.displayMessage("Applying conda config...")
	p.displayMessage("  Checking if Torch is installed...")
	current, ok, err := p.readGeneric(p.installerFile(".major_version"))
	if err != nil {
		return err
	}
	if ok && current == majorVersion {
		installed, err := p.checkIfTorchInstalled(ctx)
		if err != nil {
			return err
		}
		if installed {
			p.displayMessage("  Torch is already installed. Skipping installation...")
			return p.pipInstallAll(ctx, false)
		}
		p.displayMessage("  Torch is not installed. Starting installation...\n")
	} else {
		p.displayMessage("  Major version update detected. Upgrading base environment")
	}

	gpuchoice, err := p.getGPUChoice(ctx)
	if err != nil {
		return err
	}
	if err := p.installDependencies(ctx, gpuchoice); err != nil {
		return err
	}
	return p.pipInstallAll(ctx, true) // approximate first install
}

func (p *jsProcess) chooseExtensions() {
	p.displayMessage("Choose extensions to install...")
}

// initializeApp ports exports.initializeApp.
func (p *jsProcess) initializeApp(ctx context.Context) error {
	p.displayMessage("Ensuring that python has the correct version...")
	if err := p.ensurePythonVersion(ctx); err != nil {
		return err
	}
	p.displayMessage("")
	if err := p.applyCondaConfig(ctx); err != nil {
		return err
	}
	p.displayMessage("")
	p.chooseExtensions()
	p.displayMessage("")
	// The I5 port. Its upgrade path always fails, as the original's does
	// (CONTRACTS.md, section 9); that failure is logged and the flow carries on.
	if err := applyDatabaseConfig(p.s); err != nil {
		p.displayError("Failed to apply database config")
	} else {
		p.displayMessage("")
	}
	return nil
}

func (p *jsProcess) checkIfTorchHasCudaBackground() {
	// The body of checkIfTorchHasCuda up to its first await runs at once. Its
	// failure is caught inside it, so it never ends the process.
	p.displayMessage("Checking if torch has CUDA...")
	cmd := []string{"python", "-c", "import torch; exit(0 if torch.cuda.is_available() else 1)"}
	p.consoleLog(">" + strings.Join(cmd, " "))
	p.background(func(ctx context.Context) error { return p.runCommand(ctx, cmd[0], cmd[1:]) }, false)
}

// repairTorch ports exports.repairTorch. Neither command is awaited, and
// `!checkIfTorchHasCuda()` tests a Promise, which is never falsy, so torch is
// never repaired. The port keeps that (CONTRACTS.md, section 9).
func (p *jsProcess) repairTorch() error {
	// readGPUChoice: its value only feeds the condition that is always false,
	// but a read error still throws.
	if _, _, err := p.readGeneric(p.installerFile(".gpu")); err != nil {
		return err
	}
	p.shBackground("pip show torch torchvision torchaudio")
	p.checkIfTorchHasCudaBackground()
	return nil
}

func (p *jsProcess) setupReactUIExtensions() error {
	p.displayMessage("Initializing extensions...")
	packageJSONpath := filepath.Join(p.s.Root, "frontend", "src", "extensions", "package.json")
	if !exists(packageJSONpath) {
		if err := os.WriteFile(packageJSONpath, []byte("{}"), 0o666); err != nil {
			p.displayMessage("Failed to install extensions")
			return err
		}
	}
	return nil
}

// setupReactUI ports exports.setupReactUI.
func (p *jsProcess) setupReactUI(ctx context.Context) error {
	err := func() error {
		if err := p.setupReactUIExtensions(); err != nil {
			return err
		}
		for _, dir := range []string{"outputs", "favorites"} {
			path := filepath.Join(p.s.Root, dir)
			if !exists(path) {
				if err := os.Mkdir(path, 0o777); err != nil {
					return err
				}
			}
		}
		p.displayMessage("Installing node_modules...")
		if err := p.sh(ctx, "cd frontend && npm install"); err != nil {
			return err
		}
		p.displayMessage("Successfully installed node_modules")
		p.displayMessage("Building react-ui...")
		if err := p.sh(ctx, reactUIBuild); err != nil {
			return err
		}
		p.displayMessage("Successfully built react-ui")
		return nil
	}()
	if err != nil && !mustRethrow(ctx, err) {
		p.displayMessage("Failed to install node_modules or build react-ui")
	}
	return err
}

// loadVersions does what loading js/initializeApp.js does with versions.json:
// newPipPackagesVersion = String(versions.pip_packages).
func (p *jsProcess) loadVersions() error {
	raw, err := os.ReadFile(p.installerFile("versions.json"))
	if err != nil {
		return err
	}
	var versions any
	if err := json.Unmarshal(raw, &versions); err != nil {
		return err
	}
	obj, _ := versions.(map[string]any)
	v, present := obj["pip_packages"]
	p.newPipPackagesVersion = jsString(v, present)
	return nil
}

// jsString is JavaScript's String(v) for a value decoded from JSON.
func jsString(v any, present bool) string {
	if !present {
		return "undefined"
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if e != nil {
				parts[i] = jsString(e, true)
			}
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}
