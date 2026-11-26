"""The checksum checks, run against real launcher binaries. See CHECKSUM-CHECKS.md.
Usage: CHECKSUM_WORK=<scratch> python3 -I cases.py real|mock [case ...]"""
import json, os, re, shutil, subprocess, sys, time, urllib.request
from concurrent.futures import ThreadPoolExecutor
sys.path.insert(0, os.path.dirname(__file__))
import drive

W = os.environ.get("CHECKSUM_WORK", "/tmp/checksum-work")  # the scratch folder: src-*, bin/, roots/, evidence/
EV = W + "/evidence"
KOKORO = "tts_webui_extension.kokoro"   # a native (own-venv) entry: its install begins with the uv platform
POCKET = "audiocpp_pocket_tts_en"
KITTEN = "tts_webui_onnx.kitten_nano"
UVDONE = r"The uv platform is installed"

# name: (package, extra, wrong binary, mock match, name in the error, stop_after for the retry, files that must exist after the retry)
CASES = {
  "uv":           (KOKORO, {"gpu": "CPU"}, "wrong-uv", "uv-x86_64-unknown-linux-gnu", "uv-x86_64-unknown-linux-gnu.tar.gz", UVDONE, ["platforms/uv/bin/uv", "platforms/uv/platform.json"]),
  "ffmpeg":       (KOKORO, {"gpu": "CPU"}, "wrong-ffmpeg", "ffmpeg-n8.1.3", "ffmpeg-n8.1.3-27-g8dca1586f4-linux64-gpl-shared-8.1.tar.xz", UVDONE, ["platforms/uv/tools/ffmpeg/tool.json", "platforms/uv/tools/ffmpeg/bin/ffmpeg", "platforms/uv/platform.json"]),
  "git":          (KOKORO, {"gpu": "CPU"}, "wrong-git", "MinGit", "MinGit-2.56.1-64-bit.zip", UVDONE, ["platforms/uv/tools/git/tool.json", "platforms/uv/tools/git/cmd/git.exe", "platforms/uv/platform.json"]),
  "audiocpp":     (POCKET, {"audiocpp_row": "cpu"}, "wrong-audiocpp", "audio-v0.9.6-bin-windows-x64-cpu", "audio-v0.9.6-bin-windows-x64-cpu.zip", None, ["platforms/audiocpp/platform.json", "platforms/audiocpp/bin/audiocpp_server.exe", "platforms/audiocpp/models/pocket-en-q8/model.json"]),
  "gguf":         (POCKET, {"audiocpp_row": "cpu"}, "wrong-gguf", "pocket-tts-english-q8_0.gguf", "pocket-tts-english-q8_0.gguf", None, ["platforms/audiocpp/models/pocket-en-q8/model.json", "platforms/audiocpp/models/pocket-en-q8/pocket-tts-english-q8_0.gguf"]),
  "embedding":    (POCKET, {"audiocpp_row": "cpu"}, "wrong-embedding", "alba.safetensors", "embeddings/alba.safetensors", None, ["platforms/audiocpp/models/pocket-en-q8/model.json", "platforms/audiocpp/models/pocket-en-q8/embeddings/alba.safetensors"]),
  "onnx-runtime": (KITTEN, {}, "wrong-onnx-runtime", "sherpa-onnx-v1.13.8-win", "sherpa-onnx-v1.13.8-win-x64-shared-MT-Release.tar.bz2", None, ["platforms/onnx/platform.json", "platforms/onnx/models/kitten-nano/entry.json"]),
  "onnx-model":   (KITTEN, {}, "wrong-onnx-model", "kitten-nano-en", "kitten-nano-en-v0_8-int8.tar.bz2", None, ["platforms/onnx/platform.json", "platforms/onnx/models/kitten-nano/entry.json"]),
}
PORT = iter(range(18100, 18400) if (sys.argv[1:2] == ["mock"]) else range(18500, 18800))
def fresh_root(name):
    r = f"{W}/roots/{name}"
    shutil.rmtree(r, ignore_errors=True); os.makedirs(r)
    shutil.copy(f"{W}/src-base/extensions.json", r)
    return r

def inst(L, package, rec, extra, stop_after=None):
    """The page sends the GPU / audio.cpp build choice only when it installs that platform."""
    extra = dict(extra)
    if os.path.exists(os.path.join(L.root, "platforms/uv/platform.json")): extra.pop("gpu", None)
    if os.path.exists(os.path.join(L.root, "platforms/audiocpp/platform.json")): extra.pop("audiocpp_row", None)
    return L.install(package, rec, stop_after=stop_after, **extra)

class Rec:
    def __init__(self, path):
        os.makedirs(os.path.dirname(path), exist_ok=True)
        self.f = open(path, "w")
    def __call__(self, s=""):
        self.f.write(s + "\n"); self.f.flush()

