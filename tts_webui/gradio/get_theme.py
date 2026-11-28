import gradio as gr


def get_theme(config):
    default_theme_settings = {
        "theme": "Base",
        "hues": {
            # "primary_hue": "blue",
            "primary_hue": "sky",
            "secondary_hue": "sky",
            "neutral_hue": "neutral",
        },
        "options": {
            "embed_radius": "*radius_sm",
            "block_label_radius": "*radius_sm",
            "block_label_right_radius": "*radius_sm",
            "block_radius": "*radius_sm",
            "block_title_radius": "*radius_sm",
            "container_radius": "*radius_sm",
            "checkbox_border_radius": "*radius_sm",
            "input_radius": "*radius_sm",
            "table_radius": "*radius_sm",
            "button_large_radius": "*radius_sm",
            "button_small_radius": "*radius_sm",
            # Light theme = the React UI's light theme (frontend/src/app/globals.css,
            # :root), so Gradio looks the same embedded or on its own. Keep the two
            # in sync. Dark is Gradio's own dark theme, which the React UI follows,
            # so only light values are set; the _dark ones below are pinned where
            # Gradio's _dark default is empty and would inherit the light value.
            "body_background_fill": "#f0eff0",  # page gray (top of the React UI's page)
            "body_text_color": "#030712",  # --foreground
            "body_text_color_subdued": "#585e6a",  # --muted-foreground
            "background_fill_secondary": "#f3f4f6",  # --muted
            "border_color_primary": "#e5e7eb",  # --border
            "input_background_fill": "white",
            "input_border_color": "#ced0d4",  # --input
            "input_placeholder_color": "#6b7280",
            "block_label_text_color": "#585e6a",
            "block_title_text_color": "#585e6a",
            # --primary is near-black in the React UI, not the sky hue.
            "button_primary_background_fill": "#111827",
            "button_primary_background_fill_hover": "#1f2937",
            "button_primary_border_color": "#111827",
            "button_primary_text_color": "#f9fafb",
            "button_primary_background_fill_hover_dark": "*primary_600",
            "button_secondary_background_fill": "#dfe1e7",  # --secondary
            "button_secondary_background_fill_hover": "#cfd2da",
            "button_secondary_border_color": "#dfe1e7",
            "button_secondary_text_color": "#111827",
            "button_secondary_background_fill_hover_dark": "*secondary_600",
            "slider_color": "#111827",
            "slider_color_dark": "*primary_500",
            "checkbox_background_color_selected": "#111827",
            "checkbox_border_color_selected": "#111827",
        },
    }
    theme_settings = config.get("theme_settings", default_theme_settings)
    themes = {
        "Base": gr.themes.Base,
        "Default": gr.themes.Default,
        "Monochrome": gr.themes.Monochrome,
    }
    theme: gr.themes.Base = themes[theme_settings["theme"]](
        **theme_settings["hues"],
        font=[
            gr.themes.GoogleFont("Inter"),
            "ui-sans-serif",
            "system-ui",
            "sans-serif",
        ],
    )
    theme.set(**theme_settings["options"])
    return theme
