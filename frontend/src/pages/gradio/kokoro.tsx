import React from "react";
import EnginePage from "../engines/engine";

/**
 * The Kokoro extension on its own: Go starts it (it is installed in its own
 * venv) and its page is framed directly, with no Gradio tabs around it. Any
 * other extension is the same page with another package name.
 */
const GradioKokoroPage = () => (
  <EnginePage pkg="tts_webui_extension.kokoro" name="Kokoro" />
);

export default GradioKokoroPage;
