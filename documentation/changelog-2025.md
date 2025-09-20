# Changelog 2025 (Until June)

## June 2025

June 26:
* Fix React UI file size limit of 4MB, now 50MB. Thanks https://github.com/SuperFurias ! (#446)

June 20:
* Upgrade Chatterbox to enable compilation for 2-4x speedup.
* Fix React UI build errors.
* Add 'auto start' option to OpenAI-API.

June 10:
* Patch eslint warnings during build.
* Fix extension_cuda_toolkit definition.

June 9:
* Add CUDA Toolkit extension.
* Hotfix for PyTorch 2.7.0 nightly.
* Update Docker to 2.7.0

June 8:
* Fix decorators for generation.
* Refactor server.py code.
* Hotfix for docker, thanks https://github.com/chrislawso for reporting.

June 7:
* Chatterbox upgrade for streaming.

June 6:
* Update DIA Extension for Float16 support.
* Improve decorators for streaming usage.

June 4:
* Attempt dockerfile fix.
* Add interactivity to model unloading button, improve Gradio random seed UI.
* Add sample voices.

June 1:
* Add presets API.
* Add API Preset config to React UI.

## May 2025

May 31:
* Improve React UI Audio player.
* Fix ROCm installation version.

May 30:
* Make OpenAI API extension installed by default (extension_kokoro_tts_api).
* Add Favicon.
* Fix OpenVoice v2 extension.
* Improve UI layout for StyleTTS2, MahaTTS, Vall-E-X, Parler TTS

May 29:
* Add [Chatterbox](https://github.com/resemble-ai/chatterbox) extension.
* Add Kokoro TTS to React UI.
* Fix React Build, thanks [noaht8um](https://github.com/noaht8um)!

May 28:
* Restore gr.Tabs to the old style for easier stacking of many tabs.
* Integrate custom IconButton.
* Fix Gradio's output tab display
* Add tutorial section

May 27:
* Include gradio==5.5.0 in each installation of extensions. While this might cause some extensions to fail to install,
  it should prevent extensions from breaking the UI. Please report extensions that fail to install.
  Thanks to cwlowden for debugging this issue. 
* Make XTTS-RVC-UI an unrecommended extension.

May 26:
* Add fixes for decorators to work with non-'text' inputs.
* Clean up .env generator and remove the Bark environment variables from settings.
* Add Audio book extension definitions for future use (extensions not available yet).
* Fix SeamlessM4T Audio to Audio tab.
* Update ACE-Step extension.
* Improve Kokoro TTS API.

May 14:
* Prepare for Python 3.11 and 3.12 support.

May 12:
* Fix deepspeed for Windows. Thank you for the reports!
* Improve decorator extensions for future API.
* Improve Kokoro TTS API for OpenAI compatibility, now usable with SillyTavern.
* Add setup.py for future pip installs. Sync versions.json with setup.py and package.json.
* Remove deprecated requirements_* files.
* Removed Windows deepspeed until it no longer requires NVCC, thank you https://github.com/lcmiracle for extensive debugging and testing.

May 10:
* Fix missing directory bug causing extensions to fail to load. Thanks Discord/Comstock for discovery of the bug.
* Add ACE-Step to React UI.
* Add emoji to Gradio UI categories for simplicity.
* Add enhanced logging for every update and app startup, allowing for easier debugging once issues happen.
* Show gr.Info when models are being loaded or unloaded.
* Allow users to use React UI together with Gradio auth by specifying GRADIO_AUTH="username:pass" environment variable.

May 7:
* Add [Piper TTS](https://github.com/rhasspy/piper) extension
* Add [ACE-Step](https://github.com/ACE-Step/ACE-Step) extension

May 6:
* Add Kimi Audio 7B Instruct extension
* Fix React-Gradio file proxy missing slash
* Add Kokoro TTS API extension

## April 2025

Apr 25:
* Add OpenVoice V2 extension

Apr 24:
* Add OpenVoice V1 extension

Apr 23:
* Deprecate requirements_* files using direct extension installation instead.
* Add proxy for gradio files in React UI.
* Added [DIA extension](https://github.com/nari-labs/dia).

Apr 22:
* Allow newer versions of pip
* Remove PyTorch's +cpu for Apple M Series Chip
* Installer fixes - fix CUDA repair, CRLF, warn about GCC, terminate if pip fails.

Apr 20:
* Fix install/uninstall in extension manager
* Add Kokoro TTS extension

Apr 18:
* Fix extension manager startup
* Convert most models to extensions, install the classic ones by default
* Attempt to fix linux installer
* Add 'recommended' flag for extensions

Apr 17:
* Create extension manager
* Warn Windows users if conda is installed
* upgrade dockerfile to PyTorch 2.6.0

Apr 12:
* Upgrade to PyTorch 2.6.0 Cuda 12.4, switch to pip for pytorch install
* Add compatibility layer for older models
* Fix StyleTTS2 missing nlkt downloader
* Reorder TTS tabs
* Allow disabled extensions to be configured in config.json
* Remove PyTorch CPU via pip option, redundant
* Move all core conda packages to init_mamba scripts.
* Upgrade the installer to include a web-based UI
* Add conda storage optimizer extension
* Hotfix: New init_app bug that caused the installer to freeze

Apr 11:
* Add AP BWE upscaling extension

Apr 02:
* Fix pydantic (#465, #468)
* Add --no-react --no-database advanced flags
* Add a fix to avoid directory errors on the very first React UI build (#466)

## March 2025

Mar 21:
* Add CosyVoice extension [Unstable] and GPT-SoVITS [Alpha] extension

Mar 20:
* Add executable macOS script for double-click launching
* Add unstable CosyVoice extension

Mar 18:
* Remove old rvc files
* Fix missing torchfcpe dependency for RVC

Mar 17:
* Upgrade Google Colab to PyTorch 2.6.0, add Conda to downgrade Python to 3.10
* No longer abort when the automatic update fails to fetch the new code (Improving offline support #457)
* Upgrade Tortoise to v3.0.1 for transformers 4.49.0 #454
* Prevent running in Windows/System32 folder #459

## February 2025

Feb 15:
* Fix Stable Audio to match the new version

Feb 14:
* Pin accelerate>=0.33.0 project wide
* Add basic Seamless M4T quantization code

Feb 13:
* Fix Stable Audio and Seamless M4T incompatibility
* Make Seamless M4T automatically use CUDA if available, otherwise CPU

Feb 10:
* Improve installation instructions in README
