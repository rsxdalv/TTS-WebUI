// `npm run build:static`: builds the static export into react-ui/out, next to
// the server build in .next (`npm run build` / `npm start` are unchanged).
//
// Next 16 always compiles an export into .next, so the server build there
// would be overwritten. This script therefore sets .next aside as
// .next-server-saved for the length of the export and puts it back at the end
// (also when the build fails). If a run is killed in between, the next run
// (or `node build-static.js --recover`, or `npm run build`) puts it back
// first (see recover). Nothing here detects a
// running `next start`: stop it first, or it serves errors while .next is
// away.
//
// The export is built in .out-building and renamed to out/ only when it is
// complete, so a failed or interrupted build leaves the previous out/ as it
// was. out/_tts-export.json marks a complete export.

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

// A run that was killed (the `finally` below did not run) leaves the marker
// .next-export-scratch next to .next-server-saved. Then .next is the export's
// scratch directory, whatever it contains (Next writes BUILD_ID early), and
// the saved server build is put back. `npm run build` calls this first (the
// prebuild script), so a later server build starts from a restored tree and
// simply overwrites it. Without the marker nothing is restored: a saved copy
// left without one is an orphan and is deleted.
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

function run(cmd, args, env) {
  const r = spawnSync(cmd, args, {
    cwd: root,
    stdio: "inherit",
    shell: process.platform === "win32",
    env: { ...process.env, ...env },
  });
  return r.status === 0;
}

function main() {
  recover();
  if (process.argv.includes("--recover")) return 0;

  // prebuild step of the server build: generates src/extensions/link.ts
  if (!run("node", ["generate-plugin-imports.js"])) return 1;

  rm(BUILDING);
  const hadServerBuild = fs.existsSync(NEXT);
  try {
    if (hadServerBuild) {
      fs.writeFileSync(MARKER, "the server build is in .next-server-saved\n");
      fs.renameSync(NEXT, SAVED);
    }
  } catch (e) {
    console.error(
      `build:static: cannot move .next aside (${e.code}). Stop \`next start\` and retry.`
    );
    return 1;
  }

  let ok = false;
  let buildId = "";
  try {
    ok = run("npx", ["next", "build"], { TTS_STATIC_EXPORT: "1" });
    if (ok) buildId = fs.readFileSync(path.join(NEXT, "BUILD_ID"), "utf8").trim();
  } finally {
    rm(NEXT); // the export's scratch directory
    if (hadServerBuild) {
      fs.renameSync(SAVED, NEXT);
      rm(MARKER);
    }
  }
  if (!ok) {
    rm(BUILDING);
    return 1;
  }

  fs.writeFileSync(
    path.join(BUILDING, "_tts-export.json"),
    JSON.stringify({ buildId, builtAt: new Date().toISOString() }) + "\n"
  );
  // Next copies public/ into the export. Uploads (file-input-cache) are served
  // live from public/ by Go, and a copy here would be stale.
  rm(path.join(BUILDING, "file-input-cache"));
  rm(OUT);
  fs.renameSync(BUILDING, OUT);
  console.log("build:static: export written to react-ui/out");
  return 0;
}

if (require.main === module) process.exit(main());
module.exports = { recover };
