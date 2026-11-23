"""
The interface extensions list, read straight from the extension files.

The app no longer uses this: the Go core merges the files and decides what to
load (core/internal/serverpy/extcatalog, bridge.extcatalog.resolve). It stays
for installer_scripts/tools/extension_manager.py, a separate CLI process that
runs without the core. Keep its merge in step with extcatalog/catalog.go.
"""

import json
import os
from itertools import chain
from typing import Any, Dict, Iterable, List

# Default paths for extensions files
DEFAULT_EXTENSIONS_FILE = "extensions.json"
EXTERNAL_EXTENSIONS_FILE = "extensions.external.json"
# Catalog (git-cloned) source
CATALOG_DIR = os.path.join("data", "extensions-catalog")
CATALOG_EXTENSIONS_FILE = os.path.join(CATALOG_DIR, "lib", "extensions.json")


def load_json_file(file_path: str) -> Dict[str, Any]:
    """
    Load a JSON file.

    Args:
        file_path (str): Path to the JSON file.

    Returns:
        Dict[str, Any]: The contents of the JSON file as a dictionary.
        Returns an empty dict if the file cannot be loaded.
    """
    try:
        with open(file_path, "r") as f:
            return json.load(f)
    except Exception as e:
        print(f"\n! Failed to load {file_path}: {e}")
        return {}


def load_extensions_json() -> Dict[str, Any]:
    """
    Load the extensions.json file.

    Returns:
        Dict[str, Any]: The contents of extensions.json as a dictionary.
        Returns an empty dict if the file cannot be loaded.
    """
    return load_json_file(DEFAULT_EXTENSIONS_FILE)


def load_catalog_extensions_json() -> Dict[str, Any]:
    """
    Load the extensions catalog from the git-synced repository, if present.

    Returns an empty dict if not available or failed to load.
    """
    if os.path.exists(CATALOG_EXTENSIONS_FILE):
        data = load_json_file(CATALOG_EXTENSIONS_FILE)
        if data:
            return data
    return {}


def _dedupe_by_package_name(items: Iterable[Any]) -> List[Dict[str, Any]]:
    """Deduplicate a sequence of tab dicts by package_name preserving order."""
    out: List[Dict[str, Any]] = []
    seen: set[str] = set()
    for item in items:
        if isinstance(item, dict):
            pkg = item.get("package_name")
            if pkg and pkg in seen:
                continue
            if pkg:
                seen.add(pkg)
        out.append(item)  # tolerate non-dict if present
    return out


def _flatten_interface_tabs(extensions_data: Dict[str, Any]) -> List[Dict[str, Any]]:
    """Return a single list of interface tabs from either 'tabs' or 'tabsInGroups'.

    - Accepts legacy format: { "tabs": [ ... ] }
    - Accepts grouped format: { "tabsInGroups": { "groupA": [ ... ], "groupB": [ ... ] } }
    - Concatenates tabs + ...Object.values(tabsInGroups) and de-duplicates by package_name.
    """
    tabs_list = extensions_data.get("tabs", []) or []
    groups = extensions_data.get("tabsInGroups", {}) or {}
    group_lists = (
        [v for v in groups.values() if isinstance(v, list)]
        if isinstance(groups, dict)
        else []
    )
    flattened = chain.from_iterable([tabs_list, *group_lists])
    return _dedupe_by_package_name(flattened)


def get_interface_extensions() -> List[Dict[str, Any]]:
    """
    Get the list of interface extensions (tabs) supporting both 'tabs' and 'tabsInGroups'.

    Precedence: external > latest > base. Implemented by concatenation order with
    keep-first de-duplication by package_name.

    Returns:
        List[Dict[str, Any]]: Ordered, de-duplicated list of interface extensions.
    """
    base_data = load_extensions_json()
    catalog_data = load_catalog_extensions_json()
    external_data = (
        load_json_file(EXTERNAL_EXTENSIONS_FILE)
        if os.path.exists(EXTERNAL_EXTENSIONS_FILE)
        else {}
    )

    base_tabs = _flatten_interface_tabs(base_data)
    catalog_tabs = _flatten_interface_tabs(catalog_data)
    external_tabs = _flatten_interface_tabs(external_data)

    combined = _dedupe_by_package_name([*external_tabs, *catalog_tabs, *base_tabs])
    return combined
