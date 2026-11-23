"""Regenerates golden.json by running the REAL python code the Go package ports.

NO LONGER RUNNABLE (step 29): the real files it copies (server.py,
tts_webui/dotenv_manager, print_gradio_options.py, gradio_proxy_tree) were deleted
with the legacy Python server. golden.json is kept as the record of what they did;
check out the commit before step 29 to regenerate it.

    python gen_golden.py <repo-root> <out-golden.json>

Run it with the project's own interpreter (it needs python-dotenv). Nothing is
written inside <repo-root>: the real files (server.py, tts_webui/dotenv_manager,
tts_webui/config, print_gradio_options.py, gradio_proxy_tree/main.py) are copied
byte for byte into a temp tree, the heavy modules (gradio blocks, proxy tree
class, torch patch) are replaced by recording stubs, and every case runs in a
fresh copy of that tree so find_dotenv() resolves inside it.
"""
import hashlib, json, os, shutil, subprocess, sys, tempfile

REPO, OUT = os.path.abspath(sys.argv[1]), os.path.abspath(sys.argv[2])
PY = sys.executable

REAL = [
    "server.py",
    "tts_webui/__init__.py",
    "tts_webui/config/config.py",
    "tts_webui/config/load_config.py",
    "tts_webui/dotenv_manager/__init__.py",
    "tts_webui/dotenv_manager/init.py",
    "tts_webui/dotenv_manager/writer.py",
    "tts_webui/gradio/print_gradio_options.py",
    "tts_webui/gradio_proxy_tree/main.py",
]
STUBS = {
    "rec.py": "R = {'launch': None, 'tree': None, 'routes': [], 'popen': None, 'browser': []}\n",
    "tts_webui/utils/suppress_warnings.py": "def suppress_warnings():\n    pass\n",
    "tts_webui/utils/torch_load_patch.py": "def apply_torch_load_patch():\n    pass\n",
    "tts_webui/gradio_proxy_tree/__init__.py": (
        "import rec\n"
        "class GradioProxyTree:\n"
        "    def __init__(self, host, port):\n"
        "        self.host, self.port = host, port\n"
        "        rec.R['tree'] = {'host': host, 'port': port}\n"
        "    def add_route(self, p, port):\n"
        "        rec.R['routes'].append([p, port])\n"
        "    def start_background(self):\n"
        "        rec.R['tree']['host'] = self.host\n"
    ),
    "tts_webui/gradio/blocks.py": (
        "import rec\n"
        "class _D:\n"
        "    def queue(self): return self\n"
        "    def launch(self, **kw):\n"
        "        rec.R['launch'] = {k: (list(v) if isinstance(v, tuple) else v) for k, v in kw.items()}\n"
        "    def block_thread(self): pass\n"
        "def main_block(config=None): return _D()\n"
    ),
}

RUNNER = r'''
import json, logging, os, sys
sys.path.insert(0, os.getcwd())
case = json.load(open("case.json"))
warn = []
class H(logging.Handler):
    def emit(self, r): warn.append(r.args[0])
logging.getLogger("dotenv.main").addHandler(H())
out = {}
if case["mode"] == "env":
    from tts_webui.dotenv_manager import init
    init.init()
    out["env"] = dict(os.environ)
    out["warn_lines"] = warn
    out["env_file"] = open(".env", "rb").read().decode("latin-1") if os.path.exists(".env") else None
else:
    import rec, runpy, subprocess, webbrowser
    class P:
        def __init__(self, cmd, **kw):
            rec.R["popen"] = {"cmd": cmd, "backend": kw["env"].get("GRADIO_BACKEND_AUTOMATIC")}
    subprocess.Popen = P
    webbrowser.open = lambda url, *a, **k: rec.R["browser"].append(url)
    sys.argv = ["server.py"] + case["flags"]
    ns = runpy.run_path("server.py", run_name="server_under_test")
    ns["start"]()
    import tts_webui.gradio_proxy_tree.main as m
    out = dict(rec.R)
    out["react_ui_port"] = str(ns["REACT_UI_PORT"])
    out["env_tree_port"] = os.environ.get("GRADIO_TREE_PORT")
    out["env_tree_url"] = os.environ.get("GRADIO_TREE_URL")
    out["main_tree_port"] = m.GRADIO_TREE_PORT
    out["main_tree_url"] = m.GRADIO_TREE_URL
json.dump(out, open("result.json", "w"))
'''


