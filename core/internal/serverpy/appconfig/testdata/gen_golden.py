"""Regenerates golden.json with the REAL python json.dump the Go package copies.

    python gen_golden.py <out-golden.json>

Each case is {"name", "input" (JSON text, as sent in a PUT), "output" (exactly
what json.dump(json.loads(input), f, indent=2) writes)}. Also records
default_config from tts_webui/config/load_config.py (run from the repo root so
the import works) as "default".
"""
import json, os, sys

CASES = {
    "empty_object": "{}",
    "nested_empty": '{"a": [], "b": {}, "c": [[], {}]}',
    "key_order": '{"z": 1, "a": 2, "m": {"y": true, "b": null}}',
    "unicode": '{"name": "caf\\u00e9 日本 \U0001F600", "ctl": "a\\u0001b\\u007f\\n\\t\\r\\b\\f", "q": "\\"\\\\/"}',
    "html": '{"s": "<script>&amp;</script>"}',
    "numbers": '{"i": 7770, "neg": -3, "f": 1.5, "e": 1e-07, "big": 12345678901234567890}',
    "auth": '{"gradio_interface_options": {"auth": "user:p\\u00e4ss", "server_port": 7771, "server_name": "0.0.0.0"}}',
    "list_of_objects": '[1]',
}


def main():
    out = sys.argv[1]
    sys.path.insert(0, os.getcwd())
    from tts_webui.config.load_config import default_config

    cases = []
    for name, text in CASES.items():
        if name == "list_of_objects":
            continue
        data = json.loads(text)
        cases.append({"name": name, "input": text, "output": json.dumps(data, indent=2)})
    cases.append({"name": "default", "input": json.dumps(default_config),
                  "output": json.dumps(default_config, indent=2)})
    with open(out, "w", encoding="utf-8", newline="\n") as f:
        json.dump(cases, f, indent=2)
        f.write("\n")


main()
