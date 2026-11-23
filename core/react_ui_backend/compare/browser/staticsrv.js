// Reference implementation of the static export serving contract
// (documentation/core-rewrite/CONTRACTS.md, "Static export"). Test tool only:
// the real server is Go (task S2). `node staticsrv.js <out dir> <port> [<public dir>]`.
// Paths under /api/ answer 404 here; put the Go proxy in front of it to get
// the claimed routes.
const http = require("http");
const fs = require("fs");
const path = require("path");

// Resolved, so the "inside the folder" check compares like with like on Windows.
const [out, port, pub] = [path.resolve(process.argv[2]), Number(process.argv[3]), process.argv[4] && path.resolve(process.argv[4])];
const types = { ".html": "text/html; charset=utf-8", ".js": "application/javascript; charset=UTF-8", ".css": "text/css; charset=UTF-8", ".json": "application/json", ".ico": "image/x-icon", ".txt": "text/plain; charset=utf-8", ".woff2": "font/woff2" };

const fileIn = (root, rel) => {
  const f = path.join(root, rel);
  if (!f.startsWith(root + path.sep)) return null;
  try { return fs.statSync(f).isFile() ? f : null; } catch { return null; }
};

// URL path -> file in the export, as in the contract (single-page app): a file
// of the export under its own name, else the entry page for a route, else 404
// for a path with a file extension.
function resolve(p) {
  if (p === "/") return fileIn(out, "index.html");
  const rel = p.replace(/\/+$/, "");
  const file = fileIn(out, rel);
  if (file) return file;
  if (/\.[^/]*$/.test(path.posix.basename(rel))) return null;
  return fileIn(out, "index.html");
}

http.createServer((req, res) => {
  let p;
  try { p = decodeURIComponent(new URL(req.url, "http://x").pathname); } catch { res.writeHead(400).end(); return; }
  if (p.split("/").some((s) => s.startsWith(".")) || p.includes("\\") || p.includes("\0")) { res.writeHead(404).end(); return; }
  let f = pub && p.startsWith("/file-input-cache/") ? fileIn(pub, p.slice(1)) : null;
  let status = 200;
  if (!f) f = resolve(p);
  if (!f) { res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8", "Cache-Control": "private, no-cache, no-store, max-age=0, must-revalidate" }).end("404 page not found\n"); return; }
  const immutable = p.startsWith("/assets/");
  const html = f.endsWith(".html");
  res.writeHead(status, {
    "Content-Type": types[path.extname(f)] || "application/octet-stream",
    "Cache-Control": immutable ? "public, max-age=31536000, immutable" : html ? "s-maxage=31536000" : "public, max-age=0",
  });
  fs.createReadStream(f).pipe(res);
}).listen(port, "127.0.0.1");
