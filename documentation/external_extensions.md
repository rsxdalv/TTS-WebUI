# External Extensions

This document explains how to use the external extensions feature to add custom extensions without modifying the main `extensions.json` file.

## Overview

The TTS WebUI supports loading extensions from multiple sources:

1. The main `extensions.json` file (included in the repository)
2. An optional `extensions.external.json` file (gitignored, for your custom extensions)

This allows you to add your own extensions without modifying the main extensions file, making it easier to update the application without conflicts.

## How to Use

### Creating the External Extensions File

You can create an `extensions.external.json` file in the root directory of the application. This file should have the same structure as the main `extensions.json` file.

### Structure of the External Extensions File

The external extensions file should have the following structure:
You can define interface extensions in two equivalent ways:

1) Flat list (legacy):

```json
```
{
    "tabs": [
        {
            "package_name": "extension_custom_example",
            "name": "Custom Example Extension",
            "version": "1.0.0",
            "requirements": "git+https://github.com/example/extension_custom_example@main",
            "description": "This is an example of a custom extension",
            "extension_type": "interface",
            "extension_class": "tools",
            "author": "Your Name",
            "extension_author": "Your Name",
            "license": "MIT",
            "website": "https://github.com/example/extension_custom_example",
            "extension_website": "https://github.com/example/extension_custom_example",
            "extension_platform_version": "0.0.1"
        }
    ],
            }
        ],
    ```

    2) Grouped lists (new):

    Use `tabsInGroups` to organize tabs into named groups. The app will automatically flatten all groups into a single list for rendering. If both `tabs` and `tabsInGroups` are present, they will be combined with duplicates removed by `package_name`.

    ```json
    {
        "tabsInGroups": {
            "text_to_speech": [
                {
                    "package_name": "extension_custom_example",
                    "name": "Custom Example Extension",
                    "version": "1.0.0",
                    "requirements": "git+https://github.com/example/extension_custom_example@main",
                    "description": "This is an example of a custom extension",
                    "extension_type": "interface",
                    "extension_class": "text-to-speech",
                    "author": "Your Name",
                    "extension_author": "Your Name",
                    "license": "MIT",
                    "website": "https://github.com/example/extension_custom_example",
                    "extension_website": "https://github.com/example/extension_custom_example",
                    "extension_platform_version": "0.0.1"
                }
            ],
            "audio_tools": []
        }
    }
    ```
    "decorators": [
        {
            "package_name": "extension_custom_decorator_example",
            "name": "Custom Decorator Example",
            "version": "1.0.0",
            "requirements": "git+https://github.com/example/extension_custom_decorator_example@main",
            "description": "This is an example of a custom decorator extension",
            "extension_type": "decorator",
            "extension_class": "outer",
            "author": "Your Name",
            "extension_author": "Your Name",
            "license": "MIT",
            "website": "https://github.com/example/extension_custom_decorator_example",
            "extension_website": "https://github.com/example/extension_custom_decorator_example",
            "extension_platform_version": "0.0.1"
        }
    ]
}
```

### Merging Behavior

When the application loads extensions, it will:

1. Load the main `extensions.json` file
2. If `extensions.external.json` exists, load and merge it with the main file
3. For lists (like `tabs` and `decorators`), extensions from the external file will be added to the main list
3. For lists (like `tabs` and `decorators`), extensions from the external file will be added to the main list. For dict-of-lists like `tabsInGroups`, groups are merged per key and items are de-duplicated by `package_name`.
4. If an extension with the same `package_name` exists in both files, the one from the main file will be used (duplicates are avoided)
