"""Condense evidence logs for the repo: drop progress lines, shorten the scratch path."""
import os, re, sys
src, dst = sys.argv[1], sys.argv[2]
os.makedirs(dst, exist_ok=True)
W = os.environ.get("CHECKSUM_WORK", "/tmp/checksum-work")  # the scratch folder: src-*, bin/, roots/, evidence/
for n in sorted(os.listdir(src)):
    if not n.endswith(".txt"): continue
    out = []
    pct = 0
    for l in open(os.path.join(src, n)):
        l = l.rstrip("\n").replace(W + "/roots/", "<scratch>/").replace("http://127.0.0.1:18999/", "mock://")
        l = re.sub(r"tcp 127\.0\.0\.1:\d+->127\.0\.0\.1:18999", "tcp <client>-><mock>", l)
        if re.search(r": \d+% of \d+ MB$", l):
            pct += 1; continue
        if pct: out.append(f"  ({pct} progress lines, 5% steps, left out)"); pct = 0
        out.append(l)
    open(os.path.join(dst, n), "w").write("\n".join(out) + "\n")
