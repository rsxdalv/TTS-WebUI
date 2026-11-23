from .base import POST, SubBridge


class ConfigBridge(SubBridge):
    """Single-setting changes to config.json: ``/api/config/*`` in
    core/internal/serverpy/appconfig. Go changes one thing under its lock and
    writes the file; Python never saves the whole document.

    What Go answers, per operation:
    - extension_enable(package_name), extension_disable(package_name),
      extension_toggle(package_name): {"changed", "disabled", "info"}; ``info``
      is the toast text for gr.Info, ``disabled`` the state after the call
    - extension_autostart(package_name, enabled): {"changed", "enabled"}
    - set_value(namespace, key, value): {"changed"}
    - replace_section(namespace, values): {"changed"}; values is a whole object

    The in-memory ``config`` dict of this process is not updated by these
    calls; it is the config as of startup.
    """

    name = "config"
    ops = {
        "extension_enable": (POST, ("package_name",)),
        "extension_disable": (POST, ("package_name",)),
        "extension_toggle": (POST, ("package_name",)),
        "extension_autostart": (POST, ("package_name", "enabled")),
        "set_value": (POST, ("namespace", "key", "value")),
        "replace_section": (POST, ("namespace", "values")),
    }
