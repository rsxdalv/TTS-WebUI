export const saveToVoices = (api_filename?: string) => {
  fetch("/api/gradio/save_to_voices", {
    method: "POST",
    body: JSON.stringify({
      npz_filename: api_filename?.replace(".ogg", ".npz"),
    }),
  });
};
