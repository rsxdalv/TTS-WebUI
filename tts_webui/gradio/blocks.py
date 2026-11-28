import html
import os

import gradio as gr

from tts_webui.gradio.all_tabs import all_tabs
from tts_webui.gradio.css import full_css
from tts_webui.gradio.fix_gradio_tabs import fix_gradio_tabs
from tts_webui.gradio.get_theme import get_theme

# The React UI shows the Gradio page in a frame: ?embed=1 hides the page header
# and tightens the layout (.tts-embed in tts_webui.css),
# ?tab=Text-to-Speech/Kokoro opens that tab (names match without emoji and
# case, one per level, outermost first) and hides the tab rows of those levels.
embed_head = """
<script>
(function () {
  var p = new URLSearchParams(location.search);
  // Embedded in the React UI: tts_webui.css (.tts-embed) uses the full width.
  if (p.get("embed") === "1") document.documentElement.classList.add("tts-embed");
  var tab = p.get("tab");
  if (!tab) return;
  function norm(s) { return s.toLowerCase().replace(/[^a-z0-9]+/g, " ").trim(); }
  var path = tab.split("/").map(norm), i = 0, tries = 0;
  var timer = setInterval(function () {
    if (i >= path.length || ++tries > 150) { clearInterval(timer); return; }
    var btn = [].slice.call(document.querySelectorAll("button[role=tab]"))
      .find(function (b) { return norm(b.textContent) === path[i]; });
    if (!btn) return;
    if (btn.getAttribute("aria-selected") !== "true") btn.click();
    // The React UI menu links to these levels, so its frame hides their tab row.
    var nav = btn.closest(".tab-nav");
    if (nav) nav.setAttribute("data-tts-hide", "");
    i++;
  }, 200);
})();
</script>
"""


def main_block(config):
    fix_gradio_tabs()

    with gr.Blocks(
        css=full_css,
        title="TTS WebUI - Gradio",
        head=embed_head,
        analytics_enabled=False,  # it broke too many times
        theme=get_theme(config),
    ) as blocks:
        # Where the React UI is. The launcher sets TTS_REACT_UI_URL ("/" when the
        # router serves it on the same port as these pages).
        react_ui_url = html.escape(
            os.environ.get("TTS_REACT_UI_URL", "http://localhost:3000"), quote=True
        )
        gr.HTML(
            f"""
        <div style="display: flex; align-items: baseline; gap: 1rem; font-family: sans-serif;">
            <h1>TTS WebUI</h1>
            | <h4>(The Gradio UI has more tools, but the UI is basic)</h4>
            | <h3><a href="{react_ui_url}">React UI</a></h3>
            | <h3><a href="https://forms.gle/2L62owhBsGFzdFBC8">Feedback / Bug reports</a></h3>
            | <h3><a href="https://discord.gg/V8BKTVRtJ9">Discord Server</a></h3>
        </div>
        """,
            elem_id="tts-header",
        )

        with gr.Tabs():
            all_tabs(config)

    print("\nGradio UI loaded")

    return blocks


if __name__ == "__main__":
    from tts_webui.config.config import config

    main_block(config).launch(
        server_port=7770,
    )
