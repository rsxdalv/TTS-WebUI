"""Compares internal/hfcache with huggingface_hub on a cache this script builds.

    python parity.py <scratch dir> <repo root>

Needs a Python with huggingface_hub and gradio (the extension imports gradio),
and `go` on PATH. For every scenario it builds the same cache twice, runs the
extension's own scan code (extensions/builtin/extension_huggingface_cache_manager)
on one and the Go scan on the other, and compares every field of every row;
then it deletes the same revisions with huggingface_hub's delete_revisions (the
extension's delete path) on one copy and with the Go plan on the other and
compares the two folder trees (paths, sizes, link targets) and the rows again.
Nothing outside <scratch dir> is touched.
"""
import json, os, shutil, subprocess, sys, time

scratch, repo_root = os.path.abspath(sys.argv[1]), os.path.abspath(sys.argv[2])
H = {n: (str(n) * 40) for n in "123456789abcd"}
DAY = 86400


def put(path, data, atime=None, mtime=None):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as f:
        f.write(data)
    if atime is not None:
        os.utime(path, (atime, mtime if mtime is not None else atime))


def snap(repo, rev, name, blob):
    link = os.path.join(repo, "snapshots", rev, *name.split("/"))
    os.makedirs(os.path.dirname(link), exist_ok=True)
    up = "../" * (2 + name.count("/"))
    os.symlink(up + "blobs/" + blob, link)


NOW = time.time()  # every cache is built from the same instant, so times are comparable


def build(cache):
    now = NOW
    os.makedirs(cache)
    # model: two revisions sharing a blob, refs main and a nested PR ref, one detached revision
    r = os.path.join(cache, "models--acme--tiny")
    put(r + "/blobs/aaa", b"x" * 1234, now - 3 * DAY, now - 5 * DAY)
    put(r + "/blobs/bbb", b"y" * 99, now - 2 * DAY, now - 9 * DAY)
    put(r + "/blobs/ccc", b"z" * 4_500_000, now - 40 * DAY, now - 41 * DAY)
    for rev, files in {
        H["1"]: {"config.json": "aaa", "model.bin": "ccc"},
        H["2"]: {"config.json": "aaa", "tokenizer/vocab.txt": "bbb"},
        H["3"]: {"extra/deep/er/file.txt": "bbb"},
    }.items():
        for n, b in files.items():
            snap(r, rev, n, b)
    put(r + "/refs/main", H["1"].encode())
    put(r + "/refs/refs/pr/7", H["2"].encode())
    # model with a name with two dashes in the org (repo_id uses "/")
    r = os.path.join(cache, "models--org--name--with--dashes")
    put(r + "/blobs/d1", b"d" * 10, now - 400 * DAY, now - 400 * DAY)
    snap(r, H["4"], "a.txt", "d1")
    put(r + "/refs/main", H["4"].encode())
    # dataset and space
    r = os.path.join(cache, "datasets--acme--data")
    put(r + "/blobs/e1", b"e" * 2_000_000_000 if False else b"e" * 2_000_001, now - 100 * DAY, now - 100 * DAY)
    snap(r, H["5"], "train.csv", "e1")
    put(r + "/refs/main", H["5"].encode())
    r = os.path.join(cache, "spaces--acme--demo")
    put(r + "/blobs/f1", b"f", now - 800 * DAY, now - 800 * DAY)
    snap(r, H["6"], "app.py", "f1")
    # revision with no files: size 0, mtime of the folder
    r = os.path.join(cache, "models--acme--empty")
    os.makedirs(os.path.join(r, "snapshots", H["7"]))
    os.utime(os.path.join(r, "snapshots", H["7"]), (now - 10 * DAY, now - 10 * DAY))
    os.utime(r, (now - 11 * DAY, now - 12 * DAY))
    # things the scan ignores
    put(cache + "/.locks/models--acme--tiny/lock", b"")
    put(cache + "/CACHEDIR.TAG", b"x")
    # a copy instead of a symlink (Windows without symlinks, hand made)
    r = os.path.join(cache, "models--acme--copies")
    put(r + "/snapshots/" + H["8"] + "/plain.txt", b"copy" * 10, now - 2 * DAY, now - 2 * DAY)
    put(r + "/refs/main", H["8"].encode())


