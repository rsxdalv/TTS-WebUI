"""Writes the golden file for the Go JSON scanner: for every input, what
CPython's json.loads answers (the error message, or the value re-dumped).

Run:  python core/internal/serverpy/extcatalog/testdata/make_pyjson_golden.py
"""
import json
import os

CASES = [
    # valid
    "{}", "[]", '{"a":1}', "[1,2,3]", "null", "true", "false", "0", "-1", "1.5",
    "1e3", "1E+3", "-1.5e-3", '"x"', '""', '"\\u00e9"', '"\\ud83d\\ude00"',
    '{"a":{"b":[1,{"c":2}]}}', "  {\"a\": 1}  ", "\n\t[1]\n", '{"a":1,"a":2}',
    '{"":1}', "[]", "[[[[]]]]",
    # invalid
    "", "   ", "nope", "tru", "fals", "nul", "{oops", '{"a":1,}', "[1,", "[1,]",
    '{"a" 1}', "{'a': 1}", '"unterminated', '"bad\\qescape"', '{"a":1} x', "1.",
    "01", "-", "+1", "[1 2]", '{"a":1 "b":2}', "[", "{", '"tab\there"',
    '{"a":"x', "nan", "{,}", '["a",]', '{"a":1,"b":}', "[1,2", "[1,2}", '{"a":1',
    '{"a"', '{"a":', '"\\u12"', '"\\uZZZZ"', '{"a":1,,"b":2}', "1e", "1e+",
    "1.2.3", "0x10", "00", "truex", "{}", "]", "}", ",", ":",
    '{"a":1,}}', '["a" "b"]', '{"a" : 1 , "b" : 2}', "/*c*/1", "#c\n1",
    '{"a":tru}', "[,]", "{,}", "1 2", '"\\x41"', '"\\u00"',
    # floats: Python makes a float out of them and writes them with repr()
    "1e3", "1E+3", "-1.5e-3", "0.1", "1e100", "1e-7", "3.0", "1e16", "1e17",
    "0.30000000000000004", "1e-5", "2.5e-10", "1.50",
    # a big int keeps its text
    "123456789012345678901234567890",
    # surrogate pairs are one character
    '"\\ud83d\\ude00"', '"\\ud800"', '"\\udc00"',
]

out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "pyjson_golden.jsonl")
lines = []
for c in CASES:
    try:
        v = json.loads(c)
    except json.JSONDecodeError as e:
        lines.append({"input": c, "error": str(e)})
        continue
    except Exception as e:  # pragma: no cover
        lines.append({"input": c, "error": f"{type(e).__name__}: {e}"})
        continue
    lines.append({"input": c, "value": v, "dump": json.dumps(v, ensure_ascii=False)})

with open(out, "w", encoding="utf-8", newline="\n") as f:
    for rec in lines:
        # ensure_ascii=True: a lone surrogate (from "\ud800") has no UTF-8
        # form, and the escapes keep the file readable.
        f.write(json.dumps(rec, ensure_ascii=True, sort_keys=True) + "\n")
print(f"{len(lines)} cases -> {out}")
