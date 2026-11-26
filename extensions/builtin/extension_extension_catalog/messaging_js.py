
messaging_js = """
() => {{
    const iframe = document.getElementById('extension-catalog');
    const CATALOG_ORIGIN = 'https://rsxdalv.github.io';

    window.addEventListener('message', (event) => {
      // The iframe loads remote content, so the sender's origin must be
      // checked too: event.source alone still matches after the frame has
      // navigated somewhere else.
      if (event.origin !== CATALOG_ORIGIN) return;
      if (event.source !== iframe.contentWindow) return;
      if (!event.data || event.data.type !== 'install-extension') return;
      {
        const extension = event.data.data;
        
        document.getElementById('json-container').innerText =
          JSON.stringify(extension, null, 2);
        document.getElementById('receive_extension_button').click();
      }
    });

    console.log("Extension catalog iframe messaging initialized.");
}}
"""
