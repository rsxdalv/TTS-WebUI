import subprocess
import sys
from pathlib import Path
from typing import List, Optional

import typer

app = typer.Typer(help="tts-webui command line")
ext_app = typer.Typer(help="Manage extensions")
app.add_typer(ext_app, name="extension")


def _run_process(cmd: List[str]) -> int:
    try:
        return subprocess.call(cmd)
    except FileNotFoundError:
        typer.secho(f"Executable not found: {cmd[0]}", fg=typer.colors.RED)
        return 2


@app.command()
def serve(extra_args: Optional[List[str]] = typer.Argument(None)) -> int:  # pragma: no cover - manual run
    """Start the web UI server by delegating to `server.py` in the project root."""
    project_root = Path.cwd()
    server = project_root / "server.py"
    if not server.exists():
        typer.secho("Error: server.py not found in project root.", fg=typer.colors.RED)
        raise typer.Exit(code=2)

    cmd = [sys.executable, str(server)] + (extra_args or [])
    typer.secho("Starting server with: " + " ".join(cmd))
    raise typer.Exit(code=_run_process(cmd))


@app.command()
def troubleshoot() -> None:
    """Run basic troubleshooting checks and print recommendations."""
    project_root = Path.cwd()
    typer.secho("Running troubleshooting checks...")

    typer.echo(f"Python executable: {sys.executable}")
    typer.echo(f"Python version: {sys.version.splitlines()[0]}")

    errors = 0
    req = project_root / "requirements.txt"
    if req.exists():
        try:
            lines = [l.strip() for l in req.read_text(encoding="utf-8").splitlines() if l.strip() and not l.startswith("#")]
            typer.echo(f"Found requirements.txt with {len(lines)} entries.")
        except Exception:
            typer.echo("Unable to read requirements.txt")
            errors += 1
    else:
        typer.echo("Warning: requirements.txt not found in project root.")
        errors += 1

    server = project_root / "server.py"
    if server.exists():
        typer.echo("Found server.py")
    else:
        typer.echo("Warning: server.py not found. 'serve' may not work.")
        errors += 1

    try:
        import importlib

        torch_spec = importlib.util.find_spec("torch")
        if torch_spec is None:
            typer.echo("PyTorch not installed. If you expect GPU acceleration, install torch.")
            errors += 1
        else:
            import torch  # type: ignore

            typer.echo(f"PyTorch version: {torch.__version__}")
            try:
                typer.echo(f"CUDA available: {torch.cuda.is_available()}")
            except Exception:
                typer.echo("Unable to query CUDA availability on this platform.")
    except Exception as e:
        typer.echo("Troubleshoot check error:", e)

    if errors:
        typer.echo(f"Troubleshooting finished with {errors} warnings/errors.")
        raise typer.Exit(code=1)

    typer.echo("Basic troubleshooting checks passed.")


@ext_app.command("list")
def extension_list() -> None:
    project_root = Path.cwd()
    ext_dir = project_root / "extensions"
    ext_dir.mkdir(parents=True, exist_ok=True)
    items = [p.name for p in ext_dir.iterdir() if p.is_dir()]
    if not items:
        typer.echo("No extensions found in" + " " + str(ext_dir))
        return
    typer.echo("Installed extensions:")
    for n in sorted(items):
        typer.echo("- " + n)


@ext_app.command("install")
def extension_install(path: str) -> None:
    project_root = Path.cwd()
    from shutil import copy2, copytree

    src = Path(path)
    dest = project_root / "extensions" / src.name
    if dest.exists():
        typer.echo("Extension already installed:" + " " + src.name)
        raise typer.Exit(code=1)
    try:
        if src.is_dir():
            copytree(src, dest)
        else:
            dest.parent.mkdir(parents=True, exist_ok=True)
            copy2(src, dest)
        typer.echo("Installed extension to" + " " + str(dest))
    except Exception as e:
        typer.echo("Failed to install extension:" + " " + str(e))
        raise typer.Exit(code=1)


@ext_app.command("remove")
def extension_remove(name: str) -> None:
    project_root = Path.cwd()
    from shutil import rmtree

    target = project_root / "extensions" / name
    if not target.exists():
        typer.echo("Extension not found:" + " " + name)
        raise typer.Exit(code=1)
    try:
        if target.is_dir():
            rmtree(target)
        else:
            target.unlink()
        typer.echo("Removed extension:" + " " + name)
    except Exception as e:
        typer.echo("Failed to remove extension:" + " " + str(e))
        raise typer.Exit(code=1)


@ext_app.command("enable")
def extension_enable(name: str) -> None:
    project_root = Path.cwd()
    target = project_root / "extensions" / name
    if not target.exists():
        typer.echo("Extension not found:" + " " + name)
        raise typer.Exit(code=1)
    marker = target / ".disabled"
    try:
        if marker.exists():
            marker.unlink()
        typer.echo("Enabled extension:" + " " + name)
    except Exception as e:
        typer.echo("Failed to change extension state:" + " " + str(e))
        raise typer.Exit(code=1)


@ext_app.command("disable")
def extension_disable(name: str) -> None:
    project_root = Path.cwd()
    target = project_root / "extensions" / name
    if not target.exists():
        typer.echo("Extension not found:" + " " + name)
        raise typer.Exit(code=1)
    marker = target / ".disabled"
    try:
        marker.write_text("", encoding="utf-8")
        typer.echo("Disabled extension:" + " " + name)
    except Exception as e:
        typer.echo("Failed to change extension state:" + " " + str(e))
        raise typer.Exit(code=1)


def main() -> None:  # pragma: no cover - manual run
    app()


if __name__ == "__main__":
    raise SystemExit(main())
