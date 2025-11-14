import gradio as gr
from gradio_goodtab import GoodTab as Tab
from gradio_goodtabs import GoodTabs


def fix_gradio_tabs():
    gr.Tab = Tab
    gr.Tabs = GoodTabs
