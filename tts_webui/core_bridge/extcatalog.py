from .base import GET, POST, STREAM, SubBridge


class ExtCatalogBridge(SubBridge):
    """Extension lists and installs: ``/api/extcatalog/*`` in
    core/internal/serverpy/extcatalog.

    What Go answers, per operation (``kind`` is "interface" or "decorator"):
    - list(kind): the merged extension entries (external > catalog > base)
    - table(kind): the Markdown table of the list tabs, as a gr.Markdown
    - picker(kind, label): a gr.Dropdown of the installable package names
    - parse(text): {"entries": [...], "preview": gr.Markdown, "info": str}
    - add(entries): {"status": str, "json": str}; Go writes extensions.external.json
    - install(package_name), uninstall(package_name): generators that yield the
      console HTML (output lines joined with <br />) as the command runs
    """

    name = "extcatalog"
    ops = {
        "list": (GET, ("kind",)),
        "table": (GET, ("kind",)),
        "picker": (GET, ("kind", "label")),
        "parse": (POST, ("text",)),
        "add": (POST, ("entries",)),
        "install": (STREAM, ("package_name",)),
        "uninstall": (STREAM, ("package_name",)),
    }
