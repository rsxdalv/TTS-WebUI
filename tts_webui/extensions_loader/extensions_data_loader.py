"""
Extensions data loader module.

This module provides functions to load extensions data from various sources.
It abstracts the data loading process to enable more complex data sources in the future,
such as multiple JSON file merges or fetching from external sources.
"""

import json
import os
from typing import Dict, List, Any, Optional, Iterable
from itertools import chain


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


def merge_extensions_data(
    base_data: Dict[str, Any], additional_data: Dict[str, Any]
) -> Dict[str, Any]:
    """
    Merge two extension data dictionaries.

    Args:
        base_data (Dict[str, Any]): Base extensions data.
        additional_data (Dict[str, Any]): Additional extensions data to merge.

    Returns:
        Dict[str, Any]: Merged extensions data.
    """
    result = base_data.copy()

    for key, value in additional_data.items():
        # Do not merge 'tabs' here; handled by get_interface_extensions
        if key == "tabs":
            continue

        # Only concatenate decorators; for all other keys, prefer base unless missing
        if (
            key == "decorators"
            and key in result
            and isinstance(value, list)
            and isinstance(result[key], list)
        ):
            # Concatenate and de-duplicate by package_name preserving order
            result[key] = _dedupe_by_package_name([*result[key], *value])
        elif key not in result:
            result[key] = value
        else:
            # For all other keys (including dicts like tabsInGroups), do not deep-merge; keep base
            pass

    return result


def load_merged_extensions_data() -> Dict[str, Any]:
    """
    Load and merge extensions data from multiple sources.

    Precedence for decorators and similar list-like data: external > latest > base.
    Tabs are handled separately in get_interface_extensions().

    Returns:
        Dict[str, Any]: Merged extensions data from all sources.
    """
    base = load_extensions_json()
    catalog = load_catalog_extensions_json()
    external = (
        load_json_file(EXTERNAL_EXTENSIONS_FILE)
        if os.path.exists(EXTERNAL_EXTENSIONS_FILE)
        else {}
    )

    # Start from base metadata
    result: Dict[str, Any] = base.copy() if isinstance(base, dict) else {}

    # Merge decorators with precedence: external > latest > base (keep-first by package_name)
    base_decor = base.get("decorators", []) if isinstance(base, dict) else []
    catalog_decor = catalog.get("decorators", []) if isinstance(catalog, dict) else []
    external_decor = (
        external.get("decorators", []) if isinstance(external, dict) else []
    )
    result["decorators"] = _dedupe_by_package_name(
        [*external_decor, *catalog_decor, *base_decor]
    )

    # For any other non-tab keys missing in base, fill from latest, then external
    for src in (catalog, external):
        if not isinstance(src, dict):
            continue
        for k, v in src.items():
            if k in ("tabs", "tabsInGroups", "decorators"):
                continue
            if k not in result:
                result[k] = v

    return result


def get_decorator_extensions() -> List[Dict[str, Any]]:
    """
    Get the list of decorator extensions.

    Returns:
        List[Dict[str, Any]]: List of decorator extensions.
    """
    extensions_data = load_merged_extensions_data()
    return extensions_data.get("decorators", [])


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


def filter_extensions_by_type_and_class(
    extensions: List[Dict[str, Any]],
    extension_type: str,
    extension_class: Optional[str] = None,
) -> List[Dict[str, Any]]:
    """
    Filter extensions by type and class.

    Args:
        extensions (List[Dict[str, Any]]): List of extensions to filter.
        extension_type (str): Extension type to filter by.
        extension_class (Optional[str], optional): Extension class to filter by.
            If None, only filters by type. Defaults to None.

    Returns:
        List[Dict[str, Any]]: Filtered list of extensions.
    """
    if extension_class is None:
        return [x for x in extensions if x.get("extension_type") == extension_type]

    return [
        x
        for x in extensions
        if x.get("extension_type") == extension_type
        and x.get("extension_class") == extension_class
    ]


def get_decorator_extensions_by_class(class_name: str) -> List[Dict[str, Any]]:
    """
    Get decorator extensions filtered by class.

    Args:
        class_name (str): Class name to filter by (e.g., "outer", "inner").

    Returns:
        List[Dict[str, Any]]: Filtered list of decorator extensions.
    """
    decorators = get_decorator_extensions()
    return filter_extensions_by_type_and_class(decorators, "decorator", class_name)


def get_interface_extensions_by_class(class_name: str) -> List[Dict[str, Any]]:
    """
    Get interface extensions filtered by class.

    Args:
        class_name (str): Class name to filter by.

    Returns:
        List[Dict[str, Any]]: Filtered list of interface extensions.
    """
    interfaces = get_interface_extensions()
    return filter_extensions_by_type_and_class(interfaces, "interface", class_name)


def get_extension_example() -> Dict[str, Any]:
    """
    Get the example extension template.

    Returns:
        Dict[str, Any]: Example extension template.
    """
    extensions_data = load_merged_extensions_data()
    return extensions_data.get("example_extension", {})


def create_empty_external_extensions_file() -> bool:
    """
    Create an empty external extensions file if it doesn't exist.

    Returns:
        bool: True if the file was created, False otherwise.
    """
    if not os.path.exists(EXTERNAL_EXTENSIONS_FILE):
        try:
            with open(EXTERNAL_EXTENSIONS_FILE, "w") as f:
                json.dump(
                    {"tabs": [], "tabsInGroups": {}, "decorators": []}, f, indent=4
                )
            print(f"Created empty {EXTERNAL_EXTENSIONS_FILE}")
            return True
        except Exception as e:
            print(f"Failed to create {EXTERNAL_EXTENSIONS_FILE}: {e}")
            return False
    return False
