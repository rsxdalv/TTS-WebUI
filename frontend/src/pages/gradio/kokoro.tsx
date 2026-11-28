import React from "react";
import { GradioFrame } from "../../components/GradioFrame";

/**
 * A demo of one Gradio tab on its own: the Kokoro tab of Text-to-Speech. Any
 * other tab is the same page with another `tab` path.
 */
const GradioKokoroPage = () => (
  <GradioFrame title="Kokoro TTS (Gradio)" tab="Text-to-Speech/Kokoro" />
);

export default GradioKokoroPage;
