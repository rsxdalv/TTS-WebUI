export const favorite = async (
  _url: string,
  data?: {
    folder_root?: string;
  }
) => {
  const folder_root = data?.folder_root;
  if (!folder_root) return;

  const response = await fetch("/api/gradio/bark_favorite", {
    method: "POST",
    body: JSON.stringify({
      folder_root,
    }),
  });

  return await response.json();
};
