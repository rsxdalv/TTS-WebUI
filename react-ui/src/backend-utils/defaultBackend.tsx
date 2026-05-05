export const defaultBackend =
  process.env.GRADIO_BACKEND ||
  process.env.GRADIO_BACKEND_AUTOMATIC ||
  "http://127.0.0.1:7770/";

console.log(
  `Using Gradio backend: ${defaultBackend} (set GRADIO_BACKEND or GRADIO_BACKEND_AUTOMATIC environment variable to change)`
)
