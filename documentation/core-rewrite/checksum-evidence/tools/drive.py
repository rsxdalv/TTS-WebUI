"""Drive a real launcher binary over its HTTP API (stdlib only)."""
import json, os, re, signal, subprocess, sys, time, urllib.request

class Launcher:
    def __init__(self, binary, root, port):
        self.binary, self.root, self.port = binary, root, port
        self.proc = None; self.token = None
    def start(self):
        os.makedirs(self.root, exist_ok=True)
        self.log = open(os.path.join(os.path.dirname(self.root.rstrip("/")), os.path.basename(self.root.rstrip("/")) + ".launcher.log"), "ab")
        self.proc = subprocess.Popen([self.binary, "--root", self.root, "--port", str(self.port), "--no-browser"],
                                     stdout=self.log, stderr=self.log, stdin=subprocess.DEVNULL, start_new_session=True)
        for _ in range(100):
            try:
                html = urllib.request.urlopen(f"http://127.0.0.1:{self.port}/", timeout=2).read().decode()
                self.token = re.search(r'name="tts-webui-token" content="([0-9a-f]+)"', html).group(1)
                return
            except Exception:
                time.sleep(0.2)
        raise RuntimeError("launcher did not start")
    def stop(self):
        if self.proc:
            try: os.killpg(self.proc.pid, signal.SIGTERM)
            except ProcessLookupError: pass
            try: self.proc.wait(10)
            except subprocess.TimeoutExpired:
                os.killpg(self.proc.pid, signal.SIGKILL)
            self.log.close(); self.proc = None
    def req(self, method, path, body=None, timeout=1800):
        data = json.dumps(body).encode() if body is not None else None
        r = urllib.request.Request(f"http://127.0.0.1:{self.port}{path}", data=data, method=method,
                                   headers={"X-TTS-Token": self.token, "Content-Type": "application/json"})
        return urllib.request.urlopen(r, timeout=timeout)
    def install(self, package, emit, stop_after=None, **extra):
        """POST /api/ext/op/install; returns (http_status, ok, lines). emit gets each line."""
        body = {"package_name": package, **extra}
        try:
            resp = self.req("POST", "/api/ext/op/install", body)
        except urllib.error.HTTPError as e:
            txt = e.read().decode()
            emit(f"[HTTP {e.code}] {txt.strip()}")
            return e.code, False, [txt]
        lines, ok = [], None
        for raw in resp:
            raw = raw.decode().strip()
            if not raw: continue
            j = json.loads(raw)
            if "line" in j:
                lines.append(j["line"]); emit(j["line"])
                if stop_after and re.search(stop_after, j["line"]):
                    resp.close(); time.sleep(3)   # the page's own "stop" is closing the stream
                    return resp.status, "stopped", lines
            if j.get("done"):
                ok = bool(j.get("ok"))
        return resp.status, ok, lines
    def ext(self, package):
        j = json.load(self.req("GET", "/api/ext/list", timeout=60))
        return j, [e for e in j.get("entries", j.get("extensions", [])) if e.get("package_name") == package or e.get("name") == package]

def tree(root):
    """Every file and folder below root (relative), with sizes; skips nothing."""
    out = []
    for d, dirs, files in os.walk(root):
        dirs.sort()
        for n in sorted(dirs):
            out.append(os.path.relpath(os.path.join(d, n), root) + "/")
        for n in sorted(files):
            p = os.path.join(d, n)
            out.append(f"{os.path.relpath(p, root)}  ({os.lstat(p).st_size} B)")
    return out

PARTIAL = re.compile(r"(\.part$|\.download$|\.new$|-archive(\.|$)|audio\.zip$|\.tmp$)")
def leftovers(root):
    """Names that mean "half done": temp/partial names anywhere below root."""
    bad = []
    for d, dirs, files in os.walk(root):
        for n in dirs + files:
            if PARTIAL.search(n) or n.endswith(".new") or n.endswith(".part"):
                bad.append(os.path.relpath(os.path.join(d, n), root))
    return bad
