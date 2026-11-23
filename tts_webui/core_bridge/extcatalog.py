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
    }