def snapshot(L, rec, package, tag):
    rec(f"--- {tag}: files below the install folder (platforms/, .venvs/, cache/ only) ---")
    for sub in ("platforms", ".venvs"):
        d = os.path.join(L.root, sub)
        if os.path.isdir(d):
            t = drive.tree(d)
            skip = re.compile(r"(audiocpp/bin|onnx/runtime|uv/python|uv/tools/ffmpeg|uv/tools/git)/.+")
            for l in t:
                if not skip.match(l): rec(f"  {sub}/{l}")
            rec(f"  ({len(t)} entries below {sub}/ in all; what is inside bin/, runtime/, python/ and the unpacked tools is not listed one by one)")
    bad = drive.leftovers(L.root)
    rec(f"--- {tag}: half-done names (*.part, *.download, *.new, *-archive*, audio.zip): {bad or 'none'} ---")
    _, e = L.ext(package)
    if e:
        rec(f"--- {tag}: /api/ext/list says: installed={e[0].get('installed')} requirements_state={json.dumps(e[0].get('requirements_state'))} ---")
    return bad

def run_real(name):
    package, extra, wrong, _, errname, stop, must = CASES[name]
    root = fresh_root("real-" + name)
    rec = Rec(f"{EV}/real-{name}.txt")
    res = {"case": name, "mode": "wrong-checksum (real download)"}
    L = drive.Launcher(f"{W}/bin/{wrong}", root, next(PORT)); L.start()
    try:
        rec(f"## launcher {wrong} (one pinned sha256 edited), install of {package} {extra}")
        t = time.time()
        st, ok, lines = inst(L, package, rec, extra)
        rec(f"## wrong-checksum install: http {st}, ok={ok}, {time.time()-t:.0f}s")
        err = [l for l in lines if "checksum mismatch" in l]
        res.update(http=st, ok=ok, mismatch=bool(err), names_artifact=any(errname in l for l in err),
                   downloaded=any(re.search(r"\b100%|9\d% of|Downloading", l) for l in lines) and any("% of" in l for l in lines),
                   error=(err[0] if err else None))
        if err:
            m = re.search(r"got ([0-9a-f]{64}), want ([0-9a-f]{64})", err[0])
            res["got"], res["want"] = m.groups() if m else (None, None)
        bad = snapshot(L, rec, package, "after the wrong checksum")
        res["leftovers"] = bad
    finally:
        L.stop()
    # retry with the right pin (same folder, the control binary)
    L2 = drive.Launcher(f"{W}/bin/remap", root, next(PORT)); L2.start()
    try:
        rec(f"## launcher remap (right pins), same folder: install of {package} again")
        st, ok, lines = inst(L2, package, rec, extra, stop)
        rec(f"## retry: http {st}, ok={ok}")
        snapshot(L2, rec, package, "after the retry")
        res["retry_ok"] = ok in (True, "stopped")
        res["retry_files"] = {f: os.path.exists(os.path.join(root, f)) for f in must}
        res["retry_matches"] = [l for l in lines if l.startswith("Checksum matches")]
    finally:
        L2.stop()
    res["pass"] = bool(res.get("mismatch") and res.get("names_artifact") and not res.get("ok") and not res["leftovers"]
                       and res["retry_ok"] and all(res["retry_files"].values()))
    json.dump(res, open(f"{EV}/real-{name}.json", "w"), indent=1)
    return res

def rule(match, mode):
    urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:18999/__rule?match={match}&mode={mode}", method="POST")).read()

MODES = ["404", "cut", "cut-chunked", "reset", "short", "empty"]
def run_mock(name):
    package, extra, _, match, errname, stop, must = CASES[name]
    root = fresh_root("mock-" + name)
    rec = Rec(f"{EV}/mock-{name}.txt")
    res = {"case": name, "mode": "mock server", "modes": {}}
    L = drive.Launcher(f"{W}/bin/mock", root, next(PORT)); L.start()
    try:
        for m in MODES:
            rule(match, m)
            rec(f"\n######## mock mode {m} for {match} ########")
            st, ok, lines = inst(L, package, rec, extra)
            rec(f"## http {st}, ok={ok}")
            bad = snapshot(L, rec, package, f"after {m}")
            fail = [l for l in lines if l.startswith("Failed") or "Failed to install" in l]
            res["modes"][m] = {"http": st, "ok": ok, "message": fail[-1] if fail else None, "leftovers": bad,
                               "names_artifact": any(errname.split("/")[-1] in l for l in fail)}
        rule(match, "ok")
        rec(f"\n######## mock mode ok (the retry) ########")
        st, ok, lines = inst(L, package, rec, extra, stop)
        rec(f"## http {st}, ok={ok}")
        snapshot(L, rec, package, "after the retry")
        res["retry_ok"] = ok in (True, "stopped")
        res["retry_files"] = {f: os.path.exists(os.path.join(root, f)) for f in must}
    finally:
        L.stop()
    res["pass"] = bool(res.get("retry_ok") and all(res["retry_files"].values()) and all(
        (not v["ok"]) and not v["leftovers"] and v["message"] and v["names_artifact"] for v in res["modes"].values()))
    json.dump(res, open(f"{EV}/mock-{name}.json", "w"), indent=1)
    return res

if __name__ == "__main__":
    kind = sys.argv[1]; names = sys.argv[2:] or list(CASES)
    fn = {"real": run_real, "mock": run_mock}[kind]
    with ThreadPoolExecutor(4 if kind == 'real' else 1) as ex:
        for name, r in zip(names, ex.map(lambda n: (lambda: fn(n))(), names)):
            print(kind, name, "PASS" if r["pass"] else "FAIL", flush=True)