def build_template(tmp):
    t = os.path.join(tmp, "template")
    sha = {}
    for rel in REAL:
        dst = os.path.join(t, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copyfile(os.path.join(REPO, rel), dst)
        sha[rel] = hashlib.sha256(open(dst, "rb").read()).hexdigest()
    for rel, text in STUBS.items():
        dst = os.path.join(t, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        open(dst, "w").write(text)
    open(os.path.join(t, "runner.py"), "w").write(RUNNER)
    return t, sha


def run(tmp, template, n, case, base, files):
    d = os.path.join(tmp, "case%d" % n)
    shutil.copytree(template, d)
    for rel, text in files.items():
        p = os.path.join(d, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "wb") as f:
            f.write(text.encode("utf-8"))
    json.dump(case, open(os.path.join(d, "case.json"), "w"))
    env = dict(base)
    env.setdefault("SYSTEMROOT", os.environ.get("SYSTEMROOT", ""))
    r = subprocess.run([PY, "-E", "runner.py"], cwd=d, env=env, capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit("case %s failed:\n%s%s" % (case.get("name"), r.stdout, r.stderr))
    return env, json.load(open(os.path.join(d, "result.json")))


ENV_CASES = [
    ("plain", {}, "A=1\nB=two\n", None),
    ("comments-export", {}, "# c\nexport A=1\nexport  B = 2 \n  C=3 # tail\nD=x#y\nE=\"q\"#c\n\n", None),
    ("quotes", {}, "A='single $x \\' \\\\ \\n'\nB=\"dq \\\" \\\\ \\n \\t \\a \\b \\f \\v \\r \\q\"\nC=\"\"\nD=''\n'E F'=1\n", None),
    ("multiline", {}, "A=\"line1\nline2\"\nB='x\ny'\nC=after\n", None),
    ("novalue", {}, "A\nexport B\nC=\nD= \n", None),
    ("bad-lines", {}, "A=1\n=oops\nB=\"unterminated\nC=3\nD=\"x\" junk\nE=5\n", None),
    ("unterminated-single", {}, "A='abc\nB=2\n", None),
    ("crlf", {}, "A=1\r\nB=\"x\r\ny\"\r\nC=3\rD=4\n", None),
    ("unicode-ws", {}, "A=1\u00a0\nB=a\u2003#c\n\u3000C=3\n", None),
    ("bom", {}, "\ufeffA=1\nB=2\n", None),
    ("duplicate", {}, "A=1\nB=${A}\nA=2\nC=${A}\n", None),
    ("interp", {"HOME2": "/h", "EMPTY": ""}, "A=${HOME2}/x\nB=${NOPE}|\nC=${NOPE:-dflt}\nD=${EMPTY:-dflt}\nE=${A}${B}\nF=$HOME2\nG=${}\nH=${a}\nI=${HOME2:-}\nJ=${X\n", None),
    ("interp-fwd-self", {}, "A=${B}\nB=1\nC=${C}x\nD=${NOVAL}\nNOVAL\n", None),
    ("override-base", {"A": "base", "B": "base"}, "A=env\nC=env\nD=${A}\n", "B=user\nC=user\nE=${B}-${C}-${A}\n"),
    ("user-only-chain", {"PATH": "/p"}, "X=1\n", "PATH=${PATH}:/q\nY=${X}\nZ=${PATH}\n"),
    ("case-windows", {"Hf_Home": "base", "UPPER": "u"}, "hf_home=file\nlower=1\nA=${lower} ${LOWER} ${Upper} ${UPPER}\n", "lower=2\nB=${lower}|${LOWER}|${Lower}\n"),
    ("stray-tts_webui-env", {}, "A=root\n", None),
    ("stray-dotenv_manager-env", {}, "A=root\n", None),
    ("user-only", {}, "", "U=1\n"),
]
STRAY = {
    "stray-tts_webui-env": {"tts_webui/.env": "A=stray1\nS=1\n"},
    "stray-dotenv_manager-env": {"tts_webui/dotenv_manager/.env": "A=stray2\nS=2\n", "tts_webui/.env": "A=stray1\n"},
}
GENENV_CASES = [
    ("empty", {}),
    ("hf", {"HUGGINGFACE_HUB_CACHE": "D:\\models\\hf", "HF_HOME": "D:\\hfhome"}),
    ("all", {"HUGGINGFACE_HUB_CACHE": "a", "HF_HOME": "b", "TORCH_HOME": "c", "XDG_CACHE_HOME": "d e"}),
    ("lowercase-name", {"torch_home": "lc"}),
]
CFG = {
    "inline": False, "inbrowser": True, "share": False, "debug": False, "max_threads": 40,
    "auth": None, "auth_message": None, "prevent_thread_lock": False, "show_error": False,
    "server_name": "127.0.0.1", "server_port": 7770, "height": 500, "width": "100%",
    "favicon_path": None, "ssl_keyfile": None, "ssl_certfile": None, "ssl_keyfile_password": None,
    "ssl_verify": True, "quiet": True, "show_api": True, "_frontend": True,
}
PLAN_VARIANTS = [
    ("default", {}, {}),
    ("noinbrowser", {"inbrowser": False}, {}),
    ("port8000-lan", {"server_port": 8000, "server_name": "0.0.0.0"}, {}),
    ("react3100", {}, {"REACT_UI_PORT": "3100"}),
    ("treeport-env", {}, {"GRADIO_TREE_PORT": "7999"}),
    ("auth-str", {"auth": "u:p"}, {}),
    ("share-cfg-strport", {"share": True, "server_port": "7800"}, {}),
]
FLAGSETS = []
for bits in range(32):
    fl = [f for i, f in enumerate(["--share", "--docker", "--no-tree-proxy", "--no-react", "--no-database"]) if bits >> i & 1]
    FLAGSETS.append(fl)


def main():
    tmp = tempfile.mkdtemp(prefix="appenv-golden-")
    try:
        template, sha = build_template(tmp)
        n = 0
        out = {"os": os.name, "python": sys.version.split()[0], "sha256": sha, "env_cases": [], "genenv_cases": [], "plan_cases": []}
        import dotenv
        out["python_dotenv"] = __import__("importlib.metadata").metadata.version("python-dotenv")
        for name, base, envf, user in ENV_CASES:
            files = {".env": envf}
            if user is not None:
                files[".env.user"] = user
            files.update(STRAY.get(name, {}))
            n += 1
            env, res = run(tmp, template, n, {"mode": "env", "name": name}, base, files)
            out["env_cases"].append({"name": name, "base": env, "env_file": envf, "env_user": user,
                                     "stray": STRAY.get(name, {}), "expected": res["env"], "warn_lines": res["warn_lines"]})
        for name, base in GENENV_CASES:
            n += 1
            env, res = run(tmp, template, n, {"mode": "env", "name": name}, base, {})
            out["genenv_cases"].append({"name": name, "base": env, "expected_file": res["env_file"]})
        for vname, over, envs in PLAN_VARIANTS:
            cfg = dict(CFG); cfg.update(over)
            user = "".join("%s=%s\n" % kv for kv in envs.items())
            for fl in FLAGSETS:
                n += 1
                files = {"config.json": json.dumps({"gradio_interface_options": cfg, "extensions": {}}), ".env": "", ".env.user": user}
                env, res = run(tmp, template, n, {"mode": "plan", "name": vname, "flags": fl}, {}, files)
                out["plan_cases"].append({"name": vname, "flags": fl, "config": cfg, "env_user": user, "result": res})
        json.dump(out, open(OUT, "w"), indent=1, sort_keys=True)
        print("wrote", OUT, len(out["env_cases"]), len(out["genenv_cases"]), len(out["plan_cases"]))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


main()
