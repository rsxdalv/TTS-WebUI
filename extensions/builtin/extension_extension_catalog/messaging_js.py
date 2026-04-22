
messaging_js = """
() => {{
    const iframe = document.getElementById('extension-catalog');

    window.addEventListener('message', (event) => {
      if (event.source !== iframe.contentWindow) return;
      if (event.data.type === 'install-extension') {
        const extension = event.data.data;
        
        document.getElementById('json-container').innerText =
          JSON.stringify(extension, null, 2);
        document.getElementById('receive_extension_button').click();
      }
    });

    console.log("Extension catalog iframe messaging initialized.");
}}
"""
