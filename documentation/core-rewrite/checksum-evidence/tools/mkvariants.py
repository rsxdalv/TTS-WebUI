#!/usr/bin/env python3
"""Make scratch copies of the repo (never committed) with one pinned sha256 edited each.
Usage: mkvariants.py WORK   (WORK/src-base must exist)"""
import os, re, shutil, subprocess, sys

work = sys.argv[1]
base = os.path.join(work, "src-base")
P = "core/internal/platform/"
# artifact -> (file, sha256 as pinned)
SHAS = {
    "uv":        (P + "uv/pin.go", "f29c3e94ffabd7e6f3ba521dba169a9b7ec12100bf3257087f0f24f6123f3645"),
    "ffmpeg":    (P + "uv/toolspin.go", "202fe67679a0860b155817a0907ffcc5c1ea82e186acaa21cb87d307f54fd59a"),
    "git":       (P + "uv/toolspin.go", "4a92850b6e917da4fd010669b614b55dccf5b0ec333a534cc4be4700dfefdafd"),
    "audiocpp":  (P + "audiocpp/pin.go", "9f4d9956400e9510b0ca48ce4fe576c887673ad6f6feeadc82ee679f27d794f7"),
    "gguf":      (P + "audiocpp/models.go", "3f71cf6a51f93c2f2e554e98970a3f49fb9a98d0c64bed5a815e65be3d2ff588"),
    "embedding": (P + "audiocpp/models.go", "69c32db63ca56843d994f81f343f62e0bf2d73f7e4c9bc73e44bb1110b1d8845"),
    "onnx-runtime": (P + "onnx/pin.go", "6dffdc715a4465b989446a6105265d2cb345e7101591a17d35534b6758f6e8df"),
    "onnx-model":   (P + "onnx/pin.go", "6fa5be852612ce761094ba74ee6123b4fc4acfefa79bf64dc63acae4a83af2fd"),
}

def rd(d, f): return open(os.path.join(d, f)).read()
def wr(d, f, s): open(os.path.join(d, f), "w").write(s)
def sub1(s, old, new):
    assert s.count(old) == 1, (old, s.count(old))
    return s.replace(old, new)

def copy(src, name):
    dst = os.path.join(work, name)
    if os.path.exists(dst): shutil.rmtree(dst)
    shutil.copytree(src, dst)
    return dst

# remap: only what lets this Linux runner run the Windows-only pins. No sha is touched.
remap = copy(base, "src-remap")
for f in (P + "audiocpp/audiocpp.go", P + "onnx/onnx.go"):
    wr(remap, f, sub1(rd(remap, f), "p.goos = runtime.GOOS", 'p.goos = "windows"'))
h = sub1(rd(remap, P + "audiocpp/handler.go"), "ArchiveFor(runtime.GOOS, runtime.GOARCH, RowCPU)", 'ArchiveFor("windows", "amd64", RowCPU)')
h = sub1(h, '\t"runtime"\n', "")
wr(remap, P + "audiocpp/handler.go", h)
tp = rd(remap, P + "uv/toolspin.go")
m = re.search(r'\t"windows/amd64": \{Name: "git".*?\n.*?\n.*?\n', tp)  # the three lines of the git windows/amd64 row
row = m.group(0)
tp = tp.replace(row, row + row.replace('"windows/amd64"', '"linux/amd64"'), 1)
wr(remap, P + "uv/toolspin.go", tp)
# the linux ffmpeg row is untouched; the git linux row now serves MinGit's zip (Probe cmd/git.exe)

def flip(sha): return ("1" if sha[0] != "1" else "2") + sha[1:]

for name, (f, sha) in SHAS.items():
    d = copy(remap, "src-wrong-" + name)
    s = rd(d, f)
    if name == "git":  # the remapped linux/amd64 row only
        lines = s.split("\n")
        idx = [i for i, l in enumerate(lines) if sha in l]
        assert len(idx) == 2, idx
        # the second occurrence is the inserted linux row (it follows the windows/amd64 one)
        lines[idx[1]] = lines[idx[1]].replace(sha, flip(sha))
        s = "\n".join(lines)
    else:
        s = sub1(s, sha, flip(sha))
    wr(d, f, s)

# mock: every pinned URL points at the local mock server
MOCK = "http://127.0.0.1:18999/"
mock = copy(remap, "src-mock")
for f in (P + "uv/pin.go", P + "uv/toolspin.go", P + "audiocpp/pin.go", P + "audiocpp/models.go", P + "onnx/pin.go"):
    s = rd(mock, f)
    n = s.count("https://github.com/") + s.count("https://huggingface.co/")
    assert n > 0, f
    s = s.replace("https://github.com/", MOCK + "github.com/").replace("https://huggingface.co/", MOCK + "huggingface.co/")
    wr(mock, f, s)

for name in ["src-remap", "src-mock"] + ["src-wrong-" + n for n in SHAS]:
    r = subprocess.run(["diff", "-ru", base, os.path.join(work, name)], capture_output=True, text=True)
    open(os.path.join(work, name + ".diff"), "w").write(r.stdout)
    print(name, "diff lines:", len(r.stdout.splitlines()))
