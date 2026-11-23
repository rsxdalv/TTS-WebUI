// `npm run build:static`: builds the static export of the React app (Vite)
// into react-ui/out. Go serves it (CONTRACTS.md section 10).
//
// The build goes to .out-building and is renamed to out/ only when it is
// complete, so a failed or interrupted build leaves the previous out/ as it
// was. out/_tts-export.json marks a complete export.
//
// The export does not touch .next. The server build of `npm run build` /
// `npm start` (Next) still lives there. recover() and --stamp-server are for
// that server build: an older build:static that was killed may have left .next
// set aside as .next-server-saved, and recover() puts it back.

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const root = __dirname;
const p = (...a) => path.join(root, ...a);
const NEXT = p(".next");
const SAVED = p(".next-server-saved");
const MARKER = p(".next-export-scratch");
const BUILDING = p(".out-building");
const OUT = p("out");

const rm = (d) => fs.rmSync(d, { recursive: true, force: true, maxRetries: 5 });

// A run that was killed leaves the marker .next-export-scratch next to
// .next-server-saved (only older builds of the export did this). Then the saved
// server build is put back. `npm run build` calls this first (the prebuild
// script). Without the marker nothing is restored: a saved copy left without
// one is an orphan and is deleted.
function recover(dir = root) {
  const next = path.join(dir, ".next");
  const saved = path.join(dir, ".next-server-saved");
  const marker = path.join(dir, ".next-export-scratch");
  if (fs.existsSync(marker) && fs.existsSync(saved)) {
    console.log("build:static: restoring the server build saved by an interrupted run");
    rm(next);
    fs.renameSync(saved, next);
  }
  rm(marker);
  rm(saved);
}

function run(cmd, args) {
  const r = spawnSync(cmd, args, {
    cwd: root,
    stdio: "inherit",
    shell: process.platform === "win32",
    env: process.env,
  });
  return r.status === 0;
}

// sourceCommit is the git commit of the install the build comes from, or null
// outside a git checkout. Go compares it with HEAD to tell a stale build.
function sourceCommit() {
  const r = spawnSync("git", ["rev-parse", "HEAD"], {
    cwd: path.join(root, ".."),
    encoding: "utf8",
    shell: process.platform === "win32",
  });
  const out = (r.stdout || "").trim();
  return r.status === 0 && /^[0-9a-f]{40,64}$/.test(out) ? out : null;
}

// stampServer records which commit the server build in .next comes from
// (postbuild of npm run build). See CONTRACTS.md section 10.
function stampServer() {
  if (!fs.existsSync(path.join(NEXT, "BUILD_ID"))) return 0;
  fs.writeFileSync(
    path.join(NEXT, "tts-build.json"),
    JSON.stringify({ commit: sourceCommit(), builtAt: new Date().toISOString() }) + "\n"
  );
  return 0;
}

function main() {
  if (process.argv.includes("--stamp-server")) return stampServer();
  recover();
  if (process.argv.includes("--recover")) return 0;

  // generates src/extensions/link.ts, which the app imports
  if (!run("node", ["generate-plugin-imports.js"])) return 1;

  rm(BUILDING);
  if (!run("npx", ["vite", "build", "--outDir", BUILDING, "--emptyOutDir"])) {
    rm(BUILDING);
    return 1;
  }

  fs.writeFileSync(
    path.join(BUILDING, "_tts-export.json"),
    JSON.stringify({ builtAt: new Date().toISOString(), commit: sourceCommit() }) + "\n"
  );
  // Vite copies public/ into the export. Uploads (file-input-cache) are served
  // live from public/ by Go, and a copy here would be stale.
  rm(path.join(BUILDING, "file-input-cache"));
  rm(OUT);
  fs.renameSync(BUILDING, OUT);
  console.log("build:static: export written to react-ui/out");
  return 0;
}

if (require.main === module) process.exit(main());
module.exports = { recover };