def tree(cache):
    out = {}
    for base, dirs, files in os.walk(cache):
        for n in dirs + files:
            p = os.path.join(base, n)
            rel = os.path.relpath(p, cache)
            if os.path.islink(p):
                out[rel] = "link:" + os.readlink(p)
            elif os.path.isdir(p):
                out[rel] = "dir"
            else:
                out[rel] = "file:%d" % os.path.getsize(p)
    return out


def py_rows(cache):
    env = dict(os.environ, HF_HUB_CACHE=cache, PYTHONPATH=repo_root)
    code = (
        "import json;"
        "from extensions.builtin.extension_huggingface_cache_manager.main import scan_cache_json;"
        "print(scan_cache_json())"
    )
    out = subprocess.run([sys.executable, "-c", code], env=env, cwd=repo_root, capture_output=True, text=True, check=True).stdout
    return json.loads(out.strip().splitlines()[-1])["rows"]


def py_delete(cache, hashes):
    env = dict(os.environ, HF_HUB_CACHE=cache, PYTHONPATH=repo_root)
    code = (
        "from huggingface_hub import scan_cache_dir;"
        "s=scan_cache_dir().delete_revisions(*%r);"
        "print(s.expected_freed_size);s.execute()" % (hashes,)
    )
    return int(subprocess.run([sys.executable, "-c", code], env=env, cwd=repo_root, capture_output=True, text=True, check=True).stdout.strip().splitlines()[-1])


def go(cache, out, hashes=None):
    env = dict(os.environ, HFCACHE_PARITY_DIR=cache, HFCACHE_PARITY_OUT=out)
    if hashes:
        env["HFCACHE_PARITY_DELETE"] = ",".join(hashes)
    subprocess.run(["go", "test", "./internal/hfcache", "-run", "TestParityDump", "-count=1"], env=env,
                   cwd=os.path.join(repo_root, "core"), capture_output=True, text=True, check=True)
    return json.load(open(out))


def compare(py, gor, label):
    problems = []
    if len(py) != len(gor):
        problems.append("%s: %d rows in python, %d in go" % (label, len(py), len(gor)))
    for a, b in zip(py, gor):
        for k in a:
            x, y = a[k], b.get(k)
            if k == "snapshot_path":  # the two copies live in different scratch folders
                x, y = x.split("/hub/", 1)[1], (y or "").split("/hub/", 1)[-1]
            if isinstance(x, float) or isinstance(y, float):
                ok = y is not None and abs(float(x) - float(y)) < 1e-3
            else:
                ok = x == y
            if not ok:
                problems.append("%s %s/%s: %s: python %r go %r" % (label, a["repo_id"], a["commit_hash"][:6], k, x, y))
    return problems


shutil.rmtree(scratch, ignore_errors=True)
os.makedirs(scratch)
problems, checked = [], 0
scenarios = [
    ("scan only", []),
    ("one revision, shared blob kept", [H["1"]]),
    ("revision with a shared blob and a ref", [H["2"]]),
    ("detached revision", [H["3"]]),
    ("all revisions of a repo", [H["1"], H["2"], H["3"]]),
    ("unknown hash and a real one", ["f" * 40, H["4"]]),
    ("a copy in the snapshot", [H["8"]]),
    ("revision with no files", [H["7"]]),
    ("several repos", [H["5"], H["6"], H["2"]]),
]
for i, (label, hashes) in enumerate(scenarios):
    py_cache, go_cache = os.path.join(scratch, "py%d" % i, "hub"), os.path.join(scratch, "go%d" % i, "hub")
    for c in (py_cache, go_cache):
        build(c)
    expected = None
    if hashes:
        expected = py_delete(py_cache, hashes)
    result = go(go_cache, os.path.join(scratch, "go%d.json" % i), hashes)
    rows_py = py_rows(py_cache)
    # python sorts by repo_path, rows come in that order in both
    p = compare(rows_py, result["rows"], label)
    if hashes:
        p += ["%s: trees differ: %s" % (label, d) for d in sorted(set(tree(py_cache).items()) ^ set(tree(go_cache).items()))]
        if expected != result["expected_freed"]:
            p.append("%s: expected freed: python %s go %s" % (label, expected, result["expected_freed"]))
        if result["errors"]:
            p.append("%s: go errors %s" % (label, result["errors"]))
    checked += len(rows_py)
    print("%-45s rows=%d %s" % (label, len(rows_py), "OK" if not p else "DIFFERENT"))
    problems += p
print("compared %d rows in %d scenarios" % (checked, len(scenarios)))
if problems:
    print("\n".join(problems))
    sys.exit(1)
print("python and go agree")
