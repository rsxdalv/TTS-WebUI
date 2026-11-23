"""Turn object markers from the Go core into Gradio components.

Go cannot build a ``gr.Button``; it sends a marker and the bridge builds the
object:

    {"__gradio__": "Button", "props": {"value": "Saved"}}  ->  gr.Button(value="Saved")

Markers may be nested in lists and dicts. Only the components in ``COMPONENTS``
and the props in ``PROPS`` are accepted, so a response cannot make Python
construct arbitrary objects or pass arbitrary keyword arguments.
"""

MARKER = "__gradio__"

# name -> attribute of the gradio module. Hand-written on purpose.
COMPONENTS = (
    "Button",
    "Dropdown",
    "Textbox",
    "Markdown",
    "Audio",
    "Image",
    "JSON",
    "Gallery",
)

PROPS = frozenset(
    {"value", "label", "visible", "interactive", "choices", "variant", "placeholder"}
)


class GradioMarkerError(ValueError):
    """A marker names an unknown component or prop."""


def marker(component, **props):
    """Build a marker (what the Go side sends)."""
    return {MARKER: component, "props": props}


def is_marker(obj):
    return isinstance(obj, dict) and MARKER in obj


def resolve(obj):
    """Recursively replace markers in obj with Gradio components."""
    if is_marker(obj):
        import gradio as gr

        name = obj[MARKER]
        props = obj.get("props", {})
        extra = set(obj) - {MARKER, "props"}
        if name not in COMPONENTS or extra or not isinstance(props, dict):
            raise GradioMarkerError(f"bad gradio marker: {obj!r}")
        bad = set(props) - PROPS
        if bad:
            raise GradioMarkerError(f"{name}: unknown props {sorted(bad)}")
        return getattr(gr, name)(**props)
    if isinstance(obj, list):
        return [resolve(x) for x in obj]
    if isinstance(obj, dict):
        return {k: resolve(v) for k, v in obj.items()}
    return obj
