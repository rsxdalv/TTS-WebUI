from .base import GET, SubBridge


class LaunchBridge(SubBridge):
    """Gradio's launch options: ``/api/launch/*`` in core/internal/serverpy/launchopts.

    - options(): {"options": {...}, "lines": [...]}. ``options`` are the keyword
      arguments of ``launch()`` (auth is null or a list of [user, password]
      pairs: JSON has no tuples) and hold secrets; ``lines`` is what to print
      before launching and never does.
    """

    name = "launch"
    ops = {"options": (GET, ())}
