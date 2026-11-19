// Reference implementation of the static export serving contract
// (documentation/core-rewrite/CONTRACTS.md, "Static export"). Test tool only:
// the real server is Go (task S2). `node staticsrv.js <out dir> <port> [<public dir>]`.
// Routes unknown to the export (everything under /api/) answer 404 here; put the
// Go proxy in front of it (gosrv -next 127.0.0.1:<port>) to get them.
const http = require("http");
const fs = require("fs");
const path = require("path");

const [out, port, pub] = [process.argv[2], Number(process.argv[3]), process.argv[4]];
const types = { ".html": "text/html; charset=utf-8", ".js": "application/javascript; charset=UTF-8", ".css": "text/css; charset=UTF-8", ".json": "application/json", ".ico": "image/x-icon", ".txt": "text/plain; charset=utf-8", ".woff2": "font/woff2" };

const fileIn = (root, rel) => {
  const f = path.join(root, rel);
  if (!f.startsWith(root + path.sep)) return null;
  try { return fs.statSync(f).isFile() ? f : null; } catch { return null; }
};

// URL path -> file in the export, as in the contract.
function resolve(p) {
  if (p === "/") return fileIn(out, "index.html");
  const rel = p.replace(/\/+$/, "");
  const data = /^(\/_next\/data\/[^/]+)\/extensions\/[^/]+\.json$/.exec(rel);
  return fileIn(out, rel) || fileIn(out, rel + ".html") || fileIn(out, rel + "/index.html") ||
    (/^\/extensions\/[^/]+$/.test(rel) ? fileIn(out, "extensions/_shell.html") : null) ||
    (data ? fileIn(out, data[1] + "/extensions/_shell.json") : null);
}

http.createServer((req, res) => {
  let p;
  try { p = decodeURIComponent(new URL(req.url, "http://x").pathname); } catch { res.writeHead(400).end(); return; }
  if (p.split("/").some((s) => s.startsWith(".")) || p.includes("\\") || p.includes("\0")) { res.writeHead(404).end(); return; }
  let f = pub && p.startsWith("/file-input-cache/") ? fileIn(pub, p.slice(1)) : null;
  let status = 200;
  if (!f) f = resolve(p);
  if (!f) { f = fileIn(out, "404.html"); status = 404; }
  const immutable = p.startsWith("/_next/static/");
  const html = f.endsWith(".html");
  res.writeHead(status, {
    "Content-Type": types[path.extname(f)] || "application/octet-stream",
    "Cache-Control": status === 404 ? "private, no-cache, no-store, max-age=0, must-revalidate"
      : immutable ? "public, max-age=31536000, immutable" : html || p.startsWith("/_next/data/") ? "s-maxage=31536000" : "public, max-age=0",
  });
  fs.createReadStream(f).pipe(res);
}).listen(port, "127.0.0.1");
