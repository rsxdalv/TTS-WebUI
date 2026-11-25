# Moving an install to the new layout

An install that has `installer_files/env` (the micromamba environment the old installer made) **keeps running on it.**
Updating to a version with the new launcher does not change it, delete it or ask you to move. The launcher's pages
show a notice that points here. This page says what moving means, and what it costs. Nothing here is automatic.

## What changes in the new layout

| Old install | New install |
|---|---|
| One shared Python env (`installer_files/env`) with Python 3.10.11, git, Node, ffmpeg and every extension | `uv` installed on demand into `platforms/uv/`; Python 3.10.11 downloaded by uv; one venv per engine in `.venvs/<entry>/`; git and ffmpeg fetched into `platforms/uv/tools/` |
| The Gradio UI and its 46 in-process tabs always installed | The Gradio host is an optional entry (`gradio-host`), installed from the launcher's Extensions page |
| Setup tab: download, pre-flight, mamba, GPU, install, update | Setup tab: download, pre-flight, update. The GPU is chosen when uv is installed, from the Extensions page |
| Update is `git pull` and a reinstall | Update downloads the installer archive and replaces the launcher, the UI and the sources (`archive.json` says which version a folder is) |

## What it costs

- **Disk.** The old env stays until you delete it. The new layout downloads Python and torch again (several GB for a
  GPU torch row); engines that use the same torch row share its files by hardlink, but the old env shares nothing with
  them.
- **Reinstalling engines.** Each engine is installed again from the Extensions page. Models in `data/` and the
  Hugging Face cache are not in the env and are reused.
- **Tabs that ran inside the Gradio host** need the host installed first (the page does it for you). A tab's own
  requirements may still conflict with another tab's there; that ends only when each tab runs in its own venv.

## How to move

1. Download the installer archive into a **new folder** (the old one stays as it is, so you can go back).
2. Start it. The Setup tab has download, pre-flight and update; there is no micromamba step.
3. Open the Extensions page and install the engines you use. The first install asks for the GPU (it proposes the one
   it found), then installs uv, then the engine.
4. Copy what is yours from the old folder into the new one while both are stopped: `outputs/`, `favorites/`, `voices/`,
   `config.json`, `.env`, and `data/` if you keep models there.
5. When the new install does what you need, delete the old folder yourself. Nothing deletes it for you.

## Not moving

Nothing breaks. `legacy-env` is kept until the maintainers say it can go (`PLAN.md`, P9), and that is announced
in the release notes first.
