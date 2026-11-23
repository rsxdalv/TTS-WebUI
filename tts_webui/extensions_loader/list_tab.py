import gradio as gr

# The list pages are served by Go from the same origin as Gradio: the router
# sends /_core to core/internal/serverpy/extui, which lists the extensions and
# runs installs and uninstalls by package name behind its own cookie, Origin and
# token checks. Python holds no token for it and runs nothing.
LIST_PAGE = "/_core/ext/list"


def extension_list_ui(kind: str):
    """The body of the two list tabs. ``kind`` is "interface" or "decorator"."""
    gr.HTML(
        f'<iframe src="{LIST_PAGE}?kind={kind}" '
        'title="Extensions list" style="width: 100%; height: 85vh; border: 0;"></iframe>'
    )
