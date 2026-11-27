export const modelsFnFactory = (
  setLoading: React.Dispatch<React.SetStateAction<boolean>>,
  setOptions: React.Dispatch<React.SetStateAction<string[]>>,
  prefix: string
) => ({
  fetchOptions: async () => {
    setLoading(true);
    const response = await fetch(`/api/gradio/${prefix}_get_models`, {
      method: "POST",
    });
    const result = await response.json();
    setOptions(result);
    return setLoading(false);
  },

  openModels: () =>
    fetch(`/api/gradio/${prefix}_open_model_dir`, {
      method: "POST",
    }),

  unloadModel: () =>
    fetch(`/api/gradio/${prefix}_unload_model`, {
      method: "POST",
    }),
});
