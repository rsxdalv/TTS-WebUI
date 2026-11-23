from .base import GET, POST, STREAM, SubBridge


class ExtCatalogBridge(SubBridge):
    """Extension lists and installs: ``/api/extcatalog/*`` in
    core/internal/serverpy/extcatalog.

    What Go answers, per operation (``kind`` is "interface" or "decorator"):
    - list(kind): the merged extension entries (external > catalog > base)
    - resolve(kind, extension_class): {"extensions": [...], "disabled_names": [...]},
      what to load for that kind and class, in order. Each extension has
      package_name, name, requirements, proxy ("native" or ""), enabled and
      autostart (interface only); disabled ones stay in the list. For
      decorators, disabled_names is the whole disabled_decorators list: package
      names and decorator function names.
    - table(kind): the Markdown table of the list tabs, as a gr.Markdown
    - picker(kind, label): a gr.Dropdown of the installable package names
    - parse(text): {"entries": [...], "preview": gr.Markdown, "info": str}
    - add(entries): {"status": str, "json": str}; Go writes extensions.external.json
    - install(package_name), uninstall(package_name): generators that yield the
      console HTML (output lines joined with <br />) as the command runs
    - install_entry(entry): the same, but Go installs the entry it is given
      (its own requirements and proxy), not a package it looks up by name
    - check_dependencies(package_name): the same stream for a pip dry run of the
      package's requirements (not for native extensions)
    - latest_version(package_name): the version a pip dry run would install, or
      "Already up to date (sometimes incorrect)"

    install covers both kinds of extension: Go reads proxy from the lists and
    runs pip in the main env or sets up .venvs/<package> with uv and the torch
    build for the GPU in installer_scripts/.gpu.
    """

    name = "extcatalog"
    ops = {
        "list": (GET, ("kind",)),
        "resolve": (GET, ("kind", "extension_class")),
        "table": (GET, ("kind",)),
        "picker": (GET, ("kind", "label")),
        "parse": (POST, ("text",)),
        "add": (POST, ("entries",)),
        "install": (STREAM, ("package_name",)),
        "install_entry": (STREAM, ("entry",)),
        "uninstall": (STREAM, ("package_name",)),
        "check_dependencies": (STREAM, ("package_name",)),
        "latest_version": (POST, ("package_name",)),
    }
